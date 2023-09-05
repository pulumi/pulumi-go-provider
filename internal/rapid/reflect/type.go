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

package reflect

import (
	"fmt"
	"reflect"

	"pgregory.net/rapid"
)

type GenerateType = *rapid.Generator[reflect.Type]

var pulumiFieldName = rapid.StringMatching("[a-zA-Z]+")
var structFieldName = rapid.StringMatching("[A-Z][a-zA-Z]*")

func String() GenerateType { return rapid.Just(reflect.TypeOf("")) }
func Bool() GenerateType   { return rapid.Just(reflect.TypeOf(false)) }
func Number() GenerateType { return rapid.Just(reflect.TypeOf(float64(0.0))) }
func Null() GenerateType   { return rapid.Just(reflect.TypeOf(nil)) }
func Struct(maxDepth int) GenerateType {
	return rapid.Custom(func(t *rapid.T) reflect.Type {
		if maxDepth <= 0 {
			t.Fatalf("Cannot create struct with maxDepth <= 0")
		}
		numFields := rapid.IntRange(1, 8).Draw(t, "numFields")
		fieldNames := rapid.SliceOfNDistinct(
			structFieldName, numFields, numFields, rapid.ID[string],
		).Draw(t, "fieldNames")
		pulumiNames := rapid.SliceOfNDistinct(
			pulumiFieldName, numFields, numFields, rapid.ID[string],
		).Draw(t, "pulumiNames")
		structFields := make([]reflect.StructField, numFields)
		for i := range fieldNames {
			var optional string
			typ := Type(maxDepth - 1)

			// Make some types optional:
			//
			// If a type is optional, then we ensure that the type is a
			// pointer type. This makes the space explored by this test
			// smaller then our actual input space, but allows us to fully
			// round trip our values. Consider the following:
			//
			//	type Bool struct { field bool `pulumi:"b,optional"` }
			//
			// When we round trip the empty map (`resource.PropertyMap{}`)
			// through `ende`'s decode->endcode through `Bool`, we get back:
			//
			//	resource.PropertyMap{ "b": resource.PropertyValue{V: false}
			//
			// Our round trip process isn't able to distinguish between the
			// zero value for a non-ptr type and no value. To allow us to test
			// round tripping, we don't generate these problematic values as
			// inputs.
			//
			// The difference should not be important, since the library user
			// will not be able to see the difference (they look at the
			// de-serialized version).
			if rapid.Bool().Draw(t, fmt.Sprintf("optional-%d", i)) {
				optional = ",optional"
				typ = PtrOf(typ)
			}

			tag := fmt.Sprintf(`pulumi:"%s%s"`, pulumiNames[i], optional)

			structFields[i] = reflect.StructField{
				Name: fieldNames[i],
				Type: typ.Draw(t, "type"),
				Tag:  reflect.StructTag(tag),
				// TODO: Consider a mechanism for adding Anonymous.
			}
		}

		return reflect.StructOf(structFields)
	})
}

func Type(maxDepth int) GenerateType {
	if maxDepth <= 1 {
		return Primitive()
	}
	return rapid.OneOf(
		Primitive(),
		Struct(maxDepth-1),
		Ptr(maxDepth-1),
		Array(maxDepth-1),
		Map(maxDepth-1),
	)
}

func Primitive() GenerateType {
	return rapid.OneOf(
		String(),
		Bool(),
		Number(),
	)
}

func Ptr(maxDepth int) GenerateType   { return PtrOf(Type(maxDepth - 1)) }
func Map(maxDepth int) GenerateType   { return MapOf(Type(maxDepth - 1)) }
func Array(maxDepth int) GenerateType { return ArrayOf(Type(maxDepth - 1)) }

// Return a pointer to typ.
//
// If typ is already a pointer, typ is returned as is.
func PtrOf(typ GenerateType) GenerateType {
	return rapid.Custom(func(t *rapid.T) reflect.Type {
		elem := typ.Draw(t, "elem")
		if elem.Kind() == reflect.Pointer {
			return elem
		}
		return reflect.PointerTo(elem)
	})
}

func ArrayOf(typ GenerateType) GenerateType {
	return rapid.Custom(func(t *rapid.T) reflect.Type {
		return reflect.SliceOf(typ.Draw(t, "elem"))
	})
}
func MapOf(typ GenerateType) GenerateType {
	return rapid.Custom(func(t *rapid.T) reflect.Type {
		return reflect.MapOf(reflect.TypeOf(""), typ.Draw(t, "elem"))
	})
}
