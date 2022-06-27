package main

import (
	"reflect"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

type PropertyDescriptor struct {
	t reflect.Type
}

type Resource interface {
}

type Type interface {
}

type Function interface {
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

	resourceType := reflect.TypeOf((*Resource)(nil)).Elem()
	typeType := reflect.TypeOf((*Type)(nil)).Elem()
	functionType := reflect.TypeOf((*Function)(nil)).Elem()

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
