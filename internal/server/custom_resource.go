package server

import (
	"reflect"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/pulumi/pulumi-go-provider/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

type CustomResources map[tokens.Type]reflect.Type

func NewCustomResources(pkg tokens.Package, resources []resource.Custom) (CustomResources, error) {
	var c CustomResources = map[tokens.Type]reflect.Type{}
	for _, r := range resources {
		urn, err := getToken(pkg, r)
		if err != nil {
			return nil, err
		}
		typ := reflect.TypeOf(r)
		for typ.Kind() == reflect.Pointer {
			typ = typ.Elem()
		}
		c[urn] = typ
	}
	return c, nil
}

func (c CustomResources) GetCustom(typ tokens.Type) (resource.Custom, error) {
	r, ok := c[typ]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "There is no custom resource ''%s'", typ)
	}

	return reflect.New(r).Interface().(resource.Custom), nil
}
