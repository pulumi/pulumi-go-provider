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

package resource

import (
	"reflect"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"pgregory.net/rapid"
)

type Typed struct {
	Type  reflect.Type
	Value resource.PropertyValue
}

// Annotate a type with a value that fits inside of it.
func Type(typ *rapid.Generator[reflect.Type]) *rapid.Generator[Typed] {
	// Note: we generate values from types instead of vice versa because the set of
	// possible resource.PropertyValue is much larger then the set of possible
	// reflect.Type.
	//
	// For example: []{number(1), bool(true)} is a valid resource.PropertyValue but
	// not a valid reflect.Type.
	return rapid.Custom(func(t *rapid.T) Typed {
		typ := typ.Draw(t, "typ")
		return Typed{typ, valueOf(typ, false).Draw(t, "value")}
	})
}

func valueOf(typ reflect.Type, allowMarker bool) *rapid.Generator[resource.PropertyValue] {
	if typ == nil {
		return Null()
	}

	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}

	var v *rapid.Generator[resource.PropertyValue]

	switch typ.Kind() {
	case reflect.String:
		v = String()
	case reflect.Float64:
		v = Number()
	case reflect.Bool:
		v = Bool()
	case reflect.Map:
		v = MapOf(valueOf(typ.Elem(), true))
	case reflect.Slice:
		v = ArrayOf(valueOf(typ.Elem(), true))
	case reflect.Struct:
		v = structOf(reflect.VisibleFields(typ))
	default:
		panic(typ)
	}

	if allowMarker {
		return maybeMarked(v)
	}
	return v
}

func maybeMarked(v *rapid.Generator[resource.PropertyValue]) *rapid.Generator[resource.PropertyValue] {
	return rapid.Custom(func(t *rapid.T) resource.PropertyValue {
		v := v.Draw(t, "value")
		if rapid.Bool().Draw(t, "make-secret") {
			v = makeSecret(t, v)
		}
		if rapid.Bool().Draw(t, "make-computed") {
			v = makeComputed(t, v)
		}
		return v
	})
}

func structOf(fields []reflect.StructField) *rapid.Generator[resource.PropertyValue] {
	return rapid.Custom(func(t *rapid.T) resource.PropertyValue {
		pMap := resource.PropertyMap{}

		for _, f := range fields {
			tag := string(f.Tag)
			name := strings.TrimPrefix(tag, `pulumi:"`)
			if strings.Contains(tag, ",optional") {
				if rapid.Bool().Draw(t, "skip-optional-field") {
					continue
				}
				name = name[:strings.IndexRune(name, ',')]
			}
			if i := strings.IndexRune(name, '"'); i > 0 {
				name = name[:i]
			}
			pMap[resource.PropertyKey(name)] = valueOf(f.Type, true).Draw(t, "field")
		}

		return resource.NewObjectProperty(pMap)
	})
}

func PropertyValue() *rapid.Generator[resource.PropertyValue] {
	return rapid.OneOf(
		String(),
		Bool(),
		Number(),
		Null(),
		Array(),
		Object(),
		Secret(),
		Computed(),
	)
}

func PropertyKey() *rapid.Generator[resource.PropertyKey] {
	return rapid.Custom(func(t *rapid.T) resource.PropertyKey {
		return resource.PropertyKey(rapid.String().Draw(t, "V"))
	})
}

func String() *rapid.Generator[resource.PropertyValue] {
	return rapid.Custom(func(t *rapid.T) resource.PropertyValue {
		return resource.NewStringProperty(rapid.String().Draw(t, "V"))
	})
}

func Bool() *rapid.Generator[resource.PropertyValue] {
	return rapid.Custom(func(t *rapid.T) resource.PropertyValue {
		return resource.NewBoolProperty(rapid.Bool().Draw(t, "V"))
	})
}

func Number() *rapid.Generator[resource.PropertyValue] {
	return rapid.Custom(func(t *rapid.T) resource.PropertyValue {
		return resource.NewNumberProperty(rapid.Float64().Draw(t, "V"))
	})
}

func Null() *rapid.Generator[resource.PropertyValue] {
	return rapid.Just(resource.NewNullProperty())
}

func ArrayOf(value *rapid.Generator[resource.PropertyValue]) *rapid.Generator[resource.PropertyValue] {
	return rapid.Custom(func(t *rapid.T) resource.PropertyValue {
		return resource.NewArrayProperty(rapid.SliceOf(value).Draw(t, "V"))
	})
}

func Array() *rapid.Generator[resource.PropertyValue] { return ArrayOf(PropertyValue()) }

func MapOf(value *rapid.Generator[resource.PropertyValue]) *rapid.Generator[resource.PropertyValue] {
	return rapid.Custom(func(t *rapid.T) resource.PropertyValue {
		return resource.NewObjectProperty(rapid.MapOf(
			PropertyKey(),
			value,
		).Draw(t, "V"))
	})
}

func Object() *rapid.Generator[resource.PropertyValue] { return MapOf(PropertyValue()) }

func Secret() *rapid.Generator[resource.PropertyValue] {
	return rapid.Custom(func(t *rapid.T) resource.PropertyValue {
		v := PropertyValue().Draw(t, "V")
		return makeSecret(t, v)
	})
}

func makeSecret(t *rapid.T, v resource.PropertyValue) resource.PropertyValue {
	// If a value is marker, we fold the secretness into it.
	if v.IsSecret() {
		return v
	}
	if v.IsComputed() {
		return resource.NewOutputProperty(resource.Output{
			Element: v.Input().Element,
			Known:   false,
			Secret:  true,
		})
	}
	if v.IsOutput() {
		o := v.OutputValue()
		o.Secret = true
		return resource.NewOutputProperty(o)
	}

	// Otherwise we pick between the two kinds of secretness we can accept.
	if rapid.Bool().Draw(t, "isOutput") {
		return resource.NewOutputProperty(resource.Output{
			Element: v,
			Known:   true,
			Secret:  true,
		})
	}
	return resource.MakeSecret(v)
}

func Computed() *rapid.Generator[resource.PropertyValue] {
	return rapid.Custom(func(t *rapid.T) resource.PropertyValue {
		v := PropertyValue().Draw(t, "V")
		return makeComputed(t, v)
	})
}

func makeComputed(t *rapid.T, v resource.PropertyValue) resource.PropertyValue {
	// If a value is marker, we fold the computedness into it.
	if v.IsComputed() {
		return v
	}
	if v.IsSecret() {
		return resource.NewOutputProperty(resource.Output{
			Element: v.SecretValue().Element,
			Known:   false,
			Secret:  true,
		})
	}
	if v.IsOutput() {
		o := v.OutputValue()
		o.Known = false
		return resource.NewOutputProperty(o)
	}

	// Otherwise we pick between the two kinds of secretness we can accept.
	if rapid.Bool().Draw(t, "isOutput") {
		return resource.MakeOutput(v)
	}
	return resource.MakeComputed(v)

}
