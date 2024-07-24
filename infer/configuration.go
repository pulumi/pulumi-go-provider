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
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/mapper"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer/internal/ende"
	"github.com/pulumi/pulumi-go-provider/middleware/schema"
)

// Config turns an object into a description for the provider configuration.
//
// `T` has the same properties as an input or output type for a custom resource, and is
// responsive to the same interfaces.
//
// `T` can implement [CustomDiff] and [CustomCheck] and [CustomConfigure] and [Annotated].
func Config[T any]() InferredConfig {
	return &config[T]{}
}

type InferredConfig interface {
	schema.Resource
	underlyingType() reflect.Type
	checkConfig(ctx context.Context, req p.CheckRequest) (p.CheckResponse, error)
	diffConfig(ctx context.Context, req p.DiffRequest) (p.DiffResponse, error)
	configure(ctx context.Context, req p.ConfigureRequest) error
}

// CustomConfigure describes a provider that requires custom configuration before running.
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

type config[T any] struct{ t *T }

func (*config[T]) underlyingType() reflect.Type {
	var t T
	return reflect.TypeOf(t)
}

func (*config[T]) GetToken() (tokens.Type, error) { return "pulumi:providers:pkg", nil }
func (*config[T]) GetSchema(reg schema.RegisterDerivativeType) (pschema.ResourceSpec, error) {
	if err := registerTypes[T](reg); err != nil {
		return pschema.ResourceSpec{}, err
	}
	r, errs := getResourceSchema[T, T, T](false)
	return r, errs.ErrorOrNil()
}

func (c *config[T]) checkConfig(ctx context.Context, req p.CheckRequest) (p.CheckResponse, error) {
	var t T
	if v := reflect.ValueOf(t); v.Kind() == reflect.Pointer && v.IsNil() {
		t = reflect.New(v.Type().Elem()).Interface().(T)
	}

	encoder, decodeError := ende.DecodeConfig(req.News, &t)
	if t, ok := ((interface{})(t)).(CustomCheck[T]); ok {
		// The user implemented check manually, so call that.
		//
		// We don't apply defaults, but [DefaultCheck] does.
		var name string
		if req.Urn != "" {
			name = req.Urn.Name()
		}
		defCheckEnc, i, failures, err := callCustomCheck(ctx, t, name, req.Olds, req.News)
		if err != nil {
			return p.CheckResponse{}, err
		}

		if defCheckEnc != nil {
			encoder = *defCheckEnc
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

	return p.CheckResponse{
		Inputs:   applySecrets[T](news),
		Failures: failures,
	}, nil
}

func (c *config[T]) diffConfig(ctx context.Context, req p.DiffRequest) (p.DiffResponse, error) {
	c.ensure()
	return diff[T, T, T](ctx, req, c.t, func(string) bool { return true })
}

func (c *config[T]) configure(ctx context.Context, req p.ConfigureRequest) error {
	c.ensure()
	_, err := ende.DecodeConfig(req.Args, c.t)
	if err != nil {
		return c.handleConfigFailures(ctx, err)
	}

	// If we have a custom configure command, call that and return the error if any.
	if typ := reflect.TypeOf(c.t).Elem(); typ.Implements(reflect.TypeOf((*CustomConfigure)(nil)).Elem()) {
		return reflect.ValueOf(c.t).Elem().Interface().(CustomConfigure).Configure(ctx)
	}

	return nil
}

// Ensure that the config value is hydrated so we can assign to it.
func (c *config[T]) ensure() {
	if c.t == nil {
		c.t = new(T)
	}

	// T might be a *C for some type C, so we need to rehydrate it.
	if v := reflect.ValueOf(c.t).Elem(); v.Kind() == reflect.Pointer && v.IsNil() {
		v.Set(reflect.New(v.Type().Elem()))
	}
}

func (c *config[T]) handleConfigFailures(ctx context.Context, err mapper.MappingError) error {
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
