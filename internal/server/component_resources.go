package server

import (
	"fmt"
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
		urn, err := getToken(pkg, comp)
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
		ctx.RegisterComponentResource(typ, name, c, opts)
		err := inputs.CopyTo(c)
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
