package provider

import (
	"encoding/json"
	"reflect"

	"github.com/pulumi/pulumi-go-provider/resource"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

// Serialize a package to JSON Schema.
func serialize(opts options) ([]byte, error) {
	pkgSpec := serializeSchema(opts)

	schemaJSON, err := json.Marshal(pkgSpec)
	if err != nil {
		return nil, err
	}
	return schemaJSON, nil
}

// Get the packagespec given resources, etc.
func serializeSchema(opts options) schema.PackageSpec {
	spec := schema.PackageSpec{}
	spec.Resources = make(map[string]schema.ResourceSpec)
	for i := 0; i < len(opts.Resources); i++ {
		resource := opts.Resources[i]
		//where to fetch name?
		token, resourceSpec := serializeResource(opts.Name, "foobar", resource)
		spec.Resources[token] = resourceSpec
	}
	return spec
}

// Get the resourceSpec for a single resource
func serializeResource(pkgname string, resourcename string, resource interface{}) (string, schema.ResourceSpec) {
	t := reflect.TypeOf(resource)
	var properties map[string]schema.PropertySpec = make(map[string]schema.PropertySpec)
	for i := 0; i < t.NumField(); i++ {
		properties[t.Field(i).Name] = serializeProperty(t.Field(i).Type)
	}

	token := pkgname + ":index:" + resourcename
	spec := schema.ResourceSpec{}
	spec.ObjectTypeSpec.Properties = properties
	return token, spec
}

//Get the propertySpec for a single property
func serializeProperty(t reflect.Type) schema.PropertySpec {
	typeName := getTypeName(t)

	if typeName == "unknown" {
		panic("Unsupported non-generic type " + t.Kind().String())
	} else {
		if typeName == "array" {
			return schema.PropertySpec{
				TypeSpec: schema.TypeSpec{
					Type:  typeName,
					Items: serializeType(t.Elem()),
				},
			}
		} else {
			return schema.PropertySpec{
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

func serializeType(t reflect.Type) *schema.TypeSpec {
	typeName := getTypeName(t)
	if typeName == "unknown" {
		panic("Unsupported non-generic type " + t.Kind().String())
	} else {
		if typeName == "array" {
			return &schema.TypeSpec{
				Type:  typeName,
				Items: serializeType(t.Elem()),
			}
		} else {
			return &schema.TypeSpec{
				Type: typeName,
			}
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
