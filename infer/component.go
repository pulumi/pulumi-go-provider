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
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/mapper"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	pprovider "github.com/pulumi/pulumi/sdk/v3/go/pulumi/provider"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/internal/introspect"
	t "github.com/pulumi/pulumi-go-provider/middleware"
	"github.com/pulumi/pulumi-go-provider/middleware/schema"
)

// A component resource.
type ComponentResource[I any, O pulumi.ComponentResource] interface {
	// Construct a component resource
	//
	// ctx.RegisterResource needs to be called, but ctx.RegisterOutputs does not need to
	// be called.
	Construct(ctx *pulumi.Context, name, typ string, inputs I, opts pulumi.ResourceOption) (O, error)
}

// A component resource inferred from code. To get an instance of an InferredComponent,
// call the function Component.
type InferredComponent interface {
	t.ComponentResource
	schema.Resource

	isInferredComponent()
}

func (derivedComponentController[R, I, O]) isInferredComponent() {}

// Define a component resource from go code. Here `R` is the component resource anchor,
// `I` describes its inputs and `O` its outputs. To add descriptions to `R`, `I` and `O`,
// see the `Annotated` trait defined in this module.
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
	var r R
	return introspect.GetToken("pkg", r)
}

func (rc *derivedComponentController[R, I, O]) Construct(
	ctx p.Context, req p.ConstructRequest,
) (p.ConstructResponse, error) {
	// Store the context in itself as a value, so we can retrieve the
	// p.Context later.
	ctx = p.CtxWithValue(ctx, componentContextKey{}, ctx)
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
				urn.Name().String(),
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

type componentContextKey struct{}

// Retrieve a provider.Context from a pulumi.Context.
//
// This function is only valid when the *pulumi.Context was passed in from the infer
// library.
func CtxFromPulumiContext(ctx *pulumi.Context) p.Context {
	v := ctx.Context().Value(componentContextKey{})
	contract.Assertf(v != nil,
		"CtxFromPulumiContext must be called on the pulumi.Context passed in to infer.Component.Construct")
	return v.(p.Context)
}

// RegisterComponentResource registers a custom resource defined by the inferred provider
// that it is called in.
//
// It is not necessary to call RegisterComponentResource for resources not defined in this
// package.
func RegisterComponentResource[R ComponentResource[I, O], I any, O pulumi.ComponentResource](
	ctx *pulumi.Context, name string, args I, opts ...pulumi.ResourceOption) (O, error) {
	var o O
	pCtx := CtxFromPulumiContext(ctx)
	token, err := Component[R, I, O]().GetToken()
	if err != nil {
		return o, err
	}
	token = applyPackage(pCtx.RuntimeInformation(), token)

	inputs, mErr := mapper.New(nil).Encode(args)
	if mErr != nil {
		return o, mErr
	}
	err = ctx.RegisterRemoteComponentResource(token.String(), name,
		pulumi.ToMap(resource.NewPropertyMapFromMap(inputs).Mappable()), o,
		append(opts, pulumi.Version(pCtx.RuntimeInformation().Version))...)
	return o, err
}

// RegisterCustomResource registers a custom resource defined by the inferred provider
// that it is called in.
//
// It is not necessary to call RegisterCustomResource for resources not defined in this
// package.
func RegisterCustomResource[R CustomResource[I, O], I any, O pulumi.CustomResource](
	ctx *pulumi.Context, name string, args I, opts ...pulumi.ResourceOption) (O, error) {
	var o O
	pCtx := CtxFromPulumiContext(ctx)
	token, err := Resource[R, I, O]().GetToken()
	if err != nil {
		return o, err
	}
	token = applyPackage(pCtx.RuntimeInformation(), token)

	inputs, mErr := mapper.New(nil).Encode(args)
	if mErr != nil {
		return o, mErr
	}
	err = ctx.RegisterResource(token.String(), name,
		pulumi.ToMap(resource.NewPropertyMapFromMap(inputs).Mappable()), o,
		append(opts, pulumi.Version(pCtx.RuntimeInformation().Version))...)
	return o, err
}

func applyPackage(info p.RunInfo, t tokens.Type) tokens.Type {
	return tokens.NewTypeToken(
		tokens.NewModuleToken(
			tokens.NewPackageToken(
				tokens.PackageName(info.PackageName),
			),
			t.Module().Name()),
		t.Name(),
	)
}
