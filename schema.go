package provider

import (
	"encoding/json"
	"reflect"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

type PropertyDescriptor struct {
	t reflect.Type
}

func serialize(opts options) ([]byte, error) {
	var pkgSpec schema.PackageSpec = serializeSchema(opts)

	schemaJSON, err := json.Marshal(pkgSpec)
	if err != nil {
		return nil, err
	}
	return schemaJSON, nil
}

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

func serializeResource(pkgname string, resourcename string, resource interface{}) (string, schema.ResourceSpec) {
	t := reflect.TypeOf(resource)
	var properties map[string]schema.PropertySpec = make(map[string]schema.PropertySpec)
	for i := 0; i < t.NumField(); i++ {
		properties[t.Field(i).Name] = serializeProperty(PropertyDescriptor{t: t.Field(i).Type})
	}

	token := pkgname + ":index:" + resourcename
	spec := schema.ResourceSpec{}
	spec.ObjectTypeSpec.Properties = properties
	return token, spec
}

func serializeProperty(spec PropertyDescriptor) schema.PropertySpec {
	var typeName string
	switch spec.t.Kind() {
	case reflect.String:
		typeName = "string"
	case reflect.Bool:
		typeName = "boolean"
	case reflect.Int:
		typeName = "integer"
	case reflect.Float64:
		typeName = "number"
	case reflect.Slice:
		typeName = "array"
	case reflect.Map:
		typeName = "object"
	default:
		typeName = "unknown"
	}

	if typeName == "unknown" {
		reference := getReference(spec.t, spec.t.Name(), "pkgname")
		return schema.PropertySpec{
			TypeSpec: schema.TypeSpec{
				Ref: reference,
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

func getReference(t reflect.Type, name string, packagename string) string {
	isResource := false
	isType := false
	isFunction := false

	resourceType := reflect.TypeOf((*Resource)(nil))
	typeType := reflect.TypeOf((*Type)(nil))
	functionType := reflect.TypeOf((*Function)(nil))

	for i := 0; i < t.NumField(); i++ {
		if t.Field(i).Name == "Resource" && t.Field(i).Type == resourceType {
			isResource = true
			break
		}
		if t.Field(i).Name == "Type" && t.Field(i).Type == typeType {
			isType = true
			break
		}
		if t.Field(i).Name == "Function" && t.Field(i).Type == functionType {
			isFunction = true
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

	return "#/" + typename + "/" + typetoken
}
