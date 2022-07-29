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
	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	pprovider "github.com/pulumi/pulumi/sdk/v3/go/pulumi/provider"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/internal/introspect"
	t "github.com/pulumi/pulumi-go-provider/middleware"
	"github.com/pulumi/pulumi-go-provider/middleware/schema"
)

type ComponentResource[I any, O pulumi.ComponentResource] interface {
	Construct(ctx *pulumi.Context, name, typ string, inputs I, opts pulumi.ResourceOption) (O, error)
}

type InferedComponent interface {
	t.ComponentResource
	schema.Resource
}

func Component[R ComponentResource[I, O], I any, O pulumi.ComponentResource]() InferedComponent {
	return &derivedComponentController[R, I, O]{}
}

type derivedComponentController[R ComponentResource[I, O], I any, O pulumi.ComponentResource] struct{}

func (rc *derivedComponentController[R, I, O]) GetSchema(reg schema.RegisterDerivativeType) (
	pschema.ResourceSpec, error) {
	r, err := getResourceSchema[R, I, O]()
	if err != nil {
		return pschema.ResourceSpec{}, err
	}
	if err := registerTypes[I](reg); err != nil {
		return pschema.ResourceSpec{}, err
	}
	if err := registerTypes[O](reg); err != nil {
		return pschema.ResourceSpec{}, err
	}
	r.IsComponent = true
	return r, nil
}

func (rc *derivedComponentController[R, I, O]) GetToken() (tokens.Type, error) {
	var r R
	return introspect.GetToken("pkg", r)
}

func (rc *derivedComponentController[R, I, O]) Construct(pctx p.Context, typ string, name string,
	ctx *pulumi.Context, inputs pprovider.ConstructInputs, opts pulumi.ResourceOption) (pulumi.ComponentResource, error) {
	var r R
	var i I
	err := inputs.CopyTo(&i)
	if err != nil {
		return nil, err
	}
	res, err := r.Construct(ctx, name, typ, i, opts)
	if err != nil {
		return nil, err
	}

	// Register the outputs
	m := introspect.StructToMap(res)
	err = ctx.RegisterResourceOutputs(res, pulumi.ToMap(m))
	if err != nil {
		return nil, err
	}
	return res, err
}
