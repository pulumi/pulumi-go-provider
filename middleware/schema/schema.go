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

package schema

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/hashicorp/go-multierror"
	p "github.com/pulumi/pulumi-go-provider"
	t "github.com/pulumi/pulumi-go-provider/middleware"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

type Resource interface {
	GetSchema() (schema.ResourceSpec, error)
	GetToken() (tokens.Type, error)
}

type Invokes interface {
	GetToken() (tokens.Type, error)
	GetSchema() (schema.FunctionSpec, error)
}

type Provider struct {
	p.Provider

	resources []Resource
	invokes   []Invokes
	schema    string
}

func Wrap(provider p.Provider) *Provider {
	if provider == nil {
		provider = &t.Scaffold{}
	}
	return &Provider{
		Provider: provider,
	}
}

func (s *Provider) WithResources(resources ...Resource) *Provider {
	s.schema = ""
	s.resources = append(s.resources, resources...)
	return s
}

func (s *Provider) WithInvokes(invokes ...Invokes) *Provider {
	s.schema = ""
	s.invokes = append(s.invokes, invokes...)
	return s
}

func (s *Provider) GetSchema(ctx p.Context, req p.GetSchemaRequest) (p.GetSchemaResponse, error) {
	if s.schema == "" {
		err := s.generateSchema(ctx)
		if err != nil {
			return p.GetSchemaResponse{}, err
		}
	}
	return p.GetSchemaResponse{
		Schema: s.schema,
	}, nil
}

// Generate a schema string from the currently present schema types.
func (s *Provider) generateSchema(ctx p.Context) error {
	info := ctx.RuntimeInformation()
	pkg := schema.PackageSpec{
		Name:      info.PackageName,
		Version:   info.Version,
		Resources: map[string]schema.ResourceSpec{},
		Functions: map[string]schema.FunctionSpec{},
	}

	errs := addElement(s.resources, pkg.Resources, info.PackageName)
	e := addElement(s.invokes, pkg.Functions, info.PackageName)
	errs.Errors = append(errs.Errors, e.Errors...)
	if err := errs.ErrorOrNil(); err != nil {
		return err
	}

	bytes, err := json.Marshal(pkg)
	if err != nil {
		return err
	}
	s.schema = string(bytes)
	return nil
}

type canGetSchema[T any] interface {
	GetToken() (tokens.Type, error)
	GetSchema() (T, error)
}

func addElement[T canGetSchema[S], S any](els []T, m map[string]S, pkgName string) multierror.Error {
	errs := multierror.Error{}
	for _, f := range els {
		tk, err := f.GetToken()
		if err != nil {
			errs.Errors = append(errs.Errors, err)
			continue
		}
		tk = assignTo(tk, pkgName)
		fun, err := f.GetSchema()
		if err != nil {
			errs.Errors = append(errs.Errors, fmt.Errorf("failed to get schema for '%s': %w", tk, err))
			continue
		}
		m[tk.String()] = renamePackage(fun, pkgName)
	}
	return multierror.Error{}
}

func assignTo(tk tokens.Type, pkg string) tokens.Type {
	return tokens.NewTypeToken(tokens.NewModuleToken(tokens.Package(pkg), tk.Module().Name()), tk.Name())
}

func fixReference(ref, pkg string) string {
	if !strings.HasPrefix(ref, "#/") {
		// Not an internal reference, so we don't rewrite
		return ref
	}
	s := strings.TrimPrefix(ref, "#/")
	i := strings.IndexRune(s, '/')
	if i == -1 {
		// Not a valid reference, so just leave it
		return ref
	}
	kind := ref[:i+3]
	tk, err := tokens.ParseTypeToken(s[i+2:])
	if err != nil {
		// Not a valid token, so again we just leave it
		return ref
	}
	return kind + string(assignTo(tk, pkg))
}

// renamePackage sets internal package references to point to the package with the name
// `pkg`.
func renamePackage[T any](typ T, pkg string) T {
	var rename func(reflect.Value)
	rename = func(v reflect.Value) {
		switch v.Kind() {
		case reflect.Pointer:
			if v.IsNil() {
				return
			}
			rename(v.Elem())
		case reflect.Struct:
			if v.Type() == reflect.TypeOf(schema.TypeSpec{}) {
				field := v.FieldByName("Ref")
				rewritten := fixReference(field.String(), pkg)
				field.SetString(rewritten)
			}
			for i := 0; i < v.Type().NumField(); i++ {
				f := v.Field(i)
				rename(f)
			}
		case reflect.Array, reflect.Slice:
			for i := 0; i < v.Len(); i++ {
				rename(v.Index(i))
			}
		case reflect.Map:
			m := map[reflect.Value]reflect.Value{}
			for iter := v.MapRange(); iter.Next(); {
				i := iter.Value()
				m[iter.Key()] = i
			}
			for k, e := range m {
				ptr := reflect.New(e.Type())
				ptr.Elem().Set(e)
				rename(ptr)
				v.SetMapIndex(k, ptr.Elem())
			}
		}
	}
	t := &typ
	v := reflect.ValueOf(t)
	rename(v)
	return *t
}
