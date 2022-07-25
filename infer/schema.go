package infer

import (
	"fmt"
	"reflect"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"

	"github.com/iwahbe/pulumi-go-provider/internal/introspect"
	sch "github.com/iwahbe/pulumi-go-provider/middleware/schema"
)

func serializeTypeAsPropertyType(t reflect.Type) (schema.TypeSpec, error) {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if isEnum(t) {
		enum, err := introspect.GetToken("pkg", reflect.New(t).Elem().Interface())
		if err != nil {
			return schema.TypeSpec{}, err
		}
		return schema.TypeSpec{
			Ref: "#/types/" + enum.String(),
		}, nil
	}
	if tk, ok, err := resourceReferenceToken(t); ok {
		if err != nil {
			return schema.TypeSpec{}, err
		}
		return schema.TypeSpec{
			Ref: "#/resources/" + tk.String(),
		}, nil
	}
	if tk, ok, err := structReferenceToken(t); ok {
		if err != nil {
			return schema.TypeSpec{}, err
		}
		return schema.TypeSpec{
			Ref: "#/types/" + tk.String(),
		}, nil
	}

	primitive := func(t string) (schema.TypeSpec, error) {
		return schema.TypeSpec{Type: t}, nil
	}

	// Must be a primitive type
	switch t.Kind() {
	case reflect.Map:
		if t.Key().Kind() != reflect.String {
			return schema.TypeSpec{}, fmt.Errorf("map keys must be strings, found %s", t.Key().String())
		}
		el, err := serializeTypeAsPropertyType(t.Elem())
		if err != nil {
			return schema.TypeSpec{}, err
		}
		return schema.TypeSpec{
			Type:                 "object",
			AdditionalProperties: &el,
		}, nil
	case reflect.Array, reflect.Slice:
		el, err := serializeTypeAsPropertyType(t.Elem())
		if err != nil {
			return schema.TypeSpec{}, err
		}
		return schema.TypeSpec{
			Type:                 "array",
			AdditionalProperties: &el,
		}, nil
	case reflect.Interface:
		return schema.TypeSpec{
			Ref: "pulumi.json#/Any",
		}, nil
	case reflect.Bool:
		return primitive("boolean")
	case reflect.Int, reflect.Int64, reflect.Int32:
		return primitive("integer")
	case reflect.Float64:
		return primitive("number")
	default:
		return schema.TypeSpec{}, fmt.Errorf("unknown type: '%s'", t.String())
	}
}

func propertyListFromType[T any]() (props map[string]schema.PropertySpec, required []string, err error) {
	typ := reflect.TypeOf(new(T)).Elem()
	props = map[string]schema.PropertySpec{}
	descriptions := map[string]string{}
	if t, ok := (interface{})(*new(T)).(Annotated); ok {
		a := introspect.NewAnnotator(t)
		t.Annotate(&a)
		descriptions = a.Descriptions
	}
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		fieldType := field.Type
		for fieldType.Kind() == reflect.Pointer {
			fieldType = fieldType.Elem()
		}
		tags, err := introspect.ParseTag(field)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid fields '%'s on '%T': %w", field.Name, *new(T), err)
		}
		if tags.Internal {
			continue
		}
		serialized, err := serializeTypeAsPropertyType(fieldType)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid type '%s' on '%T.%s': %w", fieldType, *new(T), field.Name, err)
		}
		if !tags.Optional {
			required = append(required, field.Name)
		}
		props[field.Name] = schema.PropertySpec{
			TypeSpec:         serialized,
			Secret:           tags.Secret,
			ReplaceOnChanges: tags.ReplaceOnChanges,
			Description:      descriptions[tags.Name],
		}
	}
	return props, required, nil
}

func isEnum(t reflect.Type) bool {
	var isEnum bool
	switch t.Kind() {
	case reflect.String:
		isEnum = t.String() != reflect.String.String()
	case reflect.Bool:
	case reflect.Int:
		isEnum = t.String() != reflect.Int.String()
	case reflect.Float64:
		isEnum = t.String() != reflect.Float64.String()
	}
	return isEnum
}

func resourceReferenceToken(t reflect.Type) (tokens.Type, bool, error) {
	resType := reflect.TypeOf(new(sch.Resource)).Elem()
	if t.Implements(resType) {
		tk, err := reflect.New(t).Elem().Interface().(sch.Resource).GetToken()
		return tk, true, err
	}
	return "", false, nil
}

func structReferenceToken(t reflect.Type) (tokens.Type, bool, error) {
	if t.Kind() != reflect.Struct {
		return "", false, nil
	}
	tk, err := introspect.GetToken("pkg", reflect.New(t).Elem().Interface())
	return tk, true, err
}
