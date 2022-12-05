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
	"fmt"
	"reflect"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/pulumi/pulumi/pkg/v3/codegen/cgstrings"
	pSchema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	pResource "github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	p "github.com/pulumi/pulumi-go-provider"
	t "github.com/pulumi/pulumi-go-provider/middleware"
	"github.com/pulumi/pulumi-go-provider/middleware/cancel"
	"github.com/pulumi/pulumi-go-provider/middleware/dispatch"
	s "github.com/pulumi/pulumi-go-provider/middleware/schema"
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
	r := resource{name, path}
	ps.resources = append(ps.resources, r)
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
	name string
	path *openapi3.PathItem
}

func (r *resource) GetSchema(s.RegisterDerivativeType) (pSchema.ResourceSpec, error) {
	return pSchema.ResourceSpec{}, nil
}

func (r *resource) GetToken() (tokens.Type, error) {
	path := strings.Split(r.name, "/")
	switch len(path) {
	case 0:
		return "", fmt.Errorf("Empty resource name: %#v", r.name)
	case 1:
		return tokens.NewTypeToken(tokens.NewModuleToken("pkg", "index"), cgstrings.Camel(path[0])), nil
	default:
		return tokens.NewTypeToken(
			tokens.NewModuleToken("",
				tokens.ModuleName(strings.Join(path[:len(path)-1], "/"))),
			cgstrings.Camel(path[len(path)-1])), nil
	}
}

func (r *resource) Check(ctx p.Context, req p.CheckRequest) (p.CheckResponse, error) {
	// We need to validate that req can be used to serve any CRUD operation.
	panic("Unimplemented")
}

// The set of inputs for this resource.
func (r *resource) inputs(ctx p.Context) {

}

func (r *resource) Diff(ctx p.Context, req p.DiffRequest) (p.DiffResponse, error) {
	// This default diff is copied from infer.resource. We should generalize this
	// solution.
	objDiff := req.News.Diff(req.Olds)
	pluginDiff := plugin.NewDetailedDiffFromObjectDiff(objDiff)
	diff := map[string]p.PropertyDiff{}
	for k, v := range pluginDiff {
		set := func(kind p.DiffKind) {
			diff[k] = p.PropertyDiff{
				Kind:      kind,
				InputDiff: v.InputDiff,
			}
		}
		if r.path.Put == nil {
			// We force replaces if we don't have access to updates
			v.Kind = v.Kind.AsReplace()
		}
		switch v.Kind {
		case plugin.DiffAdd:
			set(p.Add)
		case plugin.DiffAddReplace:
			set(p.AddReplace)
		case plugin.DiffDelete:
			set(p.Delete)
		case plugin.DiffDeleteReplace:
			set(p.DeleteReplace)
		case plugin.DiffUpdate:
			set(p.Update)
		case plugin.DiffUpdateReplace:
			set(p.UpdateReplace)
		}
	}
	return p.DiffResponse{
		HasChanges:   objDiff.AnyChanges(),
		DetailedDiff: diff,
	}, nil
}

func (r *resource) Create(ctx p.Context, req p.CreateRequest) (p.CreateResponse, error) {
	if t, err := assertPath[p.CreateResponse](req.Urn, "Create", r.path.Post); err != nil {
		return t, err
	}
	return p.CreateResponse{}, nil
}

func (r *resource) Read(ctx p.Context, req p.ReadRequest) (p.ReadResponse, error) {
	if t, err := assertPath[p.ReadResponse](req.Urn, "Create", r.path.Get); err != nil {
		return t, err
	}
	return p.ReadResponse{}, nil
}

func (r *resource) Update(ctx p.Context, req p.UpdateRequest) (p.UpdateResponse, error) {
	if t, err := assertPath[p.UpdateResponse](req.Urn, "Update", r.path.Put); err != nil {
		return t, err
	}
	return p.UpdateResponse{}, nil
}

func (r *resource) Delete(ctx p.Context, req p.DeleteRequest) error {
	if _, err := assertPath[struct{}](req.Urn, "Delete", r.path.Delete); err != nil {
		return err
	}
	return nil
}

func assertPath[T any](urn pResource.URN, operation string, path *openapi3.Operation) (T, error) {
	var t T
	if path == nil {
		return t, status.Errorf(codes.Unimplemented, "%s is not implemented for resource %s", urn)
	}
	return t, nil
}
