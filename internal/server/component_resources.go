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

package server

import (
	"reflect"

	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/provider"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/pulumi/pulumi-go-provider/internal/introspect"
	"github.com/pulumi/pulumi-go-provider/resource"
)

type ComponentResources map[tokens.Type]reflect.Type

func NewComponentResources(pkg tokens.Package, components []resource.Component) (ComponentResources, error) {
	var c ComponentResources = map[tokens.Type]reflect.Type{}
	for _, comp := range components {
		urn, err := introspect.GetToken(pkg, comp)
		if err != nil {
			return nil, err
		}
		typ := reflect.TypeOf(comp)
		for typ.Kind() == reflect.Pointer {
			typ = typ.Elem()
		}
		c[urn] = typ
	}
	return c, nil
}

func (c ComponentResources) GetComponent(typ tokens.Type) (resource.Component, error) {
	// TODO: Work with aliases
	comp, ok := c[typ]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "There is no component resource '%s'.", typ)
	}

	return reflect.New(comp).Interface().(resource.Component), nil
}

func componentFn(pkg string, c resource.Component) provider.ConstructFunc {
	return func(ctx *pulumi.Context, typ, name string, inputs provider.ConstructInputs,
		opts pulumi.ResourceOption) (*provider.ConstructResult, error) {
		err := ctx.RegisterComponentResource(typ, name, c, opts)
		if err != nil {
			return nil, err
		}
		err = inputs.CopyTo(c)
		if err != nil {
			return nil, err
		}

		err = c.Construct(name, ctx)
		if err != nil {
			return nil, err
		}
		m := pulumi.ToMap(introspect.StructToMap(c))
		err = ctx.RegisterResourceOutputs(c, m)
		if err != nil {
			return nil, err
		}
		return provider.NewConstructResult(c)
	}

}
