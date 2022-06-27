package provider

import (
	"encoding/json"
	"reflect"
	"strings"

	"github.com/pulumi/pulumi-go-provider/resource"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

type PropertyArgs struct {
	Description string
	Required    bool
	Input       bool
}

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

	for i := 0; i < len(opts.Resources); i++ {
		resource := opts.Resources[i]

		//Fetch the package path
		mod := reflect.TypeOf(resource).PkgPath()
		if mod == "" {
			panic("Type " + reflect.TypeOf(resource).String() + "has no module path")
		}
		// Take off the pkg name, since that is supplied by `pkg`.
		mod = mod[strings.IndexRune(mod, '/')+1:]

		token, resourceSpec := serializeResource(opts.Name, mod, resource)
		spec.Resources[token] = resourceSpec
	}
	//Components are essentially resources, I don't believe they are differentiated in the schema
	for i := 0; i < len(opts.Components); i++ {
		component := opts.Components[i]

		//Fetch the package path
		mod := reflect.TypeOf(component).PkgPath()
		if mod == "" {
			panic("Type " + reflect.TypeOf(component).String() + "has no module path")
		}
		// Take off the pkg name, since that is supplied by `pkg`.
		mod = mod[strings.IndexRune(mod, '/')+1:]

		token, componentSpec := serializeResource(opts.Name, mod, component)
		spec.Resources[token] = componentSpec
	}

	for i := 0; i < len(opts.Types); i++ {
		t := opts.Types[i]
		//Fetch the package path
		mod := reflect.TypeOf(t).PkgPath()
		if mod == "" {
			panic("Type " + reflect.TypeOf(t).String() + "has no module path")
		}
		// Take off the pkg name, since that is supplied by `pkg`.
		mod = mod[strings.IndexRune(mod, '/')+1:]

		token, typeSpec := serializeType(opts.Name, mod, t)
		spec.Types[token] = typeSpec
	}
	return spec
}

// Get the resourceSpec for a single resource
func serializeResource(pkgname string, resourcename string, resource interface{}) (string, schema.ResourceSpec) {
	t := reflect.TypeOf(resource)
	var properties map[string]schema.PropertySpec = make(map[string]schema.PropertySpec)
	var inputProperties map[string]schema.PropertySpec = make(map[string]schema.PropertySpec)
	var requiredInputs []string = make([]string, 0)

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if hasBoolFlag(field, "input") {
			inputProperties[field.Name] = serializeProperty(field.Type, getFlag(field, "description"))
			if hasBoolFlag((field), "required") {
				requiredInputs = append(requiredInputs, field.Name)
			}
		}
		properties[field.Name] = serializeProperty(field.Type, getFlag(field, "description"))
	}

	//TODO: Allow submodule name to be specified
	token := pkgname + ":index:" + resourcename
	spec := schema.ResourceSpec{}
	spec.ObjectTypeSpec.Properties = properties
	spec.InputProperties = inputProperties
	spec.RequiredInputs = requiredInputs
	return token, spec
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
		//Copy enum values into schema???

		return token, schema.ComplexTypeSpec{
			ObjectTypeSpec: schema.ObjectTypeSpec{
				Type: typeName,
			},
			Enum: enumVals,
		}
	}
}

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
