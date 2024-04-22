// Copyright 2024, Pulumi Corporation.
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

package types

import (
	"fmt"
	"reflect"
	"strings"

	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"

	"github.com/pulumi/pulumi-go-provider/internal/introspect"
	"github.com/pulumi/pulumi-go-provider/middleware/schema"
)

// underlyingType find the non-inputty, non-ptr type of t. It returns the underlying type
// and if t was an Inputty or Outputty type.
func Underlying(t reflect.Type) (reflect.Type, bool, error) {
	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	isInputType := t.Implements(reflect.TypeOf(new(pulumi.Input)).Elem())
	isOutputType := t.Implements(reflect.TypeOf(new(pulumi.Output)).Elem())

	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	if isOutputType {
		t = reflect.New(t).Elem().Interface().(pulumi.Output).ElementType()
	} else if isInputType {
		for t.Kind() == reflect.Pointer {
			t = t.Elem()
		}
		T := strings.TrimSuffix(t.Name(), "Input")
		switch t.Kind() {
		case reflect.Map:
			T = t.Elem().Name() + "Map"
		case reflect.Array, reflect.Slice:
			T = t.Elem().Name() + "Array"
		}

		toOutMethod, ok := t.MethodByName("To" + T + "Output")
		if !ok {
			return nil, false, fmt.Errorf("%v is an input type, but does not have a To%vOutput method", t.Name(), T)
		}
		outputT := toOutMethod.Type.Out(0)
		//create new object of type outputT
		strct := reflect.New(outputT).Elem().Interface()
		out, ok := strct.(pulumi.Output)
		if !ok {
			return nil, false, fmt.Errorf("return type %s of method To%vOutput on type %v does not implement Output",
				reflect.TypeOf(strct), T, t.Name())
		}
		t = out.ElementType()
	}

	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t, isOutputType || isInputType, nil
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

func GetTokenOf(t reflect.Type, transform func(tokens.Type) tokens.Type) (tokens.Type, error) {
	annotator := GetAnnotated(t)
	if annotator.Token != "" {
		return tokens.Type(annotator.Token), nil
	}

	tk, err := introspect.GetToken("pkg", t)
	if transform == nil || err != nil {
		return tk, err
	}

	return transform(tk), nil
}

func ObjectSchema(t reflect.Type) (*pschema.ObjectTypeSpec, error) {
	descriptions := GetAnnotated(t)
	props, required, err := propertyListFromType(t, false)
	if err != nil {
		return nil, fmt.Errorf("could not serialize input type %s: %w", t, err)
	}
	for n, p := range props {
		props[n] = p
	}
	return &pschema.ObjectTypeSpec{
		Description: descriptions.Descriptions[""],
		Properties:  props,
		Required:    required,
		Type:        "object",
	}, nil
}

func ResourceReferenceToken(
	t reflect.Type, extTag *introspect.ExplicitType, allowMissingExtType bool,
) (pschema.TypeSpec, bool, error) {
	ptrT := reflect.PointerTo(t)
	implements := func(typ reflect.Type) bool {
		return t.Implements(typ) || ptrT.Implements(typ)
	}
	switch {
	// This handles both components and resources
	case implements(reflect.TypeOf(new(schema.Resource)).Elem()):
		tk, err := reflect.New(t).Elem().Interface().(schema.Resource).GetToken()
		return pschema.TypeSpec{
			Ref: "#/resources/" + tk.String(),
		}, true, err
	case implements(reflect.TypeOf(new(pulumi.Resource)).Elem()):
		// This is an external resource
		if extTag == nil {
			if allowMissingExtType {
				return pschema.TypeSpec{}, true, nil
			}
			return pschema.TypeSpec{}, true, fmt.Errorf("missing type= tag on foreign resource %s", t)
		}
		tk := fmt.Sprintf("%s:%s:%s", extTag.Pkg, extTag.Module, extTag.Name)
		return pschema.TypeSpec{
			Ref: fmt.Sprintf("/%s/%s/schema.json#/resources/%s", extTag.Pkg, extTag.Version, tk),
		}, true, nil
	default:
		return pschema.TypeSpec{}, false, nil
	}
}

func serializeTypeAsPropertyType(
	t reflect.Type, indicatePlain bool, extType *introspect.ExplicitType,
) (pschema.TypeSpec, error) {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t == reflect.TypeOf(resource.Asset{}) {
		return pschema.TypeSpec{
			Ref: "pulumi.json#/Asset",
		}, nil
	}
	if t == reflect.TypeOf(resource.Archive{}) {
		return pschema.TypeSpec{
			Ref: "pulumi.json#/Archive",
		}, nil
	}
	if enum, ok := isEnum(t); ok {
		return pschema.TypeSpec{
			Ref: "#/types/" + enum.token,
		}, nil
	}
	t, inputy, err := Underlying(t)
	if err != nil {
		return pschema.TypeSpec{}, err
	}
	if tk, ok, err := ResourceReferenceToken(t, extType, false); ok {
		if err != nil {
			return pschema.TypeSpec{}, err
		}
		return tk, nil
	}
	if tk, ok, err := structReferenceToken(t, extType); ok {
		if err != nil {
			return pschema.TypeSpec{}, err
		}
		return tk, nil
	}

	// Must be a primitive type
	primitive := func(t string) (pschema.TypeSpec, error) {
		return pschema.TypeSpec{Type: t, Plain: !inputy && indicatePlain}, nil
	}

	switch t.Kind() {
	case reflect.Map:
		if t.Key().Kind() != reflect.String {
			return pschema.TypeSpec{}, fmt.Errorf("map keys must be strings, found %s", t.Key().String())
		}
		el, err := serializeTypeAsPropertyType(t.Elem(), indicatePlain, extType)
		if err != nil {
			return pschema.TypeSpec{}, err
		}
		return pschema.TypeSpec{
			Type:                 "object",
			AdditionalProperties: &el,
		}, nil
	case reflect.Array, reflect.Slice:
		el, err := serializeTypeAsPropertyType(t.Elem(), indicatePlain, extType)
		if err != nil {
			return pschema.TypeSpec{}, err
		}
		return pschema.TypeSpec{
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
		return pschema.TypeSpec{
			Ref: "pulumi.json#/Any",
		}, nil
	default:
		return pschema.TypeSpec{}, fmt.Errorf("unknown type: '%s'", t.String())
	}
}

func propertyListFromType(typ reflect.Type, indicatePlain bool) (
	props map[string]pschema.PropertySpec, required []string, err error) {
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	props = map[string]pschema.PropertySpec{}
	annotations := GetAnnotated(typ)

	for _, field := range reflect.VisibleFields(typ) {
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
		serialized, err := serializeTypeAsPropertyType(fieldType, indicatePlain, tags.ExplicitRef)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid type '%s' on '%s.%s': %w", fieldType, typ, field.Name, err)
		}
		if !tags.Optional {
			required = append(required, tags.Name)
		}
		spec := &pschema.PropertySpec{
			TypeSpec:         serialized,
			Secret:           tags.Secret,
			ReplaceOnChanges: tags.ReplaceOnChanges,
			Description:      annotations.Descriptions[tags.Name],
			Default:          annotations.Defaults[tags.Name],
		}
		if envs := annotations.DefaultEnvs[tags.Name]; len(envs) > 0 {
			spec.DefaultInfo = &pschema.DefaultSpec{
				Environment: envs,
			}
		}
		props[tags.Name] = *spec
	}
	return props, required, nil
}

func structReferenceToken(t reflect.Type, extTag *introspect.ExplicitType) (pschema.TypeSpec, bool, error) {
	if t.Kind() == reflect.Struct && extTag != nil {
		if extTag.Pkg != "" {
			return pschema.TypeSpec{
				Ref: fmt.Sprintf("/%s/%s/schema.json#/types/%s:%s:%s",
					extTag.Pkg, extTag.Version,
					extTag.Pkg, extTag.Module, extTag.Name,
				),
			}, true, nil
		}
		return pschema.TypeSpec{
			Ref: fmt.Sprintf("#/types/pkg:%s:%s", extTag.Module, extTag.Name),
		}, true, nil
	}
	if t.Kind() != reflect.Struct ||
		t.Implements(reflect.TypeOf(new(pulumi.Output)).Elem()) {
		return pschema.TypeSpec{}, false, nil
	}

	tk, err := GetTokenOf(t, nil)
	if err != nil {
		return pschema.TypeSpec{}, true, err
	}

	return pschema.TypeSpec{
		Ref: "#/types/" + tk.String(),
	}, true, nil
}
