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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/pulumi/pulumi-go-provider/internal/introspect"
	"github.com/pulumi/pulumi-go-provider/resource"
)

type CustomResources map[tokens.Type]reflect.Type

func NewCustomResources(pkg tokens.Package, resources []resource.Custom) (CustomResources, error) {
	var c CustomResources = map[tokens.Type]reflect.Type{}
	for _, r := range resources {
		urn, err := introspect.GetToken(pkg, r)
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
		return nil, status.Errorf(codes.NotFound, "There is no custom resource '%s'", typ)
	}

	return reflect.New(r).Interface().(resource.Custom), nil
}
