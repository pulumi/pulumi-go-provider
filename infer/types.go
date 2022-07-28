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

	"github.com/pulumi/pulumi-go-provider/internal/introspect"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

type EnumKind interface {
	~string | ~float64 | ~bool | ~int
}

type Enum[T EnumKind] interface {
	Values() []EnumValue[T]
}

type EnumValue[T any] struct {
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

func isEnum(t reflect.Type) (enum, bool) {
	m, ok := t.MethodByName("Values")
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

	// We have found an enum. Now we call the Values() method on it and convert the result
	// back.
	result := m.Func.Call([]reflect.Value{reflect.New(t).Elem()})[0]
	values := make([]EnumValue[any], result.Len())
	for i := 0; i < result.Len(); i++ {
		v := result.Index(i)
		values[i] = EnumValue[any]{
			Value:       coerceToBase(v.FieldByName("Value")),
			Description: v.FieldByName("Description").Interface().(string),
		}
	}
	tk, err := introspect.GetToken("pkg", reflect.New(t).Elem().Interface())
	contract.AssertNoErrorf(err, "failed to get token for enum: %s", t)
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
