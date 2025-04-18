// Copyright 2024, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package complexconfig adds middleware for schema informed complex configuration
// encoding/decoding as a work-around for https://github.com/pulumi/pulumi/pull/15032.
//
// The entry point for this package is [Wrap].
//
// Deprecated: This package will be removed after
// https://github.com/pulumi/pulumi/pull/15032 merges.
package complexconfig

import (
	"context"
	"encoding/json"

	"github.com/pulumi/pulumi-go-provider/internal/putil"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/sig"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/property"

	p "github.com/pulumi/pulumi-go-provider"
)

func Wrap(provider p.Provider) p.Provider {
	encoder := provider
	contract.Assertf(provider.GetSchema != nil, "provider.GetSchema must be implemented")
	encoder.CheckConfig = encodeCheckConfig(provider.CheckConfig, provider.GetSchema)
	return encoder
}

type (
	getSchema   = func(context.Context, p.GetSchemaRequest) (p.GetSchemaResponse, error)
	checkConfig = func(context.Context, p.CheckRequest) (p.CheckResponse, error)
)

func encodeCheckConfig(check checkConfig, getSchema getSchema) checkConfig {
	if check == nil {
		check = func(_ context.Context, req p.CheckRequest) (p.CheckResponse, error) {
			return p.CheckResponse{Inputs: req.News}, nil
		}
	}
	return func(ctx context.Context, req p.CheckRequest) (p.CheckResponse, error) {
		// Only req.News is from the engine. req.Olds (if it exists) is from state
		// and thus went through a previous normalizing pass of CheckConfig.

		// If there are no inputs, then we can just return.
		if req.News.Len() == 0 {
			return check(ctx, req)
		}

		schemaResp, err := getSchema(ctx, p.GetSchemaRequest{})
		if err != nil {
			return p.CheckResponse{},
				p.InternalErrorf("unable to decode config: no schema available: %w", err)
		}
		var spec schema.PackageSpec
		if err := json.Unmarshal([]byte(schemaResp.Schema), &spec); err != nil {
			return p.CheckResponse{},
				p.InternalErrorf("unable to decode config: invalid schema: %w", err)
		}

		news := resource.ToResourcePropertyValue(property.New(req.News)).ObjectValue()

		for k, spec := range spec.Config.Variables {
			v, ok := news[resource.PropertyKey(k)]
			if !ok {
				continue
			}

			news[resource.PropertyKey(k)] = fixEncoding(v, spec)
		}

		// Attempt to decode any inputs that don't have a matching config schema.
		for k, v := range news {
			if _, ok := spec.Config.Variables[string(k)]; ok {
				continue
			}
			news[k] = fixEncoding(v, schema.PropertySpec{})
		}

		req.News = resource.FromResourcePropertyValue(resource.NewProperty(news)).AsMap()
		return check(ctx, req)
	}
}

func fixEncoding(v resource.PropertyValue, spec schema.PropertySpec) resource.PropertyValue {
	// Ensure that v is unwrapped

	switch {
	case v.IsComputed():
		return v
	case v.IsSecret():
		return resource.MakeSecret(fixEncoding(v.SecretValue().Element, spec))
	case v.IsOutput():
		o := v.OutputValue()
		o.Element = fixEncoding(v, spec)
		return resource.NewProperty(o)
	}

	// If the value is not a string, we assume that it is the correct type.
	//
	// If spec.Type is a string, we still need to attempt to decode it, since it may
	// be secret or computed, and thus represented as a JSON encoded object.
	if !v.IsString() {
		return v
	}

	var target any
	err := json.Unmarshal([]byte(v.StringValue()), &target)
	if err != nil {
		return v
	}

	// Instead of using resource.NewPropertyValue, specialize it to detect nested
	// json-encoded secrets and computed values.
	var replv func(encoded any) (resource.PropertyValue, bool)
	replv = func(v any) (resource.PropertyValue, bool) {
		if s, ok := v.(string); ok {
			switch s {
			case plugin.UnknownBoolValue:
				return resource.MakeComputed(resource.NewProperty(false)), true
			case plugin.UnknownNumberValue:
				return resource.MakeComputed(resource.NewProperty(0.0)), true
			case plugin.UnknownStringValue:
				return resource.MakeComputed(resource.NewProperty("")), true
			case plugin.UnknownArrayValue:
				return resource.MakeComputed(resource.NewProperty([]resource.PropertyValue{})), true
			case plugin.UnknownObjectValue:
				return resource.MakeComputed(resource.NewProperty(resource.PropertyMap{})), true
			case plugin.UnknownAssetValue:
				return resource.MakeComputed(resource.NewProperty(&resource.Asset{})), true
			case plugin.UnknownArchiveValue:
				return resource.MakeComputed(resource.NewProperty(&resource.Archive{})), true
			}
		}
		m, ok := v.(map[string]any)
		if !ok {
			return resource.PropertyValue{}, false
		}

		value, ok := m[sig.Key]
		if !ok {
			return resource.PropertyValue{}, false
		}
		sigValue, ok := value.(string)
		if !ok {
			return resource.PropertyValue{}, false
		}

		switch sigValue {
		case sig.Secret:
			return putil.MakeSecret(
				resource.NewPropertyValueRepl(m["value"], nil, replv),
			), true
		case sig.OutputValue:
			castBool := func(key string) bool {
				v, ok := m[key]
				if !ok {
					return false
				}
				b, ok := v.(bool)
				return ok && b
			}

			deps, _ := m["dependencies"].([]any)
			var dependencies []resource.URN
			if len(deps) > 0 {
				dependencies = make([]resource.URN, 0, len(deps))
			}
			for _, d := range deps {
				urn, ok := d.(string)
				if !ok {
					continue
				}
				dependencies = append(dependencies, resource.URN(urn))
			}

			elem, hasElem := m["value"]
			return resource.NewProperty(resource.Output{
				Secret:       castBool("secret"),
				Dependencies: dependencies,
				Known:        hasElem,
				Element:      resource.NewPropertyValueRepl(elem, nil, replv),
			}), true
		default:
			contract.Failf("Unknown sig value: %#v", sigValue)
			return resource.PropertyValue{}, false
		}
	}

	out := resource.NewPropertyValueRepl(target, nil, replv)

	// If the expected type is a string and the raw underlying type is not a string,
	// then don't use the JSON encoded value (since it might be a valid JSON value of
	// a type other then string, for example: 42, a JSON number but a valid string
	// value).
	if spec.Type == "string" && !unwrap(out).IsString() {
		return v
	}
	return out
}

func unwrap(v resource.PropertyValue) resource.PropertyValue {
	for {
		switch {
		case v.IsSecret():
			v = v.SecretValue().Element
		case v.IsComputed():
			v = v.V.(resource.Computed).Element
		case v.IsOutput():
			v = v.OutputValue().Element
		default:
			return v
		}
	}
}
