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
	"os"
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
// `T` can implement CustomDiff and CustomCheck.
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

type config[T any] struct {
	t *T
}

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

	r, mErr := c.GetSchema(func(tk tokens.Type, typ pschema.ComplexTypeSpec) bool { return false })
	if mErr != nil {
		return p.CheckResponse{}, fmt.Errorf("could not get config secrets: %w", mErr)
	}

	if t, ok := ((interface{})(t)).(CustomCheck[T]); ok {
		// The user implemented check manually, so call that
		i, failures, err := t.Check(ctx, req.Urn.Name().String(), req.Olds, req.News)
		if err != nil {
			return p.CheckResponse{}, err
		}
		// TODO configuration secret
		rSchema, _ := getResourceSchema[T, T, T](false)
		inputs, err := encode(i, nil, false, &rSchema)
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

	var err mapper.MappingError
	if value.Kind() != reflect.Pointer {
		_, err = decode(req.News, &t, true)
	} else {
		_, err = decode(req.News, value.Interface(), true)
	}

	failures, e := checkFailureFromMapError(err)
	if e != nil {
		return p.CheckResponse{}, e
	}

	ip := r.InputProperties
	op := r.Properties
	if ip == nil {
		ip = map[string]pschema.PropertySpec{}
	}
	if op == nil {
		op = map[string]pschema.PropertySpec{}
	}

	for k, v := range req.News {
		if (op[string(k)].Secret || ip[string(k)].Secret) && !v.IsSecret() {
			req.News[k] = resource.MakeSecret(v)
		}
	}

	return p.CheckResponse{
		Inputs:   req.News,
		Failures: failures,
	}, nil
}

func (c *config[T]) diffConfig(ctx p.Context, req p.DiffRequest) (p.DiffResponse, error) {
	if c.t == nil {
		c.t = new(T)
	}
	return diff[T, T, T](ctx, req, c.t, func(string) bool { return true })
}

// Fetch an environmental or default value for a missing key.
func fetchEnvDefault(k resource.PropertyKey, prop pschema.PropertySpec) (resource.PropertyValue, bool, error) {
	assign := func(value any) (resource.PropertyValue, error) {
		switch prop.Type {
		case "bool":
			switch value := value.(type) {
			case bool:
				return resource.NewBoolProperty(value), nil
			case string:
				if value == "true" {
					return resource.NewBoolProperty(true), nil
				} else if value == "false" {
					return resource.NewBoolProperty(false), nil
				}
			}
		case "string":
			return resource.NewStringProperty(fmt.Sprintf("%v", value)), nil
		case "integer":
			switch value := value.(type) {
			case int:
				return resource.NewNumberProperty(float64(value)), nil
			case int16:
				return resource.NewNumberProperty(float64(value)), nil
			case int32:
				return resource.NewNumberProperty(float64(value)), nil
			case int64:
				return resource.NewNumberProperty(float64(value)), nil
			}
		case "number":
			switch value := value.(type) {
			case int:
				return resource.NewNumberProperty(float64(value)), nil
			case int16:
				return resource.NewNumberProperty(float64(value)), nil
			case int32:
				return resource.NewNumberProperty(float64(value)), nil
			case int64:
				return resource.NewNumberProperty(float64(value)), nil
			case float32:
				return resource.NewNumberProperty(float64(value)), nil
			case float64:
				return resource.NewNumberProperty(value), nil
			}
		case "array", "object":
			return resource.PropertyValue{}, fmt.Errorf("environmental variables do not support %ss", prop.Type)
		case "":
			return resource.PropertyValue{}, fmt.Errorf("environmental variables cannot be reference type")
		default:
			return resource.PropertyValue{}, fmt.Errorf("internal error: invalid property type: %q", prop.Type)
		}
		return resource.PropertyValue{}, fmt.Errorf(
			"cannot convert config value %s: %[1]q from %[1]T to %s", k, value, prop.Type)
	}
	if info := prop.DefaultInfo; info != nil {
		for _, env := range info.Environment {
			if value, ok := os.LookupEnv(env); ok {
				v, err := assign(value)
				return v, true, err
			}
		}
	}
	if prop.Default != nil {
		v, err := assign(prop.Default)
		return v, true, err
	}
	return resource.PropertyValue{}, false, nil
}

func (c *config[T]) configure(ctx p.Context, req p.ConfigureRequest) error {
	if c.t == nil {
		c.t = new(T)
	}
	schema, mErr := c.GetSchema(func(tk tokens.Type, typ pschema.ComplexTypeSpec) bool { return false })
	if mErr != nil {
		return mErr
	}

	for k, prop := range schema.InputProperties {
		k := resource.PropertyKey(k)
		if _, ok := req.Args[k]; !ok {
			v, ok, err := fetchEnvDefault(k, prop)
			if err != nil {
				return err
			}
			if ok {
				req.Args[k] = v
			}
		}
	}

	var err mapper.MappingError
	if typ := reflect.TypeOf(c.t).Elem(); typ.Kind() == reflect.Pointer {
		reflect.ValueOf(c.t).Elem().Set(reflect.New(typ.Elem()))
		_, err = decode(req.Args, reflect.ValueOf(c.t).Elem().Interface(), false)
	} else {
		_, err = decode(req.Args, c.t, false)
	}
	if err != nil {
		return c.handleConfigFailures(ctx, err)
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
