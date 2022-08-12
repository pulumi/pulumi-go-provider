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
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
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

	var err mapper.MappingError
	if value.Kind() != reflect.Pointer {
		_, err = decode(req.News, &t, false)
	} else {
		_, err = decode(req.News, value.Interface(), false)
	}

	failures, e := checkConfigFailures(err, ctx.RuntimeInformation().PackageName)
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
	return diff[T, T, T](ctx, req, c.t, true)
}

func (c *config[T]) configure(ctx p.Context, req p.ConfigureRequest) error {
	t := new(T)
	ctx.Logf(diag.Info, "envs: %v", req.Variables)
	if typ := reflect.TypeOf(t).Elem(); typ.Kind() == reflect.Pointer {
		reflect.ValueOf(t).Elem().Set(reflect.New(typ.Elem()))
		_, err := decode(req.Args, reflect.ValueOf(t).Elem().Interface(), false)
		if err != nil {
			return err
		}
	} else {
		_, err := decode(req.Args, t, false)
		if err != nil {
			return err
		}
	}

	c.t = t
	return nil
}

func checkConfigFailures(err mapper.MappingError, pkgName string) ([]p.CheckFailure, error) {
	if err == nil {
		return nil, nil
	}
	failures := []p.CheckFailure{}
	for _, err := range err.Failures() {
		switch err := err.(type) {
		case *mapper.MissingError:
			tk := fmt.Sprintf("%s:%s", pkgName, err.Field())
			reason := fmt.Sprintf("missing required configuration key %[1]s.\n"+
				"\tSet a value using the command `pulumi config set %[1]s <value>`",
				tk)

			failures = append(failures, p.CheckFailure{
				Property: err.Field(),
				Reason:   reason,
			})

		case mapper.FieldError:
			failures = append(failures, p.CheckFailure{
				Property: err.Field(),
				Reason:   err.Reason(),
			})
		default:
			return failures, fmt.Errorf("unknown mapper error: %w", err)
		}
	}
	return failures, nil
}
