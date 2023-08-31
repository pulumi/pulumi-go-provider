// Copyright 2016-2023, Pulumi Corporation.
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

package tfbridge

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/golang/protobuf/ptypes/struct"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
)

type ConfigEncoding struct {
	inputs map[string]schema.PropertySpec
}

func New(config *schema.ResourceSpec) *ConfigEncoding {
	var c ConfigEncoding
	if config != nil {
		c.inputs = config.InputProperties
	}
	return &c
}

func (*ConfigEncoding) tryUnwrapSecret(encoded any) (any, bool) {
	m, ok := encoded.(map[string]any)
	if !ok {
		return nil, false
	}
	sig, ok := m[resource.SigKey]
	if !ok {
		return nil, false
	}
	ss, ok := sig.(string)
	if !ok {
		return nil, false
	}
	if ss != resource.SecretSig {
		return nil, false
	}
	value, ok := m["value"]
	return value, ok
}

func (enc *ConfigEncoding) convertStringToPropertyValue(s string, typ schema.PropertySpec) (resource.PropertyValue, error) {
	// If the schema expects a string, we can just return this as-is.
	if typ.Type == "string" {
		return resource.NewStringProperty(s), nil
	}

	// Otherwise, we will attempt to deserialize the input string as JSON and convert the result into a Pulumi
	// property. If the input string is empty, we will return an appropriate zero value.
	if s == "" {
		return enc.zeroValue(typ), nil
	}

	var jsonValue interface{}
	if err := json.Unmarshal([]byte(s), &jsonValue); err != nil {
		return resource.PropertyValue{}, err
	}

	opts := enc.unmarshalOpts()

	// Instead of using resource.NewPropertyValue, specialize it to detect nested json-encoded secrets.
	var replv func(encoded any) (resource.PropertyValue, bool)
	replv = func(encoded any) (resource.PropertyValue, bool) {
		encodedSecret, isSecret := enc.tryUnwrapSecret(encoded)
		if !isSecret {
			return resource.NewNullProperty(), false
		}

		v := resource.NewPropertyValueRepl(encodedSecret, nil, replv)
		if opts.KeepSecrets {
			v = resource.MakeSecret(v)
		}

		return v, true
	}

	return resource.NewPropertyValueRepl(jsonValue, nil, replv), nil
}

func (*ConfigEncoding) zeroValue(typ schema.PropertySpec) resource.PropertyValue {
	// According to the documentation for schema.PropertySpec.Type:
	//
	// Type is the primitive or composite type, if any. May be "boolean", "string",
	// "integer", "number", "array", or "object".
	switch typ.Type {
	case "boolean":
		return resource.NewPropertyValue(false)
	case "integer", "number":
		return resource.NewPropertyValue(0)
	case "array":
		return resource.NewPropertyValue([]interface{}{})
	default:
		return resource.NewPropertyValue(map[string]interface{}{})
	}
}

func (enc *ConfigEncoding) unmarshalOpts() plugin.MarshalOptions {
	return plugin.MarshalOptions{
		Label:        "config",
		KeepUnknowns: true,
		SkipNulls:    true,
		RejectAssets: true,
	}
}

// Like plugin.UnmarshalPropertyValue but overrides string parsing with convertStringToPropertyValue.
func (enc *ConfigEncoding) unmarshalPropertyValue(key resource.PropertyKey,
	v *structpb.Value) (*resource.PropertyValue, error) {

	opts := enc.unmarshalOpts()

	pv, err := plugin.UnmarshalPropertyValue(key, v, opts)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling property %q: %w", key, err)
	}
	schemaType, ok := enc.inputs[string(key)]

	// Only apply JSON-encoded recognition for known fields.
	if !ok {
		return pv, nil
	}

	var jsonString string
	var jsonStringDetected, jsonStringSecret bool

	if pv.IsString() {
		jsonString = pv.StringValue()
		jsonStringDetected = true
	}

	if opts.KeepSecrets && pv.IsSecret() && pv.SecretValue().Element.IsString() {
		jsonString = pv.SecretValue().Element.StringValue()
		jsonStringDetected = true
		jsonStringSecret = true
	}

	if jsonStringDetected {
		v, err := enc.convertStringToPropertyValue(jsonString, schemaType)
		if err != nil {
			return nil, fmt.Errorf("error unmarshalling property %q: %w", key, err)
		}
		if jsonStringSecret {
			s := resource.MakeSecret(v)
			return &s, nil
		}
		return &v, nil
	}

	// Computed sentinels are coming in as always having an empty string, but the encoding coerses them to a zero
	// value of the appropriate type.
	if pv.IsComputed() {
		el := pv.V.(resource.Computed).Element
		if el.IsString() && el.StringValue() == "" {
			res := resource.MakeComputed(enc.zeroValue(schemaType))
			return &res, nil
		}
	}

	return pv, nil
}

// Inline from plugin.UnmarshalProperties substituting plugin.UnmarshalPropertyValue.
func (enc *ConfigEncoding) UnmarshalProperties(props *structpb.Struct) (resource.PropertyMap, error) {
	opts := enc.unmarshalOpts()

	result := make(resource.PropertyMap, len(props.Fields))

	// First sort the keys so we enumerate them in order (in case errors happen, we want determinism).
	keys := make([]string, 0, len(props.Fields))
	if props != nil {
		for k := range props.Fields {
			keys = append(keys, k)
		}
		sort.Strings(keys)
	}

	fmt.Printf("\nUnmarshalling keys: %#v\n", keys)

	// And now unmarshal every field it into the map.
	for _, key := range keys {
		pk := resource.PropertyKey(key)
		v, err := enc.unmarshalPropertyValue(pk, props.Fields[key])
		if err != nil {
			return nil, err
		}

		if v == nil || opts.SkipNulls && v.IsNull() {
			fmt.Printf("Skipping a nil value of key %q\n", key)
			continue
		}
		if opts.SkipInternalKeys && resource.IsInternalPropertyKey(pk) {
			continue
		}

		fmt.Printf("Setting key to %#v\n", *v)
		result[pk] = *v
	}

	return result, nil
}

// Inverse of UnmarshalProperties, with additional support for secrets. Since the encoding cannot represent nested
// secrets, any nested secrets will be approximated by making the entire top-level property secret.
func (enc *ConfigEncoding) MarshalProperties(props resource.PropertyMap) (*structpb.Struct, error) {
	opts := plugin.MarshalOptions{
		Label:        "config",
		KeepUnknowns: true,
		SkipNulls:    true,
		RejectAssets: true,
		KeepSecrets:  true,
	}

	copy := make(resource.PropertyMap)
	for k, v := range props {
		var err error
		copy[k], err = enc.jsonEncodePropertyValue(k, v)
		if err != nil {
			return nil, err
		}
	}
	return plugin.MarshalProperties(copy, opts)
}

func (enc *ConfigEncoding) jsonEncodePropertyValue(k resource.PropertyKey,
	v resource.PropertyValue) (resource.PropertyValue, error) {
	if v.ContainsUnknowns() {
		return resource.NewStringProperty(plugin.UnknownStringValue), nil
	}
	if v.ContainsSecrets() {
		// Destructively strip secrets from a property value.
		var stripSecrets func(p resource.PropertyValue) resource.PropertyValue
		stripSecrets = func(p resource.PropertyValue) resource.PropertyValue {
			switch {
			// There are only 4 types of property values that can contain have secrets:
			//
			// Actual secret values:
			// 1. Secrets
			// 2. Outputs (has a secrets bit)
			//
			// Generic containers:
			// 3. Arrays
			// 4. Objects

			case p.IsSecret():
				s := p.SecretValue()
				if s != nil {
					return stripSecrets(s.Element)
				}
				return resource.NewNullProperty()
			case p.IsOutput():
				o := p.OutputValue()
				o.Secret = false
				o.Element = stripSecrets(o.Element)
				return resource.NewOutputProperty(o)

			case p.IsArray():
				arr := p.ArrayValue()
				for k, v := range arr {
					arr[k] = stripSecrets(v)
				}
				return p

			case p.IsObject():
				m := p.ObjectValue()
				for k, v := range m {
					m[k] = stripSecrets(v)
				}
				return p
			default:
				return p
			}
		}

		encoded, err := enc.jsonEncodePropertyValue(k, stripSecrets(v))
		if err != nil {
			return v, err
		}
		return resource.MakeSecret(encoded), err
	}
	_, knownKey := enc.inputs[string(k)]
	switch {
	case knownKey && v.IsNull():
		return resource.NewStringProperty(""), nil
	case knownKey && !v.IsNull() && !v.IsString():
		encoded, err := json.Marshal(v.Mappable())
		if err != nil {
			return v, fmt.Errorf("JSON encoding error while marshalling property %q: %w", k, err)
		}
		return resource.NewStringProperty(string(encoded)), nil
	default:
		return v, nil
	}
}
