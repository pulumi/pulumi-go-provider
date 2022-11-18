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

// Create a Pulumi provider by deriving from an OpenAPI schema.
package openapi

import (
	"github.com/getkin/kin-openapi/openapi3"
	p "github.com/pulumi/pulumi-go-provider"
	t "github.com/pulumi/pulumi-go-provider/middleware"
	"github.com/pulumi/pulumi-go-provider/middleware/cancel"
	"github.com/pulumi/pulumi-go-provider/middleware/dispatch"
	s "github.com/pulumi/pulumi-go-provider/middleware/schema"
	pSchema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

type Options struct {
	s.Metadata
}

func V3(schema []byte, opts Options) (p.Provider, error) {
	loader := openapi3.NewLoader()
	t, err := loader.LoadFromData(schema)
	if err != nil {
		return p.Provider{}, err
	}
	spec := collect(t, opts)
	provider := dispatch.Wrap(p.Provider{}, spec.dispatch())
	provider = s.Wrap(p.Provider{}, spec.schema())
	return cancel.Wrap(provider), nil
}

type providerSpec struct {
	opts Options
	t    *openapi3.T

	resources []resource
}

func collect(t *openapi3.T, opts Options) providerSpec {
	spec := providerSpec{opts: opts, t: t}
	for name, path := range t.Paths {
		switch getItemType(path) {
		case resourceType:
			spec.collectResource(name, path)
		}
	}
	return spec
}

func (ps providerSpec) schema() s.Options {
	return s.Options{
		Metadata:  ps.opts.Metadata,
		Resources: []s.Resource{},
		Invokes:   []s.Function{},
		Provider:  nil,
		ModuleMap: map[tokens.ModuleName]tokens.ModuleName{},
	}
}

func (ps providerSpec) dispatch() dispatch.Options {
	return dispatch.Options{
		Customs:    map[tokens.Type]t.CustomResource{},
		Components: map[tokens.Type]t.ComponentResource{},
		Invokes:    map[tokens.Type]t.Invoke{},
		ModuleMap:  map[tokens.ModuleName]tokens.ModuleName{},
	}
}

func (ps *providerSpec) collectResource(name string, path *openapi3.PathItem) {

}

type itemType int

const (
	unknownType  itemType = 0
	resourceType          = iota
	functionType          = iota
)

func getItemType(path *openapi3.PathItem) itemType {
	if path.Post != nil && path.Delete != nil {
		return resourceType
	} else if path.Get != nil {
		return functionType
	}
	return unknownType
}

var _ = (s.Resource)((*resource)(nil))
var _ = (t.CustomResource)((*resource)(nil))

type resource struct {
	path *openapi3.PathItem
}

func (r *resource) GetSchema(s.RegisterDerivativeType) (pSchema.ResourceSpec, error) {
	return pSchema.ResourceSpec{}, nil
}

func (r *resource) GetToken() (tokens.Type, error) {
	return "", nil
}

func (r *resource) Check(p.Context, p.CheckRequest) (p.CheckResponse, error) {
	return p.CheckResponse{}, nil

}

func (r *resource) Diff(p.Context, p.DiffRequest) (p.DiffResponse, error) {
	return p.DiffResponse{}, nil
}

func (r *resource) Create(p.Context, p.CreateRequest) (p.CreateResponse, error) {
	return p.CreateResponse{}, nil
}

func (r *resource) Read(p.Context, p.ReadRequest) (p.ReadResponse, error) {
	return p.ReadResponse{}, nil
}

func (r *resource) Update(p.Context, p.UpdateRequest) (p.UpdateResponse, error) {
	return p.UpdateResponse{}, nil
}

func (r *resource) Delete(p.Context, p.DeleteRequest) error {
	return nil
}
