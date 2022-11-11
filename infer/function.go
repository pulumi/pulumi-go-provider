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
	"unicode"

	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/internal/introspect"
	t "github.com/pulumi/pulumi-go-provider/middleware"
	"github.com/pulumi/pulumi-go-provider/middleware/schema"
)

// A Function (also called Invoke) inferred from code. `I` is the function input, and `O`
// is the function output. Both must be structs.
type Fn[I any, O any] interface {
	// A function is a mapping from `I` to `O`.
	Call(ctx p.Context, input I) (output O, err error)
}

// A function inferred from code. See Function for creating a InferredFunction.
type InferredFunction interface {
	t.Invoke
	schema.Function

	isInferredFunction()
}

// Infer a function from `F`, which maps `I` to `O`.
func Function[F Fn[I, O], I, O any]() InferredFunction {
	return &derivedInvokeController[F, I, O]{}
}

type derivedInvokeController[F Fn[I, O], I, O any] struct{}

func (derivedInvokeController[F, I, O]) isInferredFunction() {}

func (*derivedInvokeController[F, I, O]) GetToken() (tokens.Type, error) {
	var f F
	tk, err := introspect.GetToken("pkg", f)
	if err != nil {
		return "", err
	}
	return fnToken(tk), nil
}

func fnToken(tk tokens.Type) tokens.Type {
	name := []rune(tk.Name().String())
	for i, r := range name {
		if !unicode.IsUpper(r) {
			break
		}
		if i == 0 || len(name) == i+1 || unicode.IsUpper(name[i+1]) {
			name[i] = unicode.ToLower(r)
		}
	}
	return tokens.NewTypeToken(tk.Module(), tokens.TypeName(name))
}

func (*derivedInvokeController[F, I, O]) GetSchema(reg schema.RegisterDerivativeType) (pschema.FunctionSpec, error) {
	var f F
	descriptions := getAnnotated(reflect.TypeOf(f))

	input, err := objectSchema(reflect.TypeOf(new(I)))
	if err != nil {
		return pschema.FunctionSpec{}, err
	}
	output, err := objectSchema(reflect.TypeOf(new(O)))
	if err != nil {
		return pschema.FunctionSpec{}, err
	}

	if err := registerTypes[I](reg); err != nil {
		return pschema.FunctionSpec{}, err
	}
	if err := registerTypes[O](reg); err != nil {
		return pschema.FunctionSpec{}, err
	}

	return pschema.FunctionSpec{
		Description: descriptions.Descriptions[""],
		Inputs:      input,
		Outputs:     output,
	}, nil
}

func objectSchema(t reflect.Type) (*pschema.ObjectTypeSpec, error) {
	descriptions := getAnnotated(t)
	props, required, err := propertyListFromType(t, false)
	if err != nil {
		return nil, fmt.Errorf("could not serialize input type %s: %w", t, err)
	}
	for n, p := range props {
		props[n] = p
	}
	return &pschema.ObjectTypeSpec{
		Description: descriptions.Descriptions[""],
		Properties:  props,
		Required:    required,
		Type:        "object",
	}, nil
}

func (r *derivedInvokeController[F, I, O]) Invoke(ctx p.Context, req p.InvokeRequest) (p.InvokeResponse, error) {
	var i I
	secrets, mapErr := decode(req.Args, &i, false)
	mapFailures, err := checkFailureFromMapError(mapErr)
	if err != nil {
		return p.InvokeResponse{}, err
	}
	if len(mapFailures) > 0 {
		return p.InvokeResponse{
			Failures: mapFailures,
		}, nil
	}
	var f F
	// If F is a *struct, we need to rehydrate the underlying struct
	if v := reflect.ValueOf(f); v.Kind() == reflect.Pointer && v.IsNil() {
		f = reflect.New(v.Type().Elem()).Interface().(F)
	}
	o, err := f.Call(ctx, i)
	if err != nil {
		return p.InvokeResponse{}, err
	}

	// TODO secret function call parameters
	m, err := encode(o, secrets, false, nil)
	if err != nil {
		return p.InvokeResponse{}, err
	}
	return p.InvokeResponse{
		Return: m,
	}, nil
}
