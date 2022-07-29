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

package infer

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"

	"github.com/pulumi/pulumi-go-provider/internal/introspect"
	sch "github.com/pulumi/pulumi-go-provider/middleware/schema"
)

func getAnnotated(i any) map[string]string {
	// If we have type *R with value(i) = nil, NewAnnotator will fail. We need to get
	// value(i) = *R{}, so we reinflate the underlying value
	if v := reflect.ValueOf(i); v.Kind() == reflect.Pointer && v.IsNil() {
		i = reflect.New(v.Type().Elem()).Interface()
	}

	if r, ok := i.(Annotated); ok {
		a := introspect.NewAnnotator(r)
		r.Annotate(&a)
		return a.Descriptions
	}
	return map[string]string{}
}

func getResourceSchema[R, I, O any]() (schema.ResourceSpec, error) {
	var r R
	descriptions := getAnnotated(r)

	properties, required, err := propertyListFromType(reflect.TypeOf(new(O)))
	if err != nil {
		var o O
		return schema.ResourceSpec{}, fmt.Errorf("could not serialize output type %T: %w", o, err)
	}

	inputProperties, requiredInputs, err := propertyListFromType(reflect.TypeOf(new(I)))
	if err != nil {
		var i I
		return schema.ResourceSpec{}, fmt.Errorf("could not serialize input type %T: %w", i, err)
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

func serializeTypeAsPropertyType(t reflect.Type) (schema.TypeSpec, error) {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if enum, ok := isEnum(t); ok {
		return schema.TypeSpec{
			Ref: "#/types/" + enum.token,
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
	t, err := underlyingType(t)
	if err != nil {
		return schema.TypeSpec{}, err
	}
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
			Type:  "array",
			Items: &el,
		}, nil
	case reflect.Bool:
		return primitive("boolean")
	case reflect.Int, reflect.Int64, reflect.Int32:
		return primitive("integer")
	case reflect.Float64:
		return primitive("number")
	case reflect.String:
		return primitive("string")
	case reflect.Interface:
		return schema.TypeSpec{
			Ref: "pulumi.json#/Any",
		}, nil
	default:
		return schema.TypeSpec{}, fmt.Errorf("unknown type: '%s'", t.String())
	}
}

func underlyingType(t reflect.Type) (reflect.Type, error) {
	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	isInputType := t.Implements(reflect.TypeOf(new(pulumi.Input)).Elem())
	_, isOutputType := reflect.New(t).Interface().(pulumi.Output)

	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	if isOutputType {
		t = reflect.New(t).Elem().Interface().(pulumi.Output).ElementType()
	} else if isInputType {
		T := t.Name()
		if strings.HasSuffix(T, "Input") {
			T = strings.TrimSuffix(T, "Input")
		} else {
			return nil, fmt.Errorf("%v is an input type, but does not end in \"Input\"", T)
		}
		toOutMethod, ok := t.MethodByName("To" + T + "Output")
		if !ok {
			return nil, fmt.Errorf("%v is an input type, but does not have a To%vOutput method", t.Name(), T)
		}
		outputT := toOutMethod.Type.Out(0)
		//create new object of type outputT
		strct := reflect.New(outputT).Elem().Interface()
		out, ok := strct.(pulumi.Output)
		if !ok {
			return nil, fmt.Errorf("return type %s of method To%vOutput on type %v does not implement Output",
				reflect.TypeOf(strct), T, t.Name())
		}
		t = out.ElementType()
	}

	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t, nil
}

func propertyListFromType(typ reflect.Type) (props map[string]schema.PropertySpec, required []string, err error) {
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	props = map[string]schema.PropertySpec{}
	descriptions := getAnnotated(reflect.New(typ))

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		fieldType := field.Type
		for fieldType.Kind() == reflect.Pointer {
			fieldType = fieldType.Elem()
		}
		tags, err := introspect.ParseTag(field)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid fields '%s' on '%s': %w", field.Name, typ, err)
		}
		if tags.Internal {
			continue
		}
		serialized, err := serializeTypeAsPropertyType(fieldType)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid type '%s' on '%s.%s': %w", fieldType, typ, field.Name, err)
		}
		if !tags.Optional {
			required = append(required, tags.Name)
		}
		props[tags.Name] = schema.PropertySpec{
			TypeSpec:         serialized,
			Secret:           tags.Secret,
			ReplaceOnChanges: tags.ReplaceOnChanges,
			Description:      descriptions[tags.Name],
		}
	}
	return props, required, nil
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
	if t.Kind() != reflect.Struct ||
		t.Implements(reflect.TypeOf(new(pulumi.Output)).Elem()) {
		return "", false, nil
	}
	tk, err := introspect.GetToken("pkg", reflect.New(t).Elem().Interface())
	return tk, true, err
}

func schemaNameForType(t reflect.Kind) string {
	switch t {
	case reflect.String:
		return "string"
	case reflect.Bool:
		return "boolean"
	case reflect.Float64:
		return "number"
	case reflect.Int:
		return "integer"
	default:
		panic(fmt.Sprintf("unknown primitive type: %s", t))
	}
}
