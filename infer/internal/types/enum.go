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
	"reflect"

	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

type enum struct {
	token  string
	values []pschema.EnumValueSpec
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
	_, isCorrectMethod := m.Type.Out(0).Elem().MethodByName("isEnumValue")
	// isCorrectMethod := m.Type.Out(0).Elem().
	// 	Implements(reflect.TypeOf(new(isEnumValue)).Elem())
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
	values := make([]pschema.EnumValueSpec, result.Len())
	for i := 0; i < result.Len(); i++ {
		v := result.Index(i)
		values[i] = pschema.EnumValueSpec{
			Value:       coerceToBase(v.FieldByName("Value")),
			Description: v.FieldByName("Description").String(),
			Name:        v.FieldByName("Name").String(),
		}
	}

	tk, err := GetTokenOf(t, nil)
	contract.AssertNoErrorf(err, "failed to get token for enum: %s", t)

	return enum{
		token:  tk.String(),
		values: values,
	}, true
}

// Take a enum type and return it's base type.
//
// Example:
//
//	type Foo string
//	const foo Foo = "foo"
//
// Would result in `coerseToBase(reflect.ValueOf(foo)) == string(foo)`
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
