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
	"reflect"

	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	"github.com/pulumi/pulumi-go-provider/internal/introspect"
	"github.com/pulumi/pulumi-go-provider/middleware/schema"
)

// The set of allowed enum underlying values.
type EnumKind interface {
	~string | ~float64 | ~bool | ~int
}

// An Enum in the Pulumi type system.
type Enum[T EnumKind] interface {
	// A list of all allowed values for the enum.
	Values() []EnumValue[T]
}

// An EnumValue represents an allowed value for an Enum.
type EnumValue[T any] struct {
	Name        string
	Value       T
	Description string
}

// A non-generic marker to determine that an enum value has been found.
type isEnumValue interface {
	isEnumValue()
}

func (EnumValue[T]) isEnumValue() {}

type enum struct {
	token  string
	values []EnumValue[any]
}

// isEnum detects if a type implements Enum[T] without naming T. There is no function to
// do this in the `reflect` package, so we implement this manually.
func isEnum(t reflect.Type) (enum, bool) {
	// To Simplify, we ensure that `t` is not a pointer type.
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	// Look for the "Value" method
	m, ok := t.MethodByName("Values")
	if !ok {
		// If it is not defined on T, maybe it is defined on *T
		m, ok = reflect.PointerTo(t).MethodByName("Values")
	}

	// The input is the receiver.
	if !ok || m.Type.NumIn() != 1 ||
		m.Type.NumOut() != 1 || m.Type.Out(0).Kind() != reflect.Slice {
		return enum{}, false
	}
	// We have now found a method with the right name and basic signature. We check that
	// it returns []EnumValue, by checking for implementation of a private method.
	isCorrectMethod := m.Type.Out(0).Elem().
		Implements(reflect.TypeOf(new(isEnumValue)).Elem())
	if !isCorrectMethod {
		return enum{}, false
	}

	// We have found an enum.
	// Now we construct the receiver, careful to distinguish between T and *T

	// If we should call via a pointer, set `t` to *T
	for target := m.Type.In(0); target.Kind() == reflect.Pointer; {
		target = target.Elem()
		t = reflect.PointerTo(t)
	}
	v := reflect.New(t).Elem()
	// Re-hydrate the value, ensuring we don't have a nil pointer.
	if v.Kind() == reflect.Pointer && v.IsNil() {
		v = reflect.New(v.Type().Elem())
	}

	// Call the function on the receiver.
	result := m.Func.Call([]reflect.Value{v})[0]

	// Iterate through the returned values, constructing a EnumValue of a known type: any.
	values := make([]EnumValue[any], result.Len())
	for i := 0; i < result.Len(); i++ {
		v := result.Index(i)
		values[i] = EnumValue[any]{
			Value:       coerceToBase(v.FieldByName("Value")),
			Description: v.FieldByName("Description").String(),
			Name:        v.FieldByName("Name").String(),
		}
	}

	var tk tokens.Type
	annotator := getAnnotated(t)
	if annotator.Token != "" {
		tk = fnToken(tokens.Type(annotator.Token))
	} else {
		var err error
		tk, err = introspect.GetToken("pkg", reflect.New(t).Elem().Interface())
		contract.AssertNoErrorf(err, "failed to get token for enum: %s", t)
	}

	return enum{
		token:  tk.String(),
		values: values,
	}, true
}

// Take a enum type and return it's base type.
//
// Example:
// type Foo string
// const foo Foo = "foo"
//
// Would result in coerseToBase(reflect.ValueOf(foo)) == string(foo)
func coerceToBase(v reflect.Value) any {
	switch v.Kind() {
	case reflect.String:
		return v.String()
	case reflect.Bool:
		return v.Bool()
	case reflect.Float64:
		return v.Float()
	case reflect.Int:
		return int(v.Int())
	default:
		panic("Unexpected value")
	}
}

type Crawler func(t reflect.Type) (drill bool, err error)

// crawlTypes recursively crawles T, calling the crawler on each new type it finds.
func crawlTypes[T any](crawler Crawler) error {
	var i T
	t := reflect.TypeOf(i)
	// Drill will walk the types, calling crawl on types it finds.
	var drill func(reflect.Type) error
	drill = func(t reflect.Type) error {
		nT, inputty, err := underlyingType(t)
		if err != nil {
			return err
		}
		if inputty {
			t = nT
		}
		switch t.Kind() {
		case reflect.String, reflect.Float64, reflect.Int, reflect.Bool:
			// Primitive types could be enums
			_, err := crawler(t)
			return err
		case reflect.Pointer, reflect.Array, reflect.Map, reflect.Slice:
			// Holds a reference to other types
			return drill(t.Elem())
		case reflect.Struct:
			for _, f := range reflect.VisibleFields(t) {
				info, err := introspect.ParseTag(f)
				if err != nil {
					return err
				}
				// The type is internal or it is a reference to an external package
				if info.Internal || (info.ExplicitRef != nil && info.ExplicitRef.Pkg != "") {
					continue
				}
				typ := f.Type
				for done := false; !done; {
					switch typ.Kind() {
					case reflect.Pointer, reflect.Array, reflect.Map, reflect.Slice:
						// Could hold a reference to other types
						typ = typ.Elem()
					default:
						nT, inputty, err := underlyingType(typ)
						if err != nil {
							return err
						}
						if inputty {
							typ = nT
						} else {
							done = true
						}
					}
				}
				further, err := crawler(typ)
				if err != nil {
					return err
				}
				if further {
					err = drill(typ)
					if err != nil {
						return err
					}
				}
			}
		}
		return nil
	}
	return drill(t)
}

// registerTypes recursively examines fields of T, calling reg on the schematized type when appropriate.
func registerTypes[T any](reg schema.RegisterDerivativeType) error {
	crawler := func(t reflect.Type) (bool, error) {
		if nT, inputty, err := underlyingType(t); err != nil {
			return false, err
		} else if inputty {
			t = nT
		}
		if t == reflect.TypeOf(resource.Asset{}) || t == reflect.TypeOf(resource.Archive{}) {
			return false, nil
		}
		if enum, ok := isEnum(t); ok {
			tSpec := pschema.ComplexTypeSpec{}
			for _, v := range enum.values {
				tSpec.Enum = append(tSpec.Enum, pschema.EnumValueSpec{
					Name:        "",
					Description: v.Description,
					Value:       v.Value,
				})
			}
			tSpec.Type = schemaNameForType(t.Kind())
			// We never need to recurse into primitive types
			_ = reg(tokens.Type(enum.token), tSpec)
			return false, nil
		}
		if _, ok, err := resourceReferenceToken(t, nil, true); ok {
			// This will have already been registered, so we don't need to recurse here
			return false, err
		}
		if t.Kind() == reflect.Struct {
			spec, err := objectSchema(t)
			if err != nil {
				return false, err
			}

			var tk tokens.Type
			annotator := getAnnotated(t)
			if annotator.Token != "" {
				tk = fnToken(tokens.Type(annotator.Token))
			} else {
				tk, err = introspect.GetToken("pkg", reflect.New(t).Interface())
				if err != nil {
					return false, err
				}
			}

			return reg(tk, pschema.ComplexTypeSpec{ObjectTypeSpec: *spec}), nil
		}
		return true, nil
	}
	return crawlTypes[T](crawler)
}
