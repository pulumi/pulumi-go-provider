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

// ComponentResource may be turned into an [InferredComponent] with [Component].
type ComponentResource[I any, O pulumi.ComponentResource] interface {
	// Construct a component resource
	//
	// ctx.RegisterResource needs to be called, but ctx.RegisterOutputs does not need to
	// be called.
	Construct(ctx *pulumi.Context, name, typ string, inputs I, opts pulumi.ResourceOption) (O, error)
}

// InferredComponent is a component resource inferred from code.
//
// To create an [InferredComponent], call the [Component] function.
type InferredComponent interface {
	t.ComponentResource
	schema.Resource

	isInferredComponent()
}

func (derivedComponentController[R, I, O]) isInferredComponent() {}

// Component defines a component resource from go code. Here `R` is the component resource
// anchor, `I` describes its inputs and `O` its outputs. To add descriptions to `R`, `I`
// and `O`, see the `Annotated` trait defined in this module.
func Component[R ComponentResource[I, O], I any, O pulumi.ComponentResource]() InferredComponent {
	return &derivedComponentController[R, I, O]{}
}

type derivedComponentController[R ComponentResource[I, O], I any, O pulumi.ComponentResource] struct{}

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
			var r R
			var i I
			urn := req.URN
			err := inputs.CopyTo(&i)
			if err != nil {
				return nil, fmt.Errorf("failed to copy inputs for %s (%s): %w",
					urn.Name(), urn.Type(), err)
			}
			res, err := r.Construct(ctx,
				urn.Name(),
				urn.Type().String(),
				i, opts)
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

// ** Implementations for creating an [InferredComponent] using existing Pulumi component programs. **

// ComponentFn describes the type signature of a Pulumi custom component resource that users create.
type ComponentFn[I any, O pulumi.ComponentResource] func(
	*pulumi.Context, string, I, ...pulumi.ResourceOption) (O, error)

// ProgramComponent creates an [InferredComponent] using functions and types that a existing Pulumi component program
// would have implemented. The required inputs are the inputs and outputs struct, and the function that creates
// the component resource.
// See: https://www.pulumi.com/docs/iac/concepts/resources/components/#authoring-a-new-component-resource.
func ProgramComponent[I any, O pulumi.ComponentResource](fn ComponentFn[I, O]) InferredComponent {
	return &derivedProgramComponentController[I, O]{
		construct: fn,
	}
}

// derivedProgramComponentController is a controller for a component resource authored as a Pulumi program.
type derivedProgramComponentController[I any, O pulumi.ComponentResource] struct {
	// construct is the function that works on the component resource.
	construct ComponentFn[I, O]
}

func (rc *derivedProgramComponentController[I, O]) GetToken() (tokens.Type, error) {
	return getToken[O](nil)
}

func (derivedProgramComponentController[I, O]) isInferredComponent() {}

func (rc *derivedProgramComponentController[I, O]) GetSchema(reg schema.RegisterDerivativeType) (
	pschema.ResourceSpec, error) {
	r, err := getResourceSchema[O, I, O](true)
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

func (rc *derivedProgramComponentController[I, O]) Construct(ctx context.Context, req p.ConstructRequest,
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

			// Construct the component resource.
			res, err := rc.construct(ctx, urn.Name(), i, opts)
			if err != nil {
				return nil, fmt.Errorf("failed to create component resource %s (%s): %w",
					urn.Name(), urn.Type(), err)
			}

			// Register any outputs.
			m := introspect.StructToMap(res)
			err = ctx.RegisterResourceOutputs(res, pulumi.ToMap(m))
			if err != nil {
				return nil, err
			}

			return res, err
		})
}
