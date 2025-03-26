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

	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	pprovider "github.com/pulumi/pulumi/sdk/v3/go/pulumi/provider"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/internal/introspect"
	t "github.com/pulumi/pulumi-go-provider/middleware"
	"github.com/pulumi/pulumi-go-provider/middleware/schema"
)

// InferredComponent is a component resource inferred from code.
//
// To create an [InferredComponent], call the [Component] function.
type InferredComponent interface {
	t.ComponentResource
	schema.Resource

	isInferredComponent()
}

func (derivedComponentController[R, I, O]) isInferredComponent() {}

// ComponentFn describes the type signature of a Pulumi component resource defined in Go.
type ComponentFn[A any, R pulumi.ComponentResource] = func(
	ctx *pulumi.Context, name string, args A, opts ...pulumi.ResourceOption,
) (R, error)

// Component creates an [InferredComponent] using functions and types that a existing Pulumi component program
// would have implemented.
//
// fn is the function you would use to construct the program.
//
// See: https://www.pulumi.com/docs/iac/concepts/resources/components/#authoring-a-new-component-resource.
func Component[A any, R pulumi.ComponentResource, F ComponentFn[A, R]](fn F) InferredComponent {
	return &derivedComponentController[R, A, R]{fn}
}

type derivedComponentController[R any, I any, O pulumi.ComponentResource] struct {
	construct ComponentFn[I, O]
}

func (rc *derivedComponentController[R, I, O]) GetSchema(reg schema.RegisterDerivativeType) (
	pschema.ResourceSpec, error) {
	r, err := getResourceSchema[R, I, O](true)
	if err := err.ErrorOrNil(); err != nil {
		return pschema.ResourceSpec{}, err
	}
	if err := registerTypes[I](reg); err != nil {
		return pschema.ResourceSpec{}, err
	}
	if err := registerTypes[O](reg); err != nil {
		return pschema.ResourceSpec{}, err
	}
	return r, nil
}

func (rc *derivedComponentController[R, I, O]) GetToken() (tokens.Type, error) {
	return getToken[R](nil)
}

func (rc *derivedComponentController[R, I, O]) Construct(
	ctx context.Context, req p.ConstructRequest,
) (p.ConstructResponse, error) {
	return req.Construct(ctx,
		func(
			ctx *pulumi.Context, inputs pprovider.ConstructInputs, opts pulumi.ResourceOption,
		) (pulumi.ComponentResource, error) {
			var i I
			urn := req.URN
			err := inputs.CopyTo(&i)
			if err != nil {
				return nil, fmt.Errorf("failed to copy inputs for %s (%s): %w",
					urn.Name(), urn.Type(), err)
			}
			res, err := rc.construct(ctx, urn.Name(), i, opts)
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
		})
}
