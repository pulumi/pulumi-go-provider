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

	"github.com/hashicorp/go-multierror"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"

	"github.com/pulumi/pulumi-go-provider/infer/types"
	"github.com/pulumi/pulumi-go-provider/internal/introspect"
	sch "github.com/pulumi/pulumi-go-provider/middleware/schema"
)

func getAnnotated(t reflect.Type) introspect.Annotator {
	// If we have type *R with value(i) = nil, NewAnnotator will fail. We need to get
	// value(i) = *R{}, so we reinflate the underlying value
	for t.Kind() == reflect.Pointer && t.Elem().Kind() == reflect.Pointer {
		t = t.Elem()
	}
	i := reflect.New(t).Elem()
	if i.Kind() == reflect.Pointer && i.IsNil() {
		i = reflect.New(i.Type().Elem())
	}

	if i.Kind() != reflect.Pointer {
		v := reflect.New(i.Type())
		v.Elem().Set(i)
		i = v
	}
	t = i.Type()

	merge := func(dst *introspect.Annotator, src introspect.Annotator) {
		for k, v := range src.Descriptions {
			(*dst).Descriptions[k] = v
		}
		for k, v := range src.Defaults {
			(*dst).Defaults[k] = v
		}
		for k, v := range src.DefaultEnvs {
			(*dst).DefaultEnvs[k] = v
		}
		dst.Token = src.Token
		dst.Aliases = append(dst.Aliases, src.Aliases...)
		dst.DeprecationMessage = src.DeprecationMessage
	}

	ret := introspect.Annotator{
		Descriptions: map[string]string{},
		Defaults:     map[string]any{},
		DefaultEnvs:  map[string][]string{},
	}
	if t.Elem().Kind() == reflect.Struct {
		for _, f := range reflect.VisibleFields(t.Elem()) {
			if f.Anonymous && f.IsExported() {
				r := getAnnotated(f.Type)
				merge(&ret, r)
			}
		}
	}

	if r, ok := i.Interface().(Annotated); ok {
		a := introspect.NewAnnotator(r)
		r.Annotate(&a)
		merge(&ret, a)
	}

	return ret
}

func getResourceSchema[R, I, O any](isComponent bool) (schema.ResourceSpec, multierror.Error) {
	var r R
	var errs multierror.Error
	annotations := getAnnotated(reflect.TypeOf(r))

	properties, required, err := propertyListFromType(reflect.TypeOf(new(O)), isComponent, outputType)
	if err != nil {
		var o O
		errs.Errors = append(errs.Errors, fmt.Errorf("could not serialize output type %T: %w", o, err))
	}

	inputProperties, requiredInputs, err := propertyListFromType(reflect.TypeOf(new(I)), isComponent, inputType)
	if err != nil {
		var i I
		errs.Errors = append(errs.Errors, fmt.Errorf("could not serialize input type %T: %w", i, err))
	}

	var aliases []schema.AliasSpec
	for _, alias := range annotations.Aliases {
		a := alias
		aliases = append(aliases, schema.AliasSpec{Type: a})
	}

	return schema.ResourceSpec{
		ObjectTypeSpec: schema.ObjectTypeSpec{
			Properties:  properties,
			Description: annotations.Descriptions[""],
			Required:    required,
		},
		InputProperties:    inputProperties,
		RequiredInputs:     requiredInputs,
		IsComponent:        isComponent,
		Aliases:            aliases,
		DeprecationMessage: annotations.DeprecationMessage,
	}, errs
}

func serializeTypeAsPropertyType(
	t reflect.Type, indicatePlain bool, extType *introspect.ExplicitType, propType propertyType,
) (schema.TypeSpec, error) {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	// Provider authors should not be using resource.Asset directly, but rather types.AssetOrArchive.
	// We will returrn an error if resource.Asset or resource.Archive is used directly for an input.
	// pulumi/pulumi-go-provider#243
	if propType == inputType && t == reflect.TypeOf(resource.Asset{}) {
		return schema.TypeSpec{},
			fmt.Errorf("resource.Asset is not a valid input type, please use types.AssetOrArchive instead")
	}
	if propType == inputType && t == reflect.TypeOf(resource.Archive{}) {
		return schema.TypeSpec{},
			fmt.Errorf("resource.Archive is not a valid input type, please use types.AssetOrArchive instead")
	}

	if t == reflect.TypeOf(resource.Asset{}) {
		// We allow this for output types, but not inputs.
		return schema.TypeSpec{
			Ref: "pulumi.json#/Asset",
		}, nil
	}
	if t == reflect.TypeOf(resource.Archive{}) {
		return schema.TypeSpec{
			Ref: "pulumi.json#/Archive",
		}, nil
	}
	if t == reflect.TypeOf(types.AssetOrArchive{}) {
		return schema.TypeSpec{
			Ref: "pulumi.json#/Asset",
		}, nil
	}
	if enum, ok := isEnum(t); ok {
		return schema.TypeSpec{
			Ref: "#/types/" + enum.token,
		}, nil
	}
	t, inputy, err := underlyingType(t)
	if err != nil {
		return schema.TypeSpec{}, err
	}
	if tk, ok, err := resourceReferenceToken(t, extType, false); ok {
		if err != nil {
			return schema.TypeSpec{}, err
		}
		return tk, nil
	}
	if tk, ok, err := structReferenceToken(t, extType); ok {
		if err != nil {
			return schema.TypeSpec{}, err
		}
		return tk, nil
	}

	// Must be a primitive type
	primitive := func(t string) (schema.TypeSpec, error) {
		return schema.TypeSpec{Type: t, Plain: !inputy && indicatePlain}, nil
	}

	switch t.Kind() {
	case reflect.Map:
		if t.Key().Kind() != reflect.String {
			return schema.TypeSpec{}, fmt.Errorf("map keys must be strings, found %s", t.Key().String())
		}
		el, err := serializeTypeAsPropertyType(t.Elem(), indicatePlain, extType, propType)
		if err != nil {
			return schema.TypeSpec{}, err
		}
		return schema.TypeSpec{
			Type:                 "object",
			AdditionalProperties: &el,
		}, nil
	case reflect.Array, reflect.Slice:
		el, err := serializeTypeAsPropertyType(t.Elem(), indicatePlain, extType, propType)
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

// underlyingType find the non-inputty, non-ptr type of t. It returns the underlying type
// and if t was an Inputty or Outputty type.
func underlyingType(t reflect.Type) (reflect.Type, bool, error) {
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

func propertyListFromType(typ reflect.Type, indicatePlain bool, propType propertyType) (
	props map[string]schema.PropertySpec, required []string, err error) {
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	props = map[string]schema.PropertySpec{}
	annotations := getAnnotated(typ)

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
		serialized, err := serializeTypeAsPropertyType(fieldType, indicatePlain, tags.ExplicitRef, propType)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid type '%s' on '%s.%s': %w", fieldType, typ, field.Name, err)
		}
		if !tags.Optional {
			required = append(required, tags.Name)
		}
		spec := &schema.PropertySpec{
			TypeSpec:         serialized,
			Secret:           tags.Secret,
			ReplaceOnChanges: tags.ReplaceOnChanges,
			Description:      annotations.Descriptions[tags.Name],
			Default:          annotations.Defaults[tags.Name],
		}
		if envs := annotations.DefaultEnvs[tags.Name]; len(envs) > 0 {
			spec.DefaultInfo = &schema.DefaultSpec{
				Environment: envs,
			}
		}
		props[tags.Name] = *spec
	}
	return props, required, nil
}

func resourceReferenceToken(
	t reflect.Type, extTag *introspect.ExplicitType, allowMissingExtType bool,
) (schema.TypeSpec, bool, error) {
	ptrT := reflect.PointerTo(t)
	implements := func(typ reflect.Type) bool {
		return t.Implements(typ) || ptrT.Implements(typ)
	}
	switch {
	// This handles both components and resources
	case implements(reflect.TypeOf(new(sch.Resource)).Elem()):
		tk, err := reflect.New(t).Elem().Interface().(sch.Resource).GetToken()
		return schema.TypeSpec{
			Ref: "#/resources/" + tk.String(),
		}, true, err
	case implements(reflect.TypeOf(new(pulumi.Resource)).Elem()):
		// This is an external resource
		if extTag == nil {
			if allowMissingExtType {
				return schema.TypeSpec{}, true, nil
			}
			return schema.TypeSpec{}, true, fmt.Errorf("missing type= tag on foreign resource %s", t)
		}
		tk := fmt.Sprintf("%s:%s:%s", extTag.Pkg, extTag.Module, extTag.Name)
		return schema.TypeSpec{
			Ref: fmt.Sprintf("/%s/%s/schema.json#/resources/%s", extTag.Pkg, extTag.Version, tk),
		}, true, nil
	default:
		return schema.TypeSpec{}, false, nil
	}
}

func structReferenceToken(t reflect.Type, extTag *introspect.ExplicitType) (schema.TypeSpec, bool, error) {
	if t.Kind() == reflect.Struct && extTag != nil {
		if extTag.Pkg != "" {
			return schema.TypeSpec{
				Ref: fmt.Sprintf("/%s/%s/schema.json#/types/%s:%s:%s",
					extTag.Pkg, extTag.Version,
					extTag.Pkg, extTag.Module, extTag.Name,
				),
			}, true, nil
		}
		return schema.TypeSpec{
			Ref: fmt.Sprintf("#/types/pkg:%s:%s", extTag.Module, extTag.Name),
		}, true, nil
	}
	if t.Kind() != reflect.Struct ||
		t.Implements(reflect.TypeOf(new(pulumi.Output)).Elem()) {
		return schema.TypeSpec{}, false, nil
	}

	tk, err := getTokenOf(t, nil)
	if err != nil {
		return schema.TypeSpec{}, true, err
	}

	return schema.TypeSpec{
		Ref: "#/types/" + tk.String(),
	}, true, nil
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
