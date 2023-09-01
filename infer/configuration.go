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
	"fmt"
	"reflect"

	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/mapper"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/middleware/schema"
)

// Turn an object into a description for the provider configuration.
//
// `T` has the same properties as an input or output type for a custom resource, and is
// responsive to the same interfaces.
//
// `T` can implement [CustomDiff] and [CustomCheck] and [CustomConfigure].
func Config[T any]() InferredConfig {
	return &config[T]{}
}

type InferredConfig interface {
	schema.Resource
	underlyingType() reflect.Type
	checkConfig(ctx p.Context, req p.CheckRequest) (p.CheckResponse, error)
	diffConfig(ctx p.Context, req p.DiffRequest) (p.DiffResponse, error)
	configure(ctx p.Context, req p.ConfigureRequest) error
}

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
	Configure(ctx p.Context) error
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

func (c *config[T]) checkConfig(ctx p.Context, req p.CheckRequest) (p.CheckResponse, error) {
	var t T
	if v := reflect.ValueOf(t); v.Kind() == reflect.Pointer && v.IsNil() {
		t = reflect.New(v.Type().Elem()).Interface().(T)
	}

	r, err := c.GetSchema(func(tk tokens.Type, typ pschema.ComplexTypeSpec) bool { return false })
	if err != nil {
		return p.CheckResponse{}, fmt.Errorf("could not get config secrets: %w", err)
	}

	if t, ok := ((interface{})(t)).(CustomCheck[T]); ok {
		// The user implemented check manually, so call that
		i, failures, err := t.Check(ctx, req.Urn.Name().String(), req.Olds, req.News)
		if err != nil {
			return p.CheckResponse{}, err
		}
		inputs, err := encode(i, nil, false)
		if err != nil {
			return p.CheckResponse{}, err
		}
		return p.CheckResponse{
			Inputs:   inputs,
			Failures: failures,
		}, nil
	}

	value := reflect.ValueOf(t)
	for value.Kind() == reflect.Pointer && value.Elem().Kind() == reflect.Pointer {
		value = value.Elem()
	}

	var (
		secrets []resource.PropertyPath
		mErr    mapper.MappingError
	)
	if value.Kind() != reflect.Pointer {
		secrets, mErr = decodeConfigure(req.News, &t, true)
	} else {
		secrets, mErr = decodeConfigure(req.News, value.Interface(), true)
	}

	failures, e := checkFailureFromMapError(mErr)
	if e != nil {
		return p.CheckResponse{}, e
	}

	err = applyDefaults(&t)
	if err != nil {
		return p.CheckResponse{}, err
	}

	news, err := encode(t, secrets, req.News.ContainsUnknowns())
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

func (c *config[T]) diffConfig(ctx p.Context, req p.DiffRequest) (p.DiffResponse, error) {
	if c.t == nil {
		c.t = new(T)
	}
	return diff[T, T, T](ctx, req, c.t, func(string) bool { return true })
}

func (c *config[T]) configure(ctx p.Context, req p.ConfigureRequest) error {
	if c.t == nil {
		c.t = new(T)
	}
	var err mapper.MappingError
	if typ := reflect.TypeOf(c.t).Elem(); typ.Kind() == reflect.Pointer {
		reflect.ValueOf(c.t).Elem().Set(reflect.New(typ.Elem()))
		_, err = decodeConfigure(req.Args, reflect.ValueOf(c.t).Elem().Interface(), false)
	} else {
		_, err = decodeConfigure(req.Args, c.t, false)
	}
	if err != nil {
		return c.handleConfigFailures(ctx, err)
	}

	// If we have a custom configure command, call that and return the error if any.
	if typ := reflect.TypeOf(c.t).Elem(); typ.Implements(reflect.TypeOf((*CustomConfigure)(nil)).Elem()) {
		return reflect.ValueOf(c.t).Elem().Interface().(CustomConfigure).Configure(ctx)
	}

	return nil
}

func (c *config[T]) handleConfigFailures(ctx p.Context, err mapper.MappingError) error {
	if err == nil {
		return nil
	}

	pkgName := ctx.RuntimeInformation().PackageName
	schema, mErr := c.GetSchema(func(tk tokens.Type, typ pschema.ComplexTypeSpec) bool { return false })
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
