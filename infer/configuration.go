// Copyright 2022, Pulumi Corporation.
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

package infer

import (
	"context"
	"fmt"
	"reflect"

	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/mapper"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer/internal/config"
	"github.com/pulumi/pulumi-go-provider/infer/internal/ende"
)

// Turn an object into a description for the provider configuration.
//
// `T` has the same properties as an input or output type for a custom resource, and is
// responsive to the same interfaces.
//
// `T` can implement [CustomDiff] and [CustomCheck] and [CustomConfigure].
func Config[T any]() InferredConfig {
	return &config.Config[T]{}
}

type InferredConfig interface{ config.IsConfig }

// A provider that requires custom configuration before running.
//
// This interface should be implemented by reference to allow setting private fields on
// its receiver.
type CustomConfigure interface {
	// Configure the provider.
	//
	// This method will only be called once per provider process.
	//
	// By the time Configure is called, the receiver will be fully hydrated.
	//
	// Changes to the receiver will not be saved in state. For normalizing inputs see
	// [CustomCheck].
	Configure(ctx context.Context) error
}

func checkConfig[T any](ctx context.Context, c config.Config[T], req p.CheckRequest) (p.CheckResponse, error) {
	var t T
	if v := reflect.ValueOf(t); v.Kind() == reflect.Pointer && v.IsNil() {
		t = reflect.New(v.Type().Elem()).Interface().(T)
	}

	r, err := c.GetSchema(func(tokens.Type, pschema.ComplexTypeSpec) bool { return false })
	if err != nil {
		return p.CheckResponse{}, fmt.Errorf("could not get config secrets: %w", err)
	}
	encoder, decodeError := ende.DecodeConfig(req.News, &t)
	if t, ok := ((interface{})(t)).(CustomCheck[T]); ok {
		// The user implemented check manually, so call that
		i, failures, err := t.Check(ctx, req.Urn.Name(), req.Olds, req.News)
		if err != nil {
			return p.CheckResponse{}, err
		}

		inputs, err := encoder.Encode(i)
		if err != nil {
			return p.CheckResponse{}, err
		}
		return p.CheckResponse{
			Inputs:   inputs,
			Failures: failures,
		}, nil
	}

	failures, err := checkFailureFromMapError(decodeError)
	if err != nil {
		return p.CheckResponse{}, err
	}

	err = applyDefaults(&t)
	if err != nil {
		return p.CheckResponse{}, err
	}

	news, err := encoder.Encode(t)
	if err != nil {
		return p.CheckResponse{}, err
	}

	ip := r.InputProperties
	op := r.Properties
	if ip == nil {
		ip = map[string]pschema.PropertySpec{}
	}
	if op == nil {
		op = map[string]pschema.PropertySpec{}
	}

	for k, v := range news {
		if (op[string(k)].Secret || ip[string(k)].Secret) && !v.IsSecret() {
			req.News[k] = resource.MakeSecret(v)
		}
	}

	return p.CheckResponse{
		Inputs:   news,
		Failures: failures,
	}, nil
}

func diffConfig(ctx context.Context, c config.Internal, req p.DiffRequest) (p.DiffResponse, error) {
	return diff[T, T, T](ctx, req, c.Value(), func(string) bool { return true })
}

func configure(ctx context.Context, c config.Internal, req p.ConfigureRequest) error {
	_, err := ende.DecodeConfig(req.Args, c.Value())
	if err != nil {
		return handleConfigFailures(ctx, c, err)
	}

	// If we have a custom configure command, call that and return the error if any.
	if typ := reflect.TypeOf(c.Value()).Elem(); typ.Implements(reflect.TypeOf((*CustomConfigure)(nil)).Elem()) {
		return reflect.ValueOf(c.Value()).Elem().Interface().(CustomConfigure).Configure(ctx)
	}

	return nil
}

func handleConfigFailures(ctx context.Context, c config.Internal, err mapper.MappingError) error {
	if err == nil {
		return nil
	}

	pkgName := p.GetRunInfo(ctx).PackageName
	schema, mErr := c.GetSchema(func(tokens.Type, pschema.ComplexTypeSpec) bool { return false })
	if mErr != nil {
		return mErr
	}

	missing := map[string]string{}
	for _, err := range err.Failures() {
		switch err := err.(type) {
		case *mapper.MissingError:
			tk := fmt.Sprintf("%s:%s", pkgName, err.Field())
			missing[tk] = schema.InputProperties[err.Field()].Description
		default:
			return fmt.Errorf("unknown mapper error: %w", err)
		}
	}
	return p.ConfigMissingKeys(missing)
}
