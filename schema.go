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

package provider

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/iwahbe/pulumi-go-provider/internal/inputmap"
	"github.com/iwahbe/pulumi-go-provider/internal/introspect"
	"github.com/iwahbe/pulumi-go-provider/resource"
	"github.com/iwahbe/pulumi-go-provider/types"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

const (
	STRING  = "string"
	INT     = "integer"
	BOOL    = "boolean"
	FLOAT   = "number"
	ARRAY   = "array"
	ANY     = "any"
	OBJECT  = "object"
	UNKNOWN = "unknown"
)

type serializationInfo struct {
	pkgname   string
	resources map[reflect.Type]string
	types     map[reflect.Type]string
	enums     map[reflect.Type]string
	inputMap  inputmap.InputToImplementor
}

// Serialize a package to JSON Schema.
func serialize(opts options) (string, error) {
	info := serializationInfo{}
	info.pkgname = opts.Name
	info.resources = make(map[reflect.Type]string)
	info.types = make(map[reflect.Type]string)
	info.enums = make(map[reflect.Type]string)
	info.inputMap = inputmap.GetInputMap()

	for _, resource := range opts.Customs {
		t := baseType(resource)
		tokenType, err := introspect.GetToken(tokens.Package(info.pkgname), resource)
		if err != nil {
			return "", err
		}
		token := tokenType.String()
		info.resources[t] = token
	}

	for _, typ := range opts.Types {
		if enum, ok := typ.(types.Enum); ok {
			instance := reflect.New(enum.Type).Elem().Interface()
			typeToken, err := introspect.GetToken(tokens.Package(info.pkgname), instance)
			if err != nil {
				return "", err
			}
			info.enums[dereference(enum.Type)] = typeToken.String()
			continue
		}
		t := baseType(typ)
		tokenType, err := introspect.GetToken(tokens.Package(info.pkgname), typ)
		if err != nil {
			return "", err
		}
		token := tokenType.String()
		info.types[t] = token
	}

	for _, component := range opts.Components {
		t := baseType(component)
		tokenType, err := introspect.GetToken(tokens.Package(info.pkgname), component)
		if err != nil {
			return "", err
		}
		token := tokenType.String()
		info.resources[t] = token
	}

	pkgSpec, err := info.serializeSchema(opts)
	if err != nil {
		return "", err
	}
	pkgSpec.Language = opts.Language

	schemaJSON, err := json.MarshalIndent(pkgSpec, "", "  ")
	if err != nil {
		return "", err
	}

	return string(schemaJSON), nil
}

// Get the packagespec given resources, etc.
func (info serializationInfo) serializeSchema(opts options) (schema.PackageSpec, error) {
	spec := schema.PackageSpec{}
	spec.Resources = make(map[string]schema.ResourceSpec)
	spec.Types = make(map[string]schema.ComplexTypeSpec)
	spec.Name = opts.Name
	spec.Version = opts.Version.String()

	for _, resource := range opts.Customs {
		resourceSpec, err := info.serializeResource(resource)
		if err != nil {
			return schema.PackageSpec{}, err
		}
		token := info.resources[baseType(resource)]
		spec.Resources[token] = resourceSpec
	}

	for _, component := range opts.Components {
		componentSpec, err := info.serializeResource(component)
		if err != nil {
			return schema.PackageSpec{}, err
		}
		componentSpec.IsComponent = true
		token := info.resources[baseType(component)]
		spec.Resources[token] = componentSpec
	}

	for _, t := range opts.Types {
		if enum, ok := t.(types.Enum); ok {
			enumSpec, err := info.serializeEnumType(enum)
			if err != nil {
				return schema.PackageSpec{}, err
			}
			token, ok := info.enums[dereference(enum.Type)]
			if !ok {
				return schema.PackageSpec{}, fmt.Errorf("internal error: could not find type enum type: %s", dereference(enum.Type))
			}
			spec.Types[token] = enumSpec
			continue
		}
		typeSpec, err := info.serializeComplexType(t)
		if err != nil {
			return schema.PackageSpec{}, err
		}
		token := info.types[baseType(t)]
		spec.Types[token] = typeSpec
	}
	spec, err := mergePackageSpec(spec, opts.PartialSpec)
	if err != nil {
		return schema.PackageSpec{}, err
	}
	return spec, nil
}

func (info serializationInfo) serializeEnumType(enum types.Enum) (schema.ComplexTypeSpec, error) {
	t := dereference(enum.Type)
	kind, _ := getTypeKind(t)
	enumVals := make([]schema.EnumValueSpec, 0, len(enum.Values))
	for _, val := range enum.Values {
		enumVals = append(enumVals, schema.EnumValueSpec{
			Name:  val.Name,
			Value: val.Value,
		})
	}
	spec := schema.ComplexTypeSpec{
		ObjectTypeSpec: schema.ObjectTypeSpec{
			Type: kind,
		},
		Enum: enumVals,
	}

	return spec, nil
}

func mergePackageSpec(spec, over schema.PackageSpec) (schema.PackageSpec, error) {
	if over.Name != "" {
		spec.Name = over.Name
	}
	if over.DisplayName != "" {
		spec.DisplayName = over.DisplayName
	}
	if over.Version != "" {
		spec.Version = over.Version
	}
	if over.Keywords != nil {
		spec.Keywords = mergeStringArrays(spec.Keywords, over.Keywords)
	}
	if over.Homepage != "" {
		spec.Homepage = over.Homepage
	}
	if over.License != "" {
		spec.License = over.License
	}
	if over.Attribution != "" {
		spec.Attribution = over.Attribution
	}
	if over.Repository != "" {
		spec.Repository = over.Repository
	}
	if over.LogoURL != "" {
		spec.LogoURL = over.LogoURL
	}
	if over.PluginDownloadURL != "" {
		spec.PluginDownloadURL = over.PluginDownloadURL
	}
	if over.Publisher != "" {
		spec.Publisher = over.Publisher
	}
	if over.Meta != nil {
		spec.Meta = over.Meta //Meta is a struct containing only one field, so we can just overwrite it
	}
	// AllowedPackageNames []string
	if over.AllowedPackageNames != nil {
		spec.AllowedPackageNames = mergeStringArrays(spec.AllowedPackageNames, over.AllowedPackageNames)
	}
	// Language map[string]RawMessage
	if over.Language != nil {
		spec.Language = mergeMapsOverride(spec.Language, over.Language)
	}
	if over.Config.Variables != nil {
		spec.Config.Variables = mergeMapsOverride(spec.Config.Variables, over.Config.Variables)
	}
	if over.Config.Required != nil {
		spec.Config.Required = mergeStringArrays(spec.Config.Required, over.Config.Required)
	}
	if over.Types != nil {
		spec.Types = mergeMapsOverride(spec.Types, over.Types)
	}
	p, err := mergeResourceSpec(spec.Provider, over.Provider)
	if err != nil {
		return schema.PackageSpec{}, err
	}
	spec.Provider = p
	if over.Resources != nil {
		spec.Resources = mergeMapsOverride(spec.Resources, over.Resources)
	}
	if over.Functions != nil {
		spec.Functions = mergeMapsOverride(spec.Functions, over.Functions)
	}
	return spec, nil
}

//mergeResourceSpec merges two resource specs together.
func mergeResourceSpec(base, over schema.ResourceSpec) (schema.ResourceSpec, error) {
	base.ObjectTypeSpec = mergeObjectTypeSpec(base.ObjectTypeSpec, over.ObjectTypeSpec)

	if over.InputProperties != nil {
		base.InputProperties = mergeMapsOverride(base.InputProperties, over.InputProperties)
	}
	if over.RequiredInputs != nil {
		base.RequiredInputs = mergeStringArrays(base.RequiredInputs, over.RequiredInputs)
	}
	//PlainInputs is deprecated and thus ignored
	if over.StateInputs != nil {
		//StateInputs is a pointer, so for now we're just going to override it.
		//It could also be dereferenced and merged, but for now we'll keep it like this
		base.StateInputs = over.StateInputs
	}
	if over.Aliases != nil {
		aliases, err := mergeStructArraysByKey[schema.AliasSpec, string](base.Aliases, over.Aliases, "Name")
		if err != nil {
			return schema.ResourceSpec{}, err
		}
		base.Aliases = aliases
	}
	if over.DeprecationMessage != "" {
		base.DeprecationMessage = over.DeprecationMessage
	}
	if over.IsComponent {
		base.IsComponent = true
	}
	if over.Methods != nil {
		base.Methods = mergeMapsOverride(base.Methods, over.Methods)
	}
	return base, nil
}

func mergeObjectTypeSpec(base, over schema.ObjectTypeSpec) schema.ObjectTypeSpec {
	if over.Description != "" {
		base.Description = over.Description
	}
	if over.Properties != nil {
		base.Properties = mergeMapsOverride(base.Properties, over.Properties)
	}
	if over.Type != "" {
		base.Type = over.Type
	}
	if over.Required != nil {
		base.Required = mergeStringArrays(base.Required, over.Required)
	}
	//Plain is deprecated and thus ignored
	if over.Language != nil {
		base.Language = mergeMapsOverride(base.Language, over.Language)
	}
	if over.IsOverlay {
		base.IsOverlay = true
	}
	return base
}

// Get the ResourceSpec for a single resource
func (info serializationInfo) serializeResource(rawResource any) (schema.ResourceSpec, error) {
	v := reflect.ValueOf(rawResource)
	for v.Type().Kind() == reflect.Ptr {
		v = v.Elem()
	}

	t := v.Type()
	properties := map[string]schema.PropertySpec{}
	inputProperties := map[string]schema.PropertySpec{}
	requiredInputs := []string{}
	required := []string{}
	descriptions := map[string]string{}
	defaults := map[string]any{}

	if rawResource, ok := rawResource.(resource.Annotated); ok {
		a := introspect.NewAnnotator(rawResource)
		rawResource.Annotate(&a)
		descriptions = a.Descriptions
		defaults = a.Defaults
	}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fieldType := field.Type
		vField := v.Field(i)

		tags, err := introspect.ParseTag(field)
		if err != nil {
			return schema.ResourceSpec{}, err
		}
		if tags.Internal {
			continue
		}

		_, isInputType := vField.Interface().(pulumi.Input)
		_, isOutputType := vField.Interface().(pulumi.Output)

		for fieldType.Kind() == reflect.Ptr {
			fieldType = fieldType.Elem()
		}
		//Need to fetch the underlying types for input and outputs
		if isOutputType {
			fieldType = reflect.New(fieldType).Elem().Interface().(pulumi.Output).ElementType()
		} else if isInputType {
			if _, ok := info.inputMap[fieldType]; !ok {
				return schema.ResourceSpec{}, fmt.Errorf("input %s for property"+
					"%s has type %s, which is not a valid input type", field.Name, t, fieldType)
			}
			fieldType = info.inputMap[fieldType]
		}
		fieldType = dereference(fieldType)
		serialized, err := info.serializeProperty(fieldType, descriptions[tags.Name], defaults[tags.Name])
		if err != nil {
			return schema.ResourceSpec{}, err
		}
		serialized.Secret = tags.Secret
		serialized.ReplaceOnChanges = tags.ReplaceOnChanges
		if !tags.Output {
			inputProperties[tags.Name] = serialized
			if !tags.Optional {
				requiredInputs = append(requiredInputs, tags.Name)
			}
		}
		properties[tags.Name] = serialized
		if !tags.Optional {
			required = append(required, tags.Name)
		}
	}

	return schema.ResourceSpec{
		ObjectTypeSpec: schema.ObjectTypeSpec{
			Properties:  properties,
			Description: descriptions[""],
			Required:    required,
		},
		InputProperties: inputProperties,
		RequiredInputs:  requiredInputs,
	}, nil
}

//Get the propertySpec for a single property
func (info serializationInfo) serializeProperty(t reflect.Type, description string,
	defValue any) (schema.PropertySpec, error) {
	// TODO: add the default
	t = dereference(t)
	typeKind, enum := getTypeKind(t)
	if enum {
		enumSpec, err := info.serializeTypeEnum(t)
		if err != nil {
			return schema.PropertySpec{}, err
		}
		return schema.PropertySpec{
			Description: description,
			Default:     defValue,
			TypeSpec:    enumSpec,
		}, nil
	} else if isTypeOrResource(t, info) {
		typeSpec, err := info.serializeReferenceType(t)
		if err != nil {
			return schema.PropertySpec{}, err
		}
		return schema.PropertySpec{
			Description: description,
			Default:     defValue,
			TypeSpec:    *typeSpec,
		}, nil
	}

	if typeKind == UNKNOWN {
		return schema.PropertySpec{}, fmt.Errorf("failed to serialize property of type %s: unknown typeName", t)
	}

	switch typeKind {
	case ARRAY:
		itemSpec, err := info.serializeTypeAny(t.Elem())
		if err != nil {
			return schema.PropertySpec{}, err
		}
		return schema.PropertySpec{
			Description: description,
			Default:     defValue,
			TypeSpec: schema.TypeSpec{
				Type:  typeKind,
				Items: itemSpec,
			},
		}, nil
	case OBJECT:
		valSpec, err := info.serializeTypeAny(t.Elem())
		if err != nil {
			return schema.PropertySpec{}, err
		}
		if t.Key().Kind() != reflect.String {
			return schema.PropertySpec{}, fmt.Errorf("map keys must be strings")
		}
		return schema.PropertySpec{
			Description: description,
			Default:     defValue,
			TypeSpec: schema.TypeSpec{
				Type:                 "object", //There is no map type in the schema
				AdditionalProperties: valSpec,
			},
		}, nil
	default:
		return schema.PropertySpec{
			Description: description,
			Default:     defValue,
			TypeSpec: schema.TypeSpec{
				Type: typeKind,
			},
		}, nil
	}
}

//Get a TypeSpec which is a reference.
func (info serializationInfo) serializeReferenceType(t reflect.Type) (*schema.TypeSpec, error) {
	t = dereference(t)
	if token, ok := info.resources[t]; ok {
		return &schema.TypeSpec{
			Ref: "#/resources/" + token,
		}, nil
	}
	if token, ok := info.types[t]; ok {
		return &schema.TypeSpec{
			Ref: "#/types/" + token,
		}, nil
	}
	return nil, fmt.Errorf("unknown reference type %s", t)
}

func isTypeOrResource(t reflect.Type, info serializationInfo) bool {
	t = dereference(t)
	_, isResource := info.resources[t]
	if isResource {
		return true
	}
	_, isType := info.types[t]
	return isType
}

func getTypeKind(t reflect.Type) (string, bool) {
	var typeName string
	isEnum := false
	switch t.Kind() {
	case reflect.String:
		typeName = STRING
		isEnum = t.String() != reflect.String.String()
	case reflect.Bool:
		typeName = BOOL
	case reflect.Int:
		typeName = INT
		isEnum = t.String() != reflect.Int.String()
	case reflect.Float64:
		typeName = FLOAT
		isEnum = t.String() != reflect.Float64.String()
	case reflect.Array, reflect.Slice:
		typeName = ARRAY
	case reflect.Map:
		typeName = OBJECT
	case reflect.Ptr:
		//This is a panic because we should always be dereferencing pointers
		//The user should not be able to cause this to happen
		//I removed the behavior where getTypeKind would automatically dereference pointers
		//Because it may be confusing when debugging if getTypeKind is returning non-pointer
		//When t is actually a pointer
		panic("Detected pointer type during serialization - did you forget to dereference?")
	case reflect.Interface:
		typeName = ANY
	default:
		typeName = UNKNOWN
	}
	return typeName, isEnum
}

func (info serializationInfo) serializeTypeAny(t reflect.Type) (*schema.TypeSpec, error) {
	typeKind, enum := getTypeKind(t)
	if enum {
		enumSpec, err := info.serializeTypeEnum(t)
		if err != nil {
			return nil, err
		}
		return &enumSpec, nil
	} else if isTypeOrResource(t, info) {
		return info.serializeReferenceType(t)
	}
	switch typeKind {
	case ARRAY:
		itemSpec, err := info.serializeTypeAny(t.Elem())
		if err != nil {
			return nil, err
		}
		return &schema.TypeSpec{
			Type:  typeKind,
			Items: itemSpec,
		}, nil
	case OBJECT:
		valSpec, err := info.serializeTypeAny(t.Elem())
		if err != nil {
			return nil, err
		}
		if t.Key().Kind() != reflect.String {
			return nil, fmt.Errorf("map keys must be strings")
		}
		return &schema.TypeSpec{
			Type:                 "object", //There is no map type in the schema
			AdditionalProperties: valSpec,
		}, nil
	case ANY:
		return &schema.TypeSpec{
			Ref: "pulumi.json#/Any",
		}, nil
	case UNKNOWN:
		return &schema.TypeSpec{}, fmt.Errorf("unknown type %s", t)
	default:
		return &schema.TypeSpec{
			Type: typeKind,
		}, nil
	}
}

func (info serializationInfo) serializeComplexType(typ any) (schema.ComplexTypeSpec, error) {
	t := reflect.TypeOf(typ)
	t = dereference(t)
	_, enum := getTypeKind(t)
	if enum {
		return schema.ComplexTypeSpec{}, fmt.Errorf("enums are implemented using provider.Enums()")
	}

	descriptions := map[string]string{}
	defaults := map[string]any{}

	if typ, ok := typ.(resource.Annotated); ok {
		a := introspect.NewAnnotator(typ)
		typ.Annotate(&a)
		descriptions = a.Descriptions
		defaults = a.Defaults
	}

	if t.Kind() == reflect.Struct {
		properties := make(map[string]schema.PropertySpec)
		var required []string
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			tags, err := introspect.ParseTag(field)
			if err != nil {
				return schema.ComplexTypeSpec{}, err
			}
			if tags.Internal {
				continue
			}
			if !tags.Optional {
				required = append(required, tags.Name)
			}
			prop, err := info.serializeProperty(field.Type, descriptions[tags.Name], defaults[tags.Name])
			if err != nil {
				return schema.ComplexTypeSpec{}, err
			}
			prop.Secret = tags.Secret
			prop.ReplaceOnChanges = tags.ReplaceOnChanges
			properties[tags.Name] = prop
		}
		return schema.ComplexTypeSpec{
			ObjectTypeSpec: schema.ObjectTypeSpec{
				Type:        "object",
				Properties:  properties,
				Required:    required,
				Description: descriptions[""],
			},
		}, nil
	}

	return schema.ComplexTypeSpec{}, fmt.Errorf("Types must be structs - use provider.Enum to create an enum type %s", t)
}

func (info serializationInfo) serializeTypeEnum(t reflect.Type) (schema.TypeSpec, error) {
	t = dereference(t)
	enum, ok := info.enums[t]
	if !ok {
		return schema.TypeSpec{}, fmt.Errorf("unknown enum type %s", t)
	}
	return schema.TypeSpec{
		Ref: "#/types/" + enum,
	}, nil
}

func mergeStringArrays(base, override []string) []string {
	m := make(map[string]bool)
	for _, x := range base {
		m[x] = true
	}
	//If an element in override is not in base, append it
	for _, y := range override {
		if !m[y] {
			base = append(base, y)
		}
	}
	return base
}

//Merge two arrays of structs which have the string property "Name" by their names
func mergeStructArraysByKey[T interface{}, K comparable](base, override []T, fieldName string) ([]T, error) {
	m := make(map[K]T)
	//Check that type T has field fieldName
	t := reflect.TypeOf((*T)(nil)).Elem()
	field, ok := t.FieldByName(fieldName)
	if !ok {
		return nil, fmt.Errorf("type %s does not have field %s", t, fieldName)
	}
	//Check that field fieldName is of type K
	k := reflect.TypeOf((*K)(nil)).Elem()
	if field.Type != k {
		return nil, fmt.Errorf("type %s field %s is not of type %s", t, fieldName, k)
	}

	for _, x := range base {
		key := reflect.ValueOf(x).FieldByName(fieldName).Interface().(K)
		m[key] = x
	}

	for _, y := range override {
		key := reflect.ValueOf(y).FieldByName(fieldName).Interface().(K)
		if _, ok := m[key]; !ok {
			base = append(base, y)
		}
	}

	return base, nil
}

func mergeMapsOverride[T any](base, override map[string]T) map[string]T {
	for k, v := range override {
		base[k] = v
	}
	return base
}

func baseType(i any) reflect.Type {
	return dereference(reflect.TypeOf(i))
}

func dereference(t reflect.Type) reflect.Type {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t
}
