package provider

import (
	"encoding/json"
	"reflect"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/go/pulumi"
)

var ignore []string = []string{"resource.Custom", "pulumi.ResourceState", "ResourceState"}

// Serialize a package to JSON Schema.
func serialize(opts options) (string, error) {
	pkgSpec := serializeSchema(opts)

	schemaJSON, err := json.Marshal(pkgSpec)
	if err != nil {
		return "", err
	}
	return string(schemaJSON), nil
}

// Get the packagespec given resources, etc.
func serializeSchema(opts options) schema.PackageSpec {
	spec := schema.PackageSpec{}
	spec.Resources = make(map[string]schema.ResourceSpec)
	spec.Types = make(map[string]schema.ComplexTypeSpec)
	spec.Name = opts.Name
	spec.Version = opts.Version.String()

	for i := 0; i < len(opts.Resources); i++ {
		resource := opts.Resources[i]

		name := reflect.TypeOf(resource).String()
		name = strings.Split(name, ".")[1]

		token, resourceSpec := serializeResource(opts.Name, name, resource)
		spec.Resources[token] = resourceSpec
	}
	//Components are essentially resources, I don't believe they are differentiated in the schema
	for i := 0; i < len(opts.Components); i++ {
		component := opts.Components[i]

		name := reflect.TypeOf(component).String()
		name = strings.Split(name, ".")[1]

		token, componentSpec := serializeResource(opts.Name, name, component)
		spec.Resources[token] = componentSpec
	}

	for i := 0; i < len(opts.Types); i++ {
		t := opts.Types[i]

		name := reflect.TypeOf(t).String()
		name = strings.Split(name, ".")[1]

		token, typeSpec := serializeType(opts.Name, name, t)
		spec.Types[token] = typeSpec
	}
	over := opts.PartialSpec
	return mergePackageSpec(spec, over)
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
func serializeResource(pkgname string, resourcename string, resource interface{}) (string, schema.ResourceSpec) {

	for reflect.TypeOf(resource).Kind() == reflect.Ptr {
		resource = reflect.ValueOf(resource).Elem().Interface()
	}

	t := reflect.TypeOf(resource)
	var properties map[string]schema.PropertySpec = make(map[string]schema.PropertySpec)
	var inputProperties map[string]schema.PropertySpec = make(map[string]schema.PropertySpec)
	var requiredInputs []string = make([]string, 0)

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
		//isOutput, _ := regexp.MatchString("pulumi.[a-zA-Z0-9]+Output", fieldType.String())
		//isInput, _ := regexp.MatchString("pulumi.[a-zA-Z0-9]+Input", fieldType.String())
		isInput := fieldType.Implements(reflect.TypeOf((*pulumi.Input)(nil)).Elem())
		isOutput := fieldType.Implements(reflect.TypeOf((*pulumi.Output)(nil)).Elem())

		for fieldType.Kind() == reflect.Ptr {
			required = false
			fieldType = fieldType.Elem()
		}
		var serialized schema.PropertySpec
		if isOutput {
			fieldType = reflect.ValueOf(field).Interface().(pulumi.Output).ElementType()
		} else if isInput {
			fieldType = reflect.ValueOf(field).Interface().(pulumi.Input).ElementType()
		}
		serialized = serializeProperty(fieldType, getFlag(field, "description"))

		if hasBoolFlag(field, "input") || isInput {
			inputProperties[field.Name] = serialized
			if required || hasBoolFlag(field, "required") {
				requiredInputs = append(requiredInputs, field.Name)
			}
		}
		properties[field.Name] = serialized
	}

	//TODO: Allow submodule name to be specified
	token := pkgname + ":index:" + resourcename
	spec := schema.ResourceSpec{}
	spec.ObjectTypeSpec.Properties = properties
	spec.InputProperties = inputProperties
	spec.RequiredInputs = requiredInputs
	return token, spec
}

func listAllFields(t reflect.Type) {
	for i := 0; i < t.NumField(); i++ {
		print(t.Field(i).Name, ",", t.Field(i).Type.String(), "\n")
	}
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
	} else {
		return ""
	}
}

//Get the propertySpec for a single property
func serializeProperty(t reflect.Type, description string) schema.PropertySpec {
	typeName := getTypeName(t)
	if typeName == "unknown" {
		panic("Unsupported non-generic type " + t.String())
	} else {
		if typeName == "array" {
			return schema.PropertySpec{
				Description: description,
				TypeSpec: schema.TypeSpec{
					Type:  typeName,
					Items: serializeTypeRef(t.Elem()),
				},
			}
		} else {
			return schema.PropertySpec{
				Description: description,
				TypeSpec: schema.TypeSpec{
					Type: typeName,
				},
			}
		}
	}
}

//Get the typeSpec for a single type
func getTypeName(t reflect.Type) string {
	var typeName string
	switch t.Kind() {
	case reflect.String:
		typeName = "string"
	case reflect.Bool:
		typeName = "boolean"
	case reflect.Int:
		typeName = "integer"
	case reflect.Float64:
		typeName = "number"
	case reflect.Array, reflect.Slice:
		typeName = "array"
	case reflect.Map:
		typeName = "object"
	default:
		typeName = "unknown"
	}
	return typeName
}

func serializeTypeRef(t reflect.Type) *schema.TypeSpec {
	typeName := getTypeName(t)
	if typeName == "unknown" {
		panic("Unsupported non-generic type " + t.Kind().String())
	} else {
		if typeName == "array" {
			return &schema.TypeSpec{
				Type:  typeName,
				Items: serializeTypeRef(t.Elem()),
			}
		} else {
			return &schema.TypeSpec{
				Type: typeName,
			}
		}
	}
}

func serializeType(pkgname string, resourcename string, typ interface{}) (string, schema.ComplexTypeSpec) {
	t := reflect.TypeOf(typ)
	typeName := getTypeName(t)

	token := pkgname + ":index:" + resourcename

	if typeName == "object" {
		properties := make(map[string]schema.PropertySpec)
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			properties[field.Name] = serializeProperty(field.Type, getFlag(field, "description"))
		}
		return token, schema.ComplexTypeSpec{
			ObjectTypeSpec: schema.ObjectTypeSpec{
				Type:       "object",
				Properties: properties,
			},
		}
	} else {
		//Type is an enum
		enumVals := make([]schema.EnumValueSpec, 0)
		//Enum values MUST be manually specified

		return token, schema.ComplexTypeSpec{
			ObjectTypeSpec: schema.ObjectTypeSpec{
				Type: typeName,
			},
			Enum: enumVals,
		}
	}
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

func mergeMapsWithMergeFunction[T any](base, override map[string]T, mergeFunc func(T, T) T) map[string]T {
	for k, v := range override {
		base[k] = mergeFunc(base[k], v)
	}
	return base
}

/*
func getReference(t reflect.Type, name string, packagename string) (string, error) {
	isResource := false
	isType := false
	isFunction := false

	resourceType := reflect.TypeOf((*resource.Custom)(nil))

	for i := 0; i < t.NumField(); i++ {
		if t.Field(i).Name == "Resource" && t.Field(i).Type == resourceType {
			isResource = true
			break
		}
	}
	typename := ""
	if isResource {
		typename = "resource"
	}
	if isType {
		if typename != "" {
			panic("Must be only one of Resource or Type or Function")
		}
		typename = "type"
	}
	if isFunction {
		if typename != "" {
			panic("Must be only one of Resource or Type or Function")
		}
		typename = "function"
	}

	if typename == "" {
		panic("Must embed Type, Resource or Function")
	}

	typetoken := packagename + ":index:" + name

	return "#/" + typename + "/" + typetoken, nil
}
*/
