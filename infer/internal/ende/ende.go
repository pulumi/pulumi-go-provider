// Copyright 2023, Pulumi Corporation.
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

package ende

import (
	"reflect"

	"github.com/pulumi/pulumi-go-provider/internal/introspect"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/mapper"
)

type Encoder struct{ *ende }

// Decode a property map to a `pulumi:"x"` annotated struct.
//
// The returned mapper can restore the metadata it removed when translating `dst` back to
// a property map. If the shape of `T` matches `m`, then this will be a no-op:
//
//	var value T
//	encoder, _ := Decode(m, &value)
//	m, _ = encoder.Encode(value)
func Decode[T any](m resource.PropertyMap, dst T) (Encoder, mapper.MappingError) {
	return decode(m, dst, false, false)
}

// DecodeTolerateMissing is like Decode, but doesn't return an error for a missing value.
func DecodeTolerateMissing[T any](m resource.PropertyMap, dst T) (Encoder, mapper.MappingError) {
	return decode(m, dst, false, true)
}

func DecodeConfig[T any](m resource.PropertyMap, dst T) (Encoder, mapper.MappingError) {
	return decode(m, dst, true, false)
}

func decode[T any](
	m resource.PropertyMap, dst T, ignoreUnrecognized, allowMissing bool,
) (Encoder, mapper.MappingError) {
	e := new(ende)
	target := reflect.ValueOf(dst)
	for target.Type().Kind() == reflect.Pointer && !target.IsNil() {
		target = target.Elem()
	}
	m = e.simplify(m, target.Type())
	return Encoder{e}, mapper.New(&mapper.Opts{
		IgnoreUnrecognized: ignoreUnrecognized,
		IgnoreMissing:      allowMissing,
	}).Decode(m.Mappable(), target.Addr().Interface())

}

// An ENcoder DEcoder
type ende struct{ changes []change }

type change struct {
	path     resource.PropertyPath
	computed bool // true if this output's value is known.
	secret   bool // true if this output's value is secret.
}

func (p change) apply(v resource.PropertyValue) resource.PropertyValue {
	if p.computed {
		v = MakeComputed(v)
	}
	if p.secret {
		v = MakeSecret(v)
	}
	return v
}

func (e *ende) simplify(m resource.PropertyMap, dst reflect.Type) resource.PropertyMap {
	return e.walk(
		resource.NewObjectProperty(m),
		resource.PropertyPath{},
		dst,
		false, /* align types */
	).ObjectValue()
}

func (e *ende) walk(
	v resource.PropertyValue, path resource.PropertyPath, typ reflect.Type,
	alignTypes bool,
) resource.PropertyValue {
	if typ == nil {
		// We can't align types when we don't have type info
		alignTypes = false
	} else {
		for typ.Kind() == reflect.Pointer {
			typ = typ.Elem()
		}
	}

	switch {
	case v.IsSecret():
		// To allow full fidelity reconstructing maps, we extract nested secrets
		// first. We then extract the top level secret. We need this ordering to
		// re-embed nested secrets.
		el := e.walk(v.SecretValue().Element, path, typ, alignTypes)
		e.changes = append(e.changes, change{path: path, secret: true})
		return el
	case v.IsComputed():
		el := e.walk(v.Input().Element, path, typ, true)
		e.changes = append(e.changes, change{path: path, computed: true})
		return el
	case v.IsOutput():
		output := v.OutputValue()
		el := e.walk(output.Element, path, typ, !output.Known)
		e.changes = append(e.changes, change{
			path:     path,
			computed: !output.Known,
			secret:   output.Secret,
		})

		return el
	}

	var elemType reflect.Type
	if typ != nil {
		switch typ.Kind() {
		case reflect.Slice, reflect.Array, reflect.Map:
			elemType = typ.Elem()
		}
	}

	if !alignTypes {
	handle:
		switch {
		case v.IsArray():
			var results []resource.PropertyValue
			results = make([]resource.PropertyValue, len(v.ArrayValue()))
			for i, v := range v.ArrayValue() {
				path := append(path, i)
				results[i] = e.walk(v, path, elemType, alignTypes)
			}
			return resource.NewArrayProperty(results)
		case v.IsObject():
			if typ != nil && typ.Kind() == reflect.Struct {
				break handle
			}

			var result resource.PropertyMap
			if v.IsObject() {
				result = make(resource.PropertyMap, len(v.ObjectValue()))
				for k, v := range v.ObjectValue() {
					path := append(path, string(k))
					result[k] = e.walk(v, path, elemType, alignTypes)
				}
			}
			return resource.NewObjectProperty(result)
		// This is a scalar value, so we can return it as is.
		default:
			return v
		}
	}

	contract.Assertf(!IsComputed(v), "failed to strip computed")
	contract.Assertf(!IsSecret(v), "failed to strip secrets")
	contract.Assertf(!v.IsOutput(), "failed to strip outputs")

	switch typ.Kind() {
	case reflect.Array, reflect.Slice:
		var results []resource.PropertyValue
		if v.IsArray() {
			results = make([]resource.PropertyValue, len(v.ArrayValue()))
			for i, v := range v.ArrayValue() {
				path := append(path, i)
				results[i] = e.walk(v, path, elemType, alignTypes)
			}
		}
		return resource.NewArrayProperty(results)
	case reflect.Map:
		var result resource.PropertyMap
		if v.IsObject() {
			result = make(resource.PropertyMap, len(v.ObjectValue()))
			for k, v := range v.ObjectValue() {
				path := append(path, string(k))
				result[k] = e.walk(v, path, elemType, alignTypes)
			}
		}
		return resource.NewObjectProperty(result)
	case reflect.Struct:
		result := resource.PropertyMap{}
		if v.IsObject() {
			result = v.ObjectValue().Copy()
		}
		for _, field := range reflect.VisibleFields(typ) {
			tag, err := introspect.ParseTag(field)
			if err != nil || tag.Internal {
				continue
			}
			pName := resource.PropertyKey(tag.Name)
			path := append(path, tag.Name)
			if v, ok := result[pName]; ok {
				result[pName] = e.walk(v, path, field.Type, alignTypes)
			} else {
				if tag.Optional || !alignTypes {
					continue
				}
				// Create a new unknown output, which we will then type
				result[pName] = e.walk(resource.NewNullProperty(),
					path, field.Type, true)
			}
		}
		return resource.NewObjectProperty(result)
	case reflect.String:
		if v.IsString() {
			return v
		}
		return resource.NewStringProperty("")
	case reflect.Bool:
		if v.IsBool() {
			return v
		}
		return resource.NewBoolProperty(false)
	case reflect.Int, reflect.Int64, reflect.Float32, reflect.Float64:
		if v.IsNumber() {
			return v
		}
		return resource.NewNumberProperty(0)
	default:
		return v
	}
}

func (e *ende) Encode(src any) (resource.PropertyMap, mapper.MappingError) {
	props, err := mapper.New(&mapper.Opts{}).Encode(src)
	if err != nil {
		return nil, err
	}
	m := resource.NewObjectProperty(
		resource.NewPropertyMapFromMap(props),
	)
	contract.Assertf(!m.ContainsUnknowns(),
		"NewPropertyMapFromMap cannot produce unknown values")
	contract.Assertf(!m.ContainsSecrets(),
		"NewPropertyMapFromMap cannot produce secrets")
	for _, s := range e.changes {
		v, ok := s.path.Get(m)
		if !ok {
			continue
		}
		s.path.Set(m, s.apply(v))
	}
	return m.ObjectValue(), nil
}

// Mark a encoder as generating values only.
//
// This is appropriate when you are encoding a value where all fields must be known, such
// as a non-preview create or update.
func (e *ende) AllowUnknown(allowUnknowns bool) Encoder {
	if allowUnknowns {
		return Encoder{e}
	}

	// If we don't allow unknowns, strip all fields that can accept them.

	changes := make([]change, 0, len(e.changes))

	for _, v := range e.changes {
		v.computed = false
		changes = append(changes, v)
	}

	return Encoder{&ende{changes}}
}