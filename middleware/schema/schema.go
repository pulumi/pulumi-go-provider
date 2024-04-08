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

// The schema middleware provides facilities to respond to GetSchema. It handles combining
// multiple resources and functions into a coherent and correct schema. It correctly sets
// the `name` field and the first segment of each token to match the provider name.
package schema

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/hashicorp/go-multierror"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	p "github.com/pulumi/pulumi-go-provider"
)

// When a resource is collecting it's schema, it should register all of the types it uses.
// The function will return `true` if the user should recursively register register used
// types. A return of `false` indicates that the type is already known, and children types
// do not need to be drilled.
type RegisterDerivativeType func(tk tokens.Type, typ schema.ComplexTypeSpec) (unknown bool)

// A Resource that can generate its own schema definition.
type Resource interface {
	// Return the Resource's type token. The first segment of the token is ignored.
	GetToken() (tokens.Type, error)
	// Return the Resource's schema definition. The passed in function should be called on
	// types transitively referenced by the resource. See the documentation of
	// RegisterDerivativeType for more details.
	GetSchema(RegisterDerivativeType) (schema.ResourceSpec, error)
}

// A Function that can generate its own schema definition.
type Function interface {
	// Return the Function's type token. The first segment of the token is ignored.
	GetToken() (tokens.Type, error)
	// Return the Function's schema definition. The passed in function should be called on
	// types transitively referenced by the function. See the documentation of
	// RegisterDerivativeType for more details.
	GetSchema(RegisterDerivativeType) (schema.FunctionSpec, error)
}

type cache struct {
	spec      schema.PackageSpec
	marshaled string
}

func newCacheFromSpec(spec schema.PackageSpec) (*cache, error) {
	bytes, err := json.Marshal(spec)
	if err != nil {
		return nil, err
	}
	return &cache{spec, string(bytes)}, nil
}

func newCacheFromMarshaled(marshaled string) (*cache, error) {
	c := &cache{marshaled: marshaled}
	return c, json.Unmarshal([]byte(marshaled), &c.spec)
}

func (c *cache) isEmpty() bool {
	return c == nil
}

type state struct {
	Options
	// The cached schema. All With* methods should set schema to "", so we regenerate it
	// on the next request.
	schema         *cache
	lowerSchema    *cache
	combinedSchema *cache
	innerGetSchema func(ctx context.Context, req p.GetSchemaRequest) (p.GetSchemaResponse, error)
}

func (s *state) invalidateCache() {
	s.schema = nil
	s.combinedSchema = nil
}

type Options struct {
	Metadata
	// Resources from which to derive the schema
	Resources []Resource
	// Invokes from which to derive the schema
	Invokes []Function
	// The provider resource for the schema
	Provider Resource

	// Map modules in the generated schema.
	//
	// For example, with the map {"foo": "bar"}, the token "pkg:foo:Name" would be present in
	// the schema as "pkg:bar:Name".
	ModuleMap map[tokens.ModuleName]tokens.ModuleName
}

// Metadata describes additional metadata to embed in the generated Pulumi Schema.
type Metadata struct {
	// LanguageMap corresponds to the [schema.PackageSpec.Language] section of the
	// resulting schema.
	//
	// Example:
	//
	// 	import (
	// 		goGen "github.com/pulumi/pulumi/pkg/v3/codegen/go"
	// 		nodejsGen "github.com/pulumi/pulumi/pkg/v3/codegen/nodejs"
	// 		pythonGen "github.com/pulumi/pulumi/pkg/v3/codegen/python"
	// 		csharpGen "github.com/pulumi/pulumi/pkg/v3/codegen/dotnet"
	// 		javaGen " github.com/pulumi/pulumi-java/pkg/codegen/java"
	//	)
	//
	//	Metadata{
	//		LanguageMap: map[string]any{
	//			"go": goGen.GoPackageInfo{
	//				RootPackageName: "go-specific",
	//			},
	//			"nodejs": nodejsGen.NodePackageInfo{
	//				PackageName: "nodejs-specific",
	//			},
	//			"python": pythonGen.PackageInfo{
	//				PackageName: "python-specific",
	//			},
	//			"csharp": csharpGen.CSharpPackageInfo{
	//				RootNamespace: "csharp-specific",
	//			},
	//			"java": javaGen.PackageInfo{
	//				BasePackage: "java-specific",
	//			},
	//		},
	//	}
	//
	// Before embedding, each field is marshaled via [json.Marshal].
	LanguageMap map[string]any
	// Description sets the [schema.PackageSpec.Description] field.
	Description string
	// DisplayName sets the [schema.PackageSpec.DisplayName] field.
	DisplayName string
	// Keywords sets the [schema.PackageSpec.Keywords] field.
	Keywords []string
	// Homepage sets the [schema.PackageSpec.Homepage] field.
	Homepage string
	// Repository sets the [schema.PackageSpec.Repository] field.
	Repository string
	// Publisher sets the [schema.PackageSpec.Publisher] field.
	Publisher string
	// LogoURL sets the [schema.PackageSpec.LogoURL] field.
	LogoURL string
	// License sets the [schema.PackageSpec.License] field.
	License string
	// PluginDownloadURL sets the [schema.PackageSpec.PluginDownloadURL] field.
	PluginDownloadURL string
}

func (m *Metadata) PopulatePackageSpec(pkg *schema.PackageSpec) {
	pkg.DisplayName = m.DisplayName
	pkg.Description = m.Description
	pkg.Keywords = m.Keywords
	pkg.Homepage = m.Homepage
	pkg.Repository = m.Repository
	pkg.Publisher = m.Publisher
	pkg.LogoURL = m.LogoURL
	pkg.License = m.License
	pkg.PluginDownloadURL = m.PluginDownloadURL
}

// Wrap a provider with the facilities to serve GetSchema.
func Wrap(provider p.Provider, opts Options) p.Provider {
	state := state{
		Options:        opts,
		innerGetSchema: provider.GetSchema,
	}
	provider.GetSchema = state.GetSchema
	return provider
}

func (s *state) GetSchema(ctx context.Context, req p.GetSchemaRequest) (p.GetSchemaResponse, error) {
	if s.schema.isEmpty() {
		spec, err := s.generateSchema(ctx)
		if err != nil {
			return p.GetSchemaResponse{}, err
		}
		s.schema, err = newCacheFromSpec(spec)
		if err != nil {
			return p.GetSchemaResponse{}, err
		}
	}
	if s.innerGetSchema != nil {
		lower, err := s.innerGetSchema(ctx, req)
		if err == nil {
			// We need to merge
			// Make sure our caches are up to date
			if s.lowerSchema.isEmpty() || s.lowerSchema.marshaled != lower.Schema {
				s.combinedSchema = nil
				s.lowerSchema, err = newCacheFromMarshaled(lower.Schema)
				if err != nil {
					return p.GetSchemaResponse{}, err
				}
			}
		} else if status.Code(err) == codes.Unimplemented {
			s.lowerSchema = nil
		} else {
			// There was an actual error, so we need to buble that up.
			return p.GetSchemaResponse{}, err
		}
	} else {
		s.lowerSchema = nil
	}

	err := s.mergeSchemas()
	if err != nil {
		return p.GetSchemaResponse{}, err
	}
	return p.GetSchemaResponse{
		Schema: s.combinedSchema.marshaled,
	}, nil
}

func (s *state) mergeSchemas() error {
	contract.Assertf(!s.schema.isEmpty(), "we must have our own schema")
	if s.combinedSchema != nil {
		return nil
	}
	if s.lowerSchema == nil {
		s.combinedSchema = s.schema
		return nil
	}

	var merge func(dst, src reflect.Value)
	merge = func(dst, src reflect.Value) {
		contract.Assertf(dst.IsValid(), "dst not valid")
		contract.Assertf(dst.CanAddr(), "we need to be able to assign to dst (%s)", dst)
		switch dst.Type().Kind() {
		case reflect.Pointer:
			if src.IsNil() {
				return
			}
			if dst.IsNil() {
				dst.Set(src)
				return
			}
			merge(dst.Elem(), src.Elem())
		case reflect.Map:
			if src.IsNil() {
				return
			}
			if dst.IsNil() {
				dst.Set(src)
			} else {
				for iter := src.MapRange(); iter.Next(); {
					dst.SetMapIndex(iter.Key(), iter.Value())
				}
			}
			// These types we just copy over
		default:
			if !src.IsZero() {
				dst.Set(src)
			}
		}
	}
	combined := s.lowerSchema.spec
	dst := reflect.ValueOf(&combined).Elem()
	src := reflect.ValueOf(s.schema.spec)
	for i := 0; i < dst.Type().NumField(); i++ {
		if !dst.Type().Field(i).IsExported() {
			continue
		}
		merge(dst.Field(i), src.Field(i))
	}
	var err error
	s.combinedSchema, err = newCacheFromSpec(combined)
	return err
}

// Generate a schema string from the currently present schema types.
func (s *state) generateSchema(ctx context.Context) (schema.PackageSpec, error) {
	info := p.GetRunInfo(ctx)
	pkg := schema.PackageSpec{
		Name:      info.PackageName,
		Version:   info.Version,
		Resources: map[string]schema.ResourceSpec{},
		Functions: map[string]schema.FunctionSpec{},
		Types:     map[string]schema.ComplexTypeSpec{},
		Language:  map[string]schema.RawMessage{},
	}
	s.PopulatePackageSpec(&pkg)

	for k, v := range s.LanguageMap {
		bytes, err := json.Marshal(v)
		if err != nil {
			return schema.PackageSpec{}, err
		}
		pkg.Language[k] = bytes
	}
	registerDerivative := func(tk tokens.Type, t schema.ComplexTypeSpec) bool {
		tkString := assignTo(tk, info.PackageName, s.ModuleMap).String()
		_, ok := pkg.Types[tkString]
		if ok {
			return false
		}
		pkg.Types[tkString] = renamePackage(t, info.PackageName, s.ModuleMap)
		return true
	}
	errs := addElements(s.Resources, pkg.Resources, info.PackageName, registerDerivative, s.ModuleMap)
	e := addElements(s.Invokes, pkg.Functions, info.PackageName, registerDerivative, s.ModuleMap)
	errs.Errors = append(errs.Errors, e.Errors...)

	if s.Provider != nil {
		_, prov, err := addElement[Resource, schema.ResourceSpec](
			info.PackageName, registerDerivative, s.ModuleMap, s.Provider)
		if err != nil {
			errs.Errors = append(errs.Errors, err)
		}
		pkg.Provider = prov
		pkg.Config = schema.ConfigSpec{
			Variables: prov.InputProperties,
			Required:  prov.RequiredInputs,
		}
	}
	if err := errs.ErrorOrNil(); err != nil {
		return schema.PackageSpec{}, err
	}
	return pkg, nil
}

type canGetSchema[T any] interface {
	GetToken() (tokens.Type, error)
	GetSchema(RegisterDerivativeType) (T, error)
}

func addElements[T canGetSchema[S], S any](els []T, m map[string]S,
	pkgName string, reg RegisterDerivativeType,
	modMap map[tokens.ModuleName]tokens.ModuleName) multierror.Error {
	errs := multierror.Error{}
	for _, f := range els {
		tk, element, err := addElement[T, S](pkgName, reg, modMap, f)
		if err != nil {
			errs.Errors = append(errs.Errors, err)
			continue
		}
		m[tk.String()] = element
	}
	return errs
}

func addElement[T canGetSchema[S], S any](pkgName string, reg RegisterDerivativeType,
	modMap map[tokens.ModuleName]tokens.ModuleName, f T) (tokens.Type, S, error) {
	var s S
	tk, err := f.GetToken()
	if err != nil {
		return "", s, err
	}
	tk = assignTo(tk, pkgName, modMap)
	fun, err := f.GetSchema(reg)
	if err != nil {
		return "", s, fmt.Errorf("failed to get schema for '%s': %w", tk, err)
	}
	return tk, renamePackage(fun, pkgName, modMap), nil
}

func assignTo(tk tokens.Type, pkg string, modMap map[tokens.ModuleName]tokens.ModuleName) tokens.Type {
	mod := tk.Module().Name()
	if m, ok := modMap[mod]; ok {
		mod = m
	}
	return tokens.NewTypeToken(tokens.NewModuleToken(tokens.Package(pkg), mod), tk.Name())
}

func fixReference(ref, pkg string, modMap map[tokens.ModuleName]tokens.ModuleName) string {
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
	return kind + string(assignTo(tk, pkg, modMap))
}

// renamePackage sets internal package references to point to the package with the name
// `pkg`.
func renamePackage[T any](typ T, pkg string, modMap map[tokens.ModuleName]tokens.ModuleName) T {
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
				rewritten := fixReference(field.String(), pkg, modMap)
				field.SetString(rewritten)
			}
			for _, f := range reflect.VisibleFields(v.Type()) {
				f := v.FieldByIndex(f.Index)
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
