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

	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	presource "github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/mapper"

	p "github.com/iwahbe/pulumi-go-provider"
	"github.com/iwahbe/pulumi-go-provider/internal/introspect"
	t "github.com/iwahbe/pulumi-go-provider/middleware"
	"github.com/iwahbe/pulumi-go-provider/middleware/schema"
)

type Fn[I any, O any] interface {
	Call(ctx p.Context, input I) (output O, err error)
}

type InferedFunction interface {
	t.Invoke
	schema.Invokes
}

func Function[F Fn[I, O], I, O any]() InferedFunction {
	return &derivedInvokeController[F, I, O]{}
}

type derivedInvokeController[F Fn[I, O], I, O any] struct{}

func (*derivedInvokeController[F, I, O]) GetToken() (tokens.Type, error) {
	var f F
	return introspect.GetToken("pkg", f)
}

func (*derivedInvokeController[F, I, O]) GetSchema() (pschema.FunctionSpec, error) {
	var f F
	descriptions := map[string]string{}
	if f, ok := (interface{})(f).(Annotated); ok {
		a := introspect.NewAnnotator(f)
		f.Annotate(&a)
		descriptions = a.Descriptions
	}

	input, err := objectSchema[I]()
	if err != nil {
		return pschema.FunctionSpec{}, err
	}
	output, err := objectSchema[O]()
	if err != nil {
		return pschema.FunctionSpec{}, err
	}
	return pschema.FunctionSpec{
		Description: descriptions[""],
		Inputs:      input,
		Outputs:     output,
	}, nil
}

func objectSchema[T any]() (*pschema.ObjectTypeSpec, error) {
	var t T
	descriptions := map[string]string{}
	if t, ok := (interface{})(t).(Annotated); ok {
		a := introspect.NewAnnotator(t)
		t.Annotate(&a)
		descriptions = a.Descriptions
	}

	props, required, err := propertyListFromType[T]()
	if err != nil {
		return nil, fmt.Errorf("could not serialize input type %T: %w", t, err)
	}
	for n, p := range props {
		p.Description = descriptions[n]
	}
	return &pschema.ObjectTypeSpec{
		Description: descriptions[""],
		Properties:  props,
		Required:    required,
	}, nil
}

func (r *derivedInvokeController[F, I, O]) Invoke(ctx p.Context, req p.InvokeRequest) (p.InvokeResponse, error) {
	var i I
	secrets, mapErr := r.decode(req.Args, &i)
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
	o, err := f.Call(ctx, i)
	if err != nil {
		return p.InvokeResponse{}, err
	}
	m, err := r.encode(o, secrets)
	if err != nil {
		return p.InvokeResponse{}, err
	}
	return p.InvokeResponse{
		Return: m,
	}, nil
}

func (*derivedInvokeController[F, I, O]) decode(m presource.PropertyMap, dst interface{}) (
	[]presource.PropertyKey, mapper.MappingError) {
	m, secrets := extractSecrets(m)
	return secrets, mapper.New(&mapper.Opts{}).Decode(m.Mappable(), dst)
}

func (*derivedInvokeController[F, I, O]) encode(src interface{}, secrets []presource.PropertyKey) (
	presource.PropertyMap, mapper.MappingError) {
	props, err := mapper.New(nil).Encode(src)
	if err != nil {
		return nil, err
	}
	m := presource.NewPropertyMapFromMap(props)
	for _, s := range secrets {
		v, ok := m[s]
		if !ok {
			continue
		}
		m[s] = presource.NewSecretProperty(&presource.Secret{Element: v})
	}
	return m, nil
}
