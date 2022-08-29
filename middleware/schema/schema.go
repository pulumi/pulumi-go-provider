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
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	p "github.com/pulumi/pulumi-go-provider"
	t "github.com/pulumi/pulumi-go-provider/middleware"
)

// When a resource is collecting it's schema, it should register all of the types it uses.
// The function will return `true` if the user should recursively register register used
// types. A return of `false` indicates that the type is already known, and children types
// do not need to be drilled.
type RegisterDerivativeType func(tk tokens.Type, typ schema.ComplexTypeSpec) (unknown bool)

type Resource interface {
	GetToken() (tokens.Type, error)
	GetSchema(RegisterDerivativeType) (schema.ResourceSpec, error)
}

type Function interface {
	GetToken() (tokens.Type, error)
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

type Provider struct {
	p.Provider

	// Resources from which to derive the schema
	resources []Resource
	invokes   []Function
	provider  Resource

	// The cached schema. All With* methods should set schema to "", so we regenerate it
	// on the next request.
	schema         *cache
	lowerSchema    *cache
	combinedSchema *cache

	// Non-inferrable schema fields
	languages         map[string]any
	description       string
	displayName       string
	keywords          []string
	homepage          string
	repository        string
	publisher         string
	logoURL           string
	license           string
	pluginDownloadURL string

	moduleMap map[tokens.ModuleName]tokens.ModuleName
}

func (s *Provider) invalidateCache() {
	s.schema = nil
	s.combinedSchema = nil
}

func Wrap(provider p.Provider) *Provider {
	if provider == nil {
		provider = &t.Scaffold{}
	}
	return &Provider{
		Provider:  provider,
		moduleMap: map[tokens.ModuleName]tokens.ModuleName{},
		languages: map[string]any{},
	}
}

func (s *Provider) WithResources(resources ...Resource) *Provider {
	s.invalidateCache()
	s.resources = append(s.resources, resources...)
	return s
}

func (s *Provider) WithInvokes(invokes ...Function) *Provider {
	s.invalidateCache()
	s.invokes = append(s.invokes, invokes...)
	return s
}

func (s *Provider) WithModuleMap(m map[tokens.ModuleName]tokens.ModuleName) *Provider {
	s.invalidateCache()
	for k, v := range m {
		s.moduleMap[k] = v
	}
	return s
}

func (s *Provider) WithProviderResource(provider Resource) *Provider {
	s.invalidateCache()
	s.provider = provider
	return s
}

func (s *Provider) WithLanguageMap(languages map[string]any) *Provider {
	s.invalidateCache()
	for k, v := range languages {
		s.languages[k] = v
	}
	return s
}

func (s *Provider) WithDescription(description string) *Provider {
	s.invalidateCache()
	s.description = description
	return s
}

func (s *Provider) WithLicense(license string) *Provider {
	s.invalidateCache()
	s.license = license
	return s
}

func (s *Provider) WithPluginDownloadURL(pluginDownloadURL string) *Provider {
	s.invalidateCache()
	s.pluginDownloadURL = pluginDownloadURL
	return s
}

func (s *Provider) WithDisplayName(name string) *Provider {
	s.invalidateCache()
	s.displayName = name
	return s
}

func (s *Provider) WithKeywords(keywords []string) *Provider {
	s.invalidateCache()
	s.keywords = append(s.keywords, keywords...)
	return s
}

func (s *Provider) WithHomepage(homepage string) *Provider {
	s.invalidateCache()
	s.homepage = homepage
	return s
}

func (s *Provider) WithRepository(repoURL string) *Provider {
	s.invalidateCache()
	s.repository = repoURL
	return s
}

func (s *Provider) WithPublisher(publisher string) *Provider {
	s.invalidateCache()
	s.publisher = publisher
	return s
}

func (s *Provider) WithLogoURL(logoURL string) *Provider {
	s.invalidateCache()
	s.logoURL = logoURL
	return s
}

func (s *Provider) GetSchema(ctx p.Context, req p.GetSchemaRequest) (p.GetSchemaResponse, error) {
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
	lower, err := s.Provider.GetSchema(ctx, req)
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
	err = s.mergeSchemas()
	if err != nil {
		return p.GetSchemaResponse{}, err
	}
	return p.GetSchemaResponse{
		Schema: s.combinedSchema.marshaled,
	}, nil
}

func (s *Provider) mergeSchemas() error {
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
func (s *Provider) generateSchema(ctx p.Context) (schema.PackageSpec, error) {
	info := ctx.RuntimeInformation()
	pkg := schema.PackageSpec{
		Name:              info.PackageName,
		Version:           info.Version,
		DisplayName:       s.displayName,
		Description:       s.description,
		Keywords:          s.keywords,
		Homepage:          s.homepage,
		Repository:        s.repository,
		Publisher:         s.publisher,
		LogoURL:           s.logoURL,
		License:           s.license,
		PluginDownloadURL: s.pluginDownloadURL,
		Resources:         map[string]schema.ResourceSpec{},
		Functions:         map[string]schema.FunctionSpec{},
		Types:             map[string]schema.ComplexTypeSpec{},
		Language:          map[string]schema.RawMessage{},
	}
	for k, v := range s.languages {
		bytes, err := json.Marshal(v)
		if err != nil {
			return schema.PackageSpec{}, err
		}
		pkg.Language[k] = bytes
	}
	registerDerivative := func(tk tokens.Type, t schema.ComplexTypeSpec) bool {
		tkString := assignTo(tk, info.PackageName, s.moduleMap).String()
		_, ok := pkg.Types[tkString]
		if ok {
			return false
		}
		pkg.Types[tkString] = renamePackage(t, info.PackageName, s.moduleMap)
		return true
	}
	errs := addElements(s.resources, pkg.Resources, info.PackageName, registerDerivative, s.moduleMap)
	e := addElements(s.invokes, pkg.Functions, info.PackageName, registerDerivative, s.moduleMap)
	errs.Errors = append(errs.Errors, e.Errors...)

	if s.provider != nil {
		_, prov, err := addElement[Resource, schema.ResourceSpec](
			info.PackageName, registerDerivative, s.moduleMap, s.provider)
		if err != nil {
			errs.Errors = append(errs.Errors, err)
		}
		pkg.Provider = prov
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
