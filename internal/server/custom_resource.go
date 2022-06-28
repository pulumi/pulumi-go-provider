package server

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/pulumi/pulumi-go-provider/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

type CustomResources map[tokens.Type]resource.Custom

func NewCustomResources(pkg tokens.Package, resources []resource.Custom) (CustomResources, error) {
	var c CustomResources = map[tokens.Type]resource.Custom{}
	for _, r := range resources {
		urn, err := getToken(pkg, r)
		if err != nil {
			return nil, err
		}
		c[urn] = r
	}
	return c, nil
}

func (c CustomResources) GetCustom(typ tokens.Type) (resource.Custom, error) {
	r, ok := c[typ]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "There is no custom resource ''%s'", typ)
	}

	return newOfType(r), nil
}
