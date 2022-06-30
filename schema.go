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
	"errors"
	"fmt"
	"reflect"

	"github.com/pulumi/pulumi-go-provider/internal/introspect"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

var ignore = []string{"resource.Custom", "pulumi.ResourceState", "ResourceState"}

const (
	STRING  = "string"
	INT     = "integer"
	BOOL    = "boolean"
	FLOAT   = "number"
	ARRAY   = "array"
	OBJECT  = "object"
	UNKNOWN = "unknown"
)

type serializationInfo struct {
	pkgname   string
	resources map[reflect.Type]string
	types     map[reflect.Type]string
	inputMap  inputToImplementor
}

// Serialize a package to JSON Schema.
func serialize(opts options) (string, error) {
	pkgSpec, err := serializeSchema(opts)
	if err != nil {
		return "", err
	}

	schemaJSON, err := json.MarshalIndent(pkgSpec, "", "  ")
	if err != nil {
		return "", err
	}
	return string(schemaJSON), nil
}

// Get the packagespec given resources, etc.
func serializeSchema(opts options) (schema.PackageSpec, error) {
	spec := schema.PackageSpec{}
	spec.Resources = make(map[string]schema.ResourceSpec)
	spec.Types = make(map[string]schema.ComplexTypeSpec)
	spec.Name = opts.Name
	spec.Version = opts.Version.String()

	info := serializationInfo{}
	info.pkgname = opts.Name
	info.resources = make(map[reflect.Type]string)
	info.types = make(map[reflect.Type]string)
	info.inputMap = initializeInputMap()

	for _, resource := range opts.Resources {
		t := reflect.TypeOf(resource)
		t = dereference(t)
		tokenType, err := introspect.GetToken(tokens.Package(info.pkgname), resource)
		if err != nil {
			return schema.PackageSpec{}, err
		}
		token := tokenType.String()
		info.resources[t] = token
	}

	//"type" is a keyword
	for _, type_ := range opts.Types {
		t := reflect.TypeOf(type_)
		t = dereference(t)
		tokenType, err := introspect.GetToken(tokens.Package(info.pkgname), type_)
		if err != nil {
			return schema.PackageSpec{}, err
		}
		token := tokenType.String()
		info.types[t] = token
	}
	for _, component := range opts.Components {
		t := reflect.TypeOf(component)
		t = dereference(t)
		tokenType, err := introspect.GetToken(tokens.Package(info.pkgname), component)
		if err != nil {
			return schema.PackageSpec{}, err
		}
		token := tokenType.String()
		info.resources[t] = token
	}

	for _, resource := range opts.Resources {
		resourceSpec, err := serializeResource(resource, info)
		if err != nil {
			return schema.PackageSpec{}, err
		}
		token := info.resources[dereference(reflect.TypeOf(resource))]
		spec.Resources[token] = resourceSpec
	}
	//Components are essentially resources, I don't believe they are differentiated in the schema
	for _, component := range opts.Components {
		componentSpec, err := serializeResource(component, info)
		if err != nil {
			return schema.PackageSpec{}, err
		}
		token := info.resources[dereference(reflect.TypeOf(component))]
		spec.Resources[token] = componentSpec
	}

	for _, t := range opts.Types {
		typeSpec, err := serializeType(t, info)
		if err != nil {
			return schema.PackageSpec{}, err
		}
		token := info.types[dereference(reflect.TypeOf(t))]
		spec.Types[token] = typeSpec
	}
	over := opts.PartialSpec
	return mergePackageSpec(spec, over), nil
}

func initializeInputMap() inputToImplementor {
	var inputMap inputToImplementor = make(map[reflect.Type]reflect.Type)
	//IntInput to int
	inputMap.add((*pulumi.IntInput)(nil), (*int)(nil))

	//IntPtrInput to *int
	inputMap.add((*pulumi.IntPtrInput)(nil), (**int)(nil))

	//IntArrayInput to []int
	inputMap.add((*pulumi.IntArrayInput)(nil), (*[]int)(nil))

	//IntMapInput to map[string]int
	inputMap.add((*pulumi.IntMapInput)(nil), (*map[string]int)(nil))

	//IntArrayMapInput to map[string][]int
	inputMap.add((*pulumi.IntArrayMapInput)(nil), (*map[string][]int)(nil))

	//IntMapArrayInput to []map[string]int
	inputMap.add((*pulumi.IntMapArrayInput)(nil), (*[]map[string]int)(nil))

	//IntMapMapInput to map[string]map[string]int
	inputMap.add((*pulumi.IntMapMapInput)(nil), (*map[string]map[string]int)(nil))

	//IntArrayArrayInput to [][]int
	inputMap.add((*pulumi.IntArrayArrayInput)(nil), (*[][]int)(nil))

	//StringInput to string
	inputMap.add((*pulumi.StringInput)(nil), (*string)(nil))

	//StringPtrInput to *string
	inputMap.add((*pulumi.StringPtrInput)(nil), (**string)(nil))

	//StringArrayInput to []string
	inputMap.add((*pulumi.StringArrayInput)(nil), (*[]string)(nil))

	//StringMapInput to map[string]string
	inputMap.add((*pulumi.StringMapInput)(nil), (*map[string]string)(nil))

	//StringArrayMapInput to map[string][]string
	inputMap.add((*pulumi.StringArrayMapInput)(nil), (*map[string][]string)(nil))

	//StringMapArrayInput to []map[string]string
	inputMap.add((*pulumi.StringMapArrayInput)(nil), (*[]map[string]string)(nil))

	//StringMapMapInput to map[string]map[string]string
	inputMap.add((*pulumi.StringMapMapInput)(nil), (*map[string]map[string]string)(nil))

	//StringArrayArrayInput to [][]string
	inputMap.add((*pulumi.StringArrayArrayInput)(nil), (*[][]string)(nil))

	//URNInput to pulumi.URN
	inputMap.add((*pulumi.URNInput)(nil), (*pulumi.URN)(nil))

	//URNPtrInput to *pulumi.URN
	inputMap.add((*pulumi.URNPtrInput)(nil), (**pulumi.URN)(nil))

	//URNArrayInput to []pulumi.URN
	inputMap.add((*pulumi.URNArrayInput)(nil), (*[]pulumi.URN)(nil))

	//URNMapInput to map[string]pulumi.URN
	inputMap.add((*pulumi.URNMapInput)(nil), (*map[string]pulumi.URN)(nil))

	//URNArrayMapInput to map[string][]pulumi.URN
	inputMap.add((*pulumi.URNArrayMapInput)(nil), (*map[string][]pulumi.URN)(nil))

	//URNMapArrayInput to []map[string]pulumi.URN
	inputMap.add((*pulumi.URNMapArrayInput)(nil), (*[]map[string]pulumi.URN)(nil))

	//URNMapMapInput to map[string]map[string]pulumi.URN
	inputMap.add((*pulumi.URNMapMapInput)(nil), (*map[string]map[string]pulumi.URN)(nil))

	//URNArrayArrayInput to [][]pulumi.URN
	inputMap.add((*pulumi.URNArrayArrayInput)(nil), (*[][]pulumi.URN)(nil))

	//ArchiveInput to pulumi.Archive
	inputMap.add((*pulumi.ArchiveInput)(nil), (*pulumi.Archive)(nil))

	//ArchiveArrayInput to []pulumi.Archive
	inputMap.add((*pulumi.ArchiveArrayInput)(nil), (*[]pulumi.Archive)(nil))

	//ArchiveMapInput to map[string]pulumi.Archive
	inputMap.add((*pulumi.ArchiveMapInput)(nil), (*map[string]pulumi.Archive)(nil))

	//ArchiveArrayMapInput to map[string][]pulumi.Archive
	inputMap.add((*pulumi.ArchiveArrayMapInput)(nil), (*map[string][]pulumi.Archive)(nil))

	//ArchiveMapArrayInput to []map[string]pulumi.Archive
	inputMap.add((*pulumi.ArchiveMapArrayInput)(nil), (*[]map[string]pulumi.Archive)(nil))

	//ArchiveMapMapInput to map[string]map[string]pulumi.Archive
	inputMap.add((*pulumi.ArchiveMapMapInput)(nil), (*map[string]map[string]pulumi.Archive)(nil))

	//ArchiveArrayArrayInput to [][]pulumi.Archive
	inputMap.add((*pulumi.ArchiveArrayArrayInput)(nil), (*[][]pulumi.Archive)(nil))

	//AssetInput to pulumi.Asset
	inputMap.add((*pulumi.AssetInput)(nil), (*pulumi.Asset)(nil))

	//AssetArrayInput to []pulumi.Asset
	inputMap.add((*pulumi.AssetArrayInput)(nil), (*[]pulumi.Asset)(nil))

	//AssetMapInput to map[string]pulumi.Asset
	inputMap.add((*pulumi.AssetMapInput)(nil), (*map[string]pulumi.Asset)(nil))

	//AssetArrayMapInput to map[string][]pulumi.Asset
	inputMap.add((*pulumi.AssetArrayMapInput)(nil), (*map[string][]pulumi.Asset)(nil))

	//AssetMapArrayInput to []map[string]pulumi.Asset
	inputMap.add((*pulumi.AssetMapArrayInput)(nil), (*[]map[string]pulumi.Asset)(nil))

	//AssetMapMapInput to map[string]map[string]pulumi.Asset
	inputMap.add((*pulumi.AssetMapMapInput)(nil), (*map[string]map[string]pulumi.Asset)(nil))

	//AssetArrayArrayInput to [][]pulumi.Asset
	inputMap.add((*pulumi.AssetArrayArrayInput)(nil), (*[][]pulumi.Asset)(nil))

	//AssetOrArchiveInput to pulumi.AssetOrArchive
	inputMap.add((*pulumi.AssetOrArchiveInput)(nil), (*pulumi.AssetOrArchive)(nil))

	//AssetOrArchiveArrayInput to []pulumi.AssetOrArchive
	inputMap.add((*pulumi.AssetOrArchiveArrayInput)(nil), (*[]pulumi.AssetOrArchive)(nil))

	//AssetOrArchiveMapInput to map[string]pulumi.AssetOrArchive
	inputMap.add((*pulumi.AssetOrArchiveMapInput)(nil), (*map[string]pulumi.AssetOrArchive)(nil))

	//AssetOrArchiveArrayMapInput to map[string][]pulumi.AssetOrArchive
	inputMap.add((*pulumi.AssetOrArchiveArrayMapInput)(nil), (*map[string][]pulumi.AssetOrArchive)(nil))

	//AssetOrArchiveMapArrayInput to []map[string]pulumi.AssetOrArchive
	inputMap.add((*pulumi.AssetOrArchiveMapArrayInput)(nil), (*[]map[string]pulumi.AssetOrArchive)(nil))

	//AssetOrArchiveMapMapInput to map[string]map[string]pulumi.AssetOrArchive
	inputMap.add((*pulumi.AssetOrArchiveMapMapInput)(nil), (*map[string]map[string]pulumi.AssetOrArchive)(nil))

	//AssetOrArchiveArrayArrayInput to [][]pulumi.AssetOrArchive
	inputMap.add((*pulumi.AssetOrArchiveArrayArrayInput)(nil), (*[][]pulumi.AssetOrArchive)(nil))

	//BoolInput to bool
	inputMap.add((*pulumi.BoolInput)(nil), (*bool)(nil))

	//BoolArrayInput to []bool
	inputMap.add((*pulumi.BoolArrayInput)(nil), (*[]bool)(nil))

	//BoolMapInput to map[string]bool
	inputMap.add((*pulumi.BoolMapInput)(nil), (*map[string]bool)(nil))

	//BoolArrayMapInput to map[string][]bool
	inputMap.add((*pulumi.BoolArrayMapInput)(nil), (*map[string][]bool)(nil))

	//BoolMapArrayInput to []map[string]bool
	inputMap.add((*pulumi.BoolMapArrayInput)(nil), (*[]map[string]bool)(nil))

	//BoolMapMapInput to map[string]map[string]bool
	inputMap.add((*pulumi.BoolMapMapInput)(nil), (*map[string]map[string]bool)(nil))

	//BoolArrayArrayInput to [][]bool
	inputMap.add((*pulumi.BoolArrayArrayInput)(nil), (*[][]bool)(nil))

	//IDInput to pulumi.ID
	inputMap.add((*pulumi.IDInput)(nil), (*pulumi.ID)(nil))

	//IDPtrInput to *pulumi.ID
	inputMap.add((*pulumi.IDPtrInput)(nil), (**pulumi.ID)(nil))

	//IDArrayInput to []pulumi.ID
	inputMap.add((*pulumi.IDArrayInput)(nil), (*[]pulumi.ID)(nil))

	//IDMapInput to map[string]pulumi.ID
	inputMap.add((*pulumi.IDMapInput)(nil), (*map[string]pulumi.ID)(nil))

	//IDArrayMapInput to map[string][]pulumi.ID
	inputMap.add((*pulumi.IDArrayMapInput)(nil), (*map[string][]pulumi.ID)(nil))

	//IDMapArrayInput to []map[string]pulumi.ID
	inputMap.add((*pulumi.IDMapArrayInput)(nil), (*[]map[string]pulumi.ID)(nil))

	//IDMapMapInput to map[string]map[string]pulumi.ID
	inputMap.add((*pulumi.IDMapMapInput)(nil), (*map[string]map[string]pulumi.ID)(nil))

	//IDArrayArrayInput to [][]pulumi.ID
	inputMap.add((*pulumi.IDArrayArrayInput)(nil), (*[][]pulumi.ID)(nil))

	//ArrayInput to []interface{}
	inputMap.add((*pulumi.ArrayInput)(nil), (*[]interface{})(nil))

	//MapInput to map[string]interface{}
	inputMap.add((*pulumi.MapInput)(nil), (*map[string]interface{})(nil))

	//ArrayMapInput to map[string][]interface{}
	inputMap.add((*pulumi.ArrayMapInput)(nil), (*map[string][]interface{})(nil))

	//MapArrayInput to []map[string]interface{}
	inputMap.add((*pulumi.MapArrayInput)(nil), (*[]map[string]interface{})(nil))

	//MapMapInput to map[string]map[string]interface{}
	inputMap.add((*pulumi.MapMapInput)(nil), (*map[string]map[string]interface{})(nil))

	//ArrayArrayInput to [][]interface{}
	inputMap.add((*pulumi.ArrayArrayInput)(nil), (*[][]interface{})(nil))

	//ArrayArrayMapInput to map[string][][]interface{}
	inputMap.add((*pulumi.ArrayArrayMapInput)(nil), (*map[string][][]interface{})(nil))

	//Float65Input to float64
	inputMap.add((*pulumi.Float64Input)(nil), (*float64)(nil))

	//Float64PtrInput to *float64
	inputMap.add((*pulumi.Float64PtrInput)(nil), (**float64)(nil))

	//Float64ArrayInput to []float64
	inputMap.add((*pulumi.Float64ArrayInput)(nil), (*[]float64)(nil))

	//Float64MapInput to map[string]float64
	inputMap.add((*pulumi.Float64MapInput)(nil), (*map[string]float64)(nil))

	//Float64ArrayMapInput to map[string][]float64
	inputMap.add((*pulumi.Float64ArrayMapInput)(nil), (*map[string][]float64)(nil))

	//Float64MapArrayInput to []map[string]float64
	inputMap.add((*pulumi.Float64MapArrayInput)(nil), (*[]map[string]float64)(nil))

	//Float64MapMapInput to map[string]map[string]float64
	inputMap.add((*pulumi.Float64MapMapInput)(nil), (*map[string]map[string]float64)(nil))

	//Float64ArrayArrayInput to [][]float64
	inputMap.add((*pulumi.Float64ArrayArrayInput)(nil), (*[][]float64)(nil))

	//ResourceInput to pulumi.Resource
	inputMap.add((*pulumi.ResourceInput)(nil), (*pulumi.Resource)(nil))

	//ResourceArrayInput to []pulumi.Resource
	inputMap.add((*pulumi.ResourceArrayInput)(nil), (*[]pulumi.Resource)(nil))

	return inputMap
}

type inputToImplementor map[reflect.Type]reflect.Type

func (m inputToImplementor) add(k interface{}, v interface{}) {
	m[reflect.TypeOf(k).Elem()] = reflect.TypeOf(v).Elem()
}

func mergePackageSpec(spec, over schema.PackageSpec) schema.PackageSpec {
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
	spec.Provider = mergeResourceSpec(spec.Provider, over.Provider)
	if over.Resources != nil {
		spec.Resources = mergeMapsOverride(spec.Resources, over.Resources)
	}
	if over.Functions != nil {
		spec.Functions = mergeMapsOverride(spec.Functions, over.Functions)
	}
	return spec
}

//mergeResourceSpec merges two resource specs together.
func mergeResourceSpec(base, over schema.ResourceSpec) schema.ResourceSpec {
	base.ObjectTypeSpec = mergeObjectTypeSpec(base.ObjectTypeSpec, over.ObjectTypeSpec)

	if over.InputProperties != nil {
		base.InputProperties = mergeMapsOverride(base.InputProperties, over.InputProperties)
	}
	if over.RequiredInputs != nil {
		base.RequiredInputs = mergeStructArraysByName(base.RequiredInputs, over.RequiredInputs)
	}
	//PlainInputs is deprecated and thus ignored
	if over.StateInputs != nil {
		//StateInputs is a pointer, so for now we're just going to override it.
		//It could also be dereferenced and merged, but for now we'll keep it like this
		base.StateInputs = over.StateInputs
	}
	if over.Aliases != nil {
		base.Aliases = mergeStructArraysByName(base.Aliases, over.Aliases)
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
	return base
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

// Get the resourceSpec for a single resource
func serializeResource(rawResource interface{}, info serializationInfo) (schema.ResourceSpec, error) {
	v := reflect.ValueOf(rawResource)
	for v.Type().Kind() == reflect.Ptr {
		v = v.Elem()
	}

	t := v.Type()
	properties := make(map[string]schema.PropertySpec)
	inputProperties := make(map[string]schema.PropertySpec)
	requiredInputs := make([]string, 0)

	for i := 0; i < t.NumField(); i++ {
		//A little janky but works for now
		ignoreField := false
		for _, itype := range ignore {
			if t.Field(i).Type.String() == itype {
				ignoreField = true
			}
		}
		if ignoreField {
			continue
		}
		field := t.Field(i)
		fieldType := field.Type
		required := true
		vField := v.Field(i)
		isInput := field.Type.Implements(reflect.TypeOf(new(pulumi.Input)).Elem())
		_, isOutput := vField.Interface().(pulumi.Output)

		for fieldType.Kind() == reflect.Ptr {
			required = false
			fieldType = fieldType.Elem()
		}
		if isOutput {
			fieldType = reflect.New(fieldType).Elem().Interface().(pulumi.Output).ElementType()
			required = false
		} else if isInput {
			if info.inputMap[fieldType] == nil {
				return schema.ResourceSpec{}, errors.New(fmt.Sprintf("Could not find base type for input type %s", fieldType))
			} else {
				fieldType = info.inputMap[fieldType]
			}
		}
		fieldType = dereference(fieldType)
		serialized, err := serializeProperty(fieldType, getFlag(field, "description"), info)
		if err != nil {
			return schema.ResourceSpec{}, err
		}

		if hasBoolFlag(field, "input") || isInput {
			inputProperties[field.Name] = serialized
			if required || hasBoolFlag(field, "required") {
				requiredInputs = append(requiredInputs, field.Name)
			}
		}
		properties[field.Name] = serialized
	}

	spec := schema.ResourceSpec{}
	spec.ObjectTypeSpec.Properties = properties
	spec.InputProperties = inputProperties
	spec.RequiredInputs = requiredInputs
	return spec, nil
}

//Check if a field contains a specified boolean flag
func hasBoolFlag(field reflect.StructField, flag string) bool {
	tag, ok := field.Tag.Lookup(flag)
	return ok && tag == "true"
}

//Get the value of a flag on a field
func getFlag(field reflect.StructField, flag string) string {
	tag, ok := field.Tag.Lookup(flag)
	if ok {
		return tag
	}
	return ""
}

//Get the propertySpec for a single property
func serializeProperty(t reflect.Type, description string, info serializationInfo) (schema.PropertySpec, error) {
	typeName := getTypeName(t)
	if isTypeOrResource(t, info) {
		typeSpec, err := serializeRef(t, info)
		if err != nil {
			return schema.PropertySpec{}, err
		}
		return schema.PropertySpec{
			Description: description,
			TypeSpec:    *typeSpec,
		}, nil
	} else if typeName != UNKNOWN {
		if typeName == ARRAY {
			itemSpec, err := serializeTypeRef(t.Elem(), info)
			if err != nil {
				return schema.PropertySpec{}, err
			}
			return schema.PropertySpec{
				Description: description,
				TypeSpec: schema.TypeSpec{
					Type:  typeName,
					Items: itemSpec,
				},
			}, nil
		}

		return schema.PropertySpec{
			Description: description,
			TypeSpec: schema.TypeSpec{
				Type: typeName,
			},
		}, nil
	} else {
		return schema.PropertySpec{}, errors.New(fmt.Sprintf("Unknown type %s", t))
	}
}

func serializeRef(t reflect.Type, info serializationInfo) (*schema.TypeSpec, error) {
	dereference(t)
	token, isResource := info.resources[t]
	path := "#/resources/"
	if !isResource {
		var isType bool
		token, isType = info.types[t]
		path = "#/types/"
		if !isType {
			return &schema.TypeSpec{}, errors.New(fmt.Sprintf("Unknown type %s", t))
		}
	}

	return &schema.TypeSpec{
		Ref: path + token,
	}, nil
}

func isTypeOrResource(t reflect.Type, info serializationInfo) bool {
	dereference(t)
	_, isResource := info.resources[t]
	if isResource {
		return true
	}
	_, isType := info.types[t]
	return isType
}

//Get the typeSpec for a single type
func getTypeName(t reflect.Type) string {
	var typeName string
	switch t.Kind() {
	case reflect.String:
		typeName = STRING
	case reflect.Bool:
		typeName = BOOL
	case reflect.Int:
		typeName = INT
	case reflect.Float64:
		typeName = FLOAT
	case reflect.Array, reflect.Slice:
		typeName = ARRAY
	case reflect.Map, reflect.Interface, reflect.Struct: //Should maps be objects?
		typeName = OBJECT
	default:
		typeName = UNKNOWN
	}
	return typeName
}

func serializeTypeRef(t reflect.Type, info serializationInfo) (*schema.TypeSpec, error) {
	typeName := getTypeName(t)
	if isTypeOrResource(t, info) {
		return serializeRef(t, info)
	}
	if typeName == ARRAY {
		itemSpec, err := serializeTypeRef(t.Elem(), info)
		if err != nil {
			return nil, err
		}
		return &schema.TypeSpec{
			Type:  typeName,
			Items: itemSpec,
		}, nil
	} else if typeName != UNKNOWN {
		return &schema.TypeSpec{
			Type: typeName,
		}, nil
	} else {
		return &schema.TypeSpec{}, errors.New(fmt.Sprintf("Unknown type %s", t))
	}
}

func serializeType(typ interface{}, info serializationInfo) (schema.ComplexTypeSpec, error) {
	t := reflect.TypeOf(typ)
	t = dereference(t)
	typeName := getTypeName(t)
	var err error
	if typeName == "object" {
		properties := make(map[string]schema.PropertySpec)
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			properties[field.Name], err = serializeProperty(field.Type, getFlag(field, "description"), info)
			if err != nil {
				return schema.ComplexTypeSpec{}, err
			}
		}
		return schema.ComplexTypeSpec{
			ObjectTypeSpec: schema.ObjectTypeSpec{
				Type:       "object",
				Properties: properties,
			},
		}, nil
	}
	enumVals := make([]schema.EnumValueSpec, 0)

	return schema.ComplexTypeSpec{
		ObjectTypeSpec: schema.ObjectTypeSpec{
			Description: "WARNING! Enum types must be manually specified. Overwrite this autogenerated type with your own.",
			Type:        typeName,
		},
		Enum: enumVals,
	}, nil
}

func mergeStringArrays(base, override []string) []string {
	m := make(map[string]bool)
	for _, x := range base {
		m[x] = true
	}
	for _, y := range override {
		m[y] = true
	}
	var merged []string
	for k := range m {
		merged = append(merged, k)
	}
	return merged
}

//Merge two arrays of structs which have the string property "Name" by their names
func mergeStructArraysByName[T any](base, override []T) []T {
	m := make(map[string]T)

	for _, x := range base {
		name := reflect.ValueOf(x).FieldByName("Name").String()
		m[name] = x
	}

	for _, y := range override {
		name := reflect.ValueOf(y).FieldByName("Name").String()
		m[name] = y
	}

	var merged []T
	for _, v := range m {
		merged = append(merged, v)
	}
	return merged
}

func mergeMapsOverride[T any](base, override map[string]T) map[string]T {
	for k, v := range override {
		base[k] = v
	}
	return base
}

/*
func mergeMapsWithMergeFunction[T any](base, override map[string]T, mergeFunc func(T, T) T) map[string]T {
	for k, v := range override {
		base[k] = mergeFunc(base[k], v)
	}
	return base
}*/

func dereference(t reflect.Type) reflect.Type {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t
}
