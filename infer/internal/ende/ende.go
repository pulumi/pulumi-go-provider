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

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	pmapper "github.com/pulumi/pulumi/sdk/v3/go/common/util/mapper"

	"github.com/pulumi/pulumi-go-provider/internal/introspect"
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
func Decode[T any](m resource.PropertyMap, dst T) (Encoder, pmapper.MappingError) {
	return decode(m, dst, false, false)
}

// DecodeTolerateMissing is like Decode, but doesn't return an error for a missing value.
func DecodeTolerateMissing[T any](m resource.PropertyMap, dst T) (Encoder, pmapper.MappingError) {
	return decode(m, dst, false, true)
}

func DecodeConfig[T any](m resource.PropertyMap, dst T) (Encoder, pmapper.MappingError) {
	return decode(m, dst, true, false)
}

func decode(
	m resource.PropertyMap, dst any, ignoreUnrecognized, allowMissing bool,
) (Encoder, pmapper.MappingError) {
	e := new(ende)
	target := reflect.ValueOf(dst)
	for target.Type().Kind() == reflect.Pointer && !target.IsNil() {
		target = target.Elem()
	}
	m = e.simplify(m, target.Type())
	return Encoder{e}, decodeProperty(m, target.Addr(), mapperOpts{
		IgnoreMissing:      allowMissing,
		IgnoreUnrecognized: ignoreUnrecognized,
	})

}

// An ENcoder DEcoder
type ende struct{ changes []change }

type change struct {
	path        resource.PropertyPath
	computed    bool // true if this output's value is known.
	secret      bool // true if this output's value is secret.
	forceOutput bool // true if this should be reserialized as an output.

	emptyAction int8
}

func (p change) apply(v resource.PropertyValue) resource.PropertyValue {
	if p.forceOutput {
		// Set v as an output preemptively.
		v = resource.NewOutputProperty(resource.Output{
			Element: v,
			Known:   true,
			Secret:  false,
		})
	}
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

func propertyPathEqual(s1, s2 resource.PropertyPath) bool {
	if len(s1) != len(s2) {
		return false
	}
	for i, v1 := range s1 {
		if v1 != s2[i] {
			return false
		}
	}
	return true
}

func (e *ende) mark(c change) {
	if len(e.changes) > 0 && propertyPathEqual(e.changes[len(e.changes)-1].path, c.path) {
		o := e.changes[len(e.changes)-1]
		c.computed = c.computed || o.computed
		c.secret = c.secret || o.secret
		if c.emptyAction == isNil {
			c.emptyAction = o.emptyAction
		}

		e.changes = e.changes[:len(e.changes)-1]
	}
	e.changes = append(e.changes, c)
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
		e.mark(change{path: path, secret: true})
		return el
	case v.IsComputed():
		el := e.walk(v.Input().Element, path, typ, true)
		e.mark(change{path: path, computed: true})
		return el
	case v.IsOutput():
		output := v.OutputValue()
		el := e.walk(output.Element, path, typ, !output.Known)
		e.mark(change{
			path:        path,
			computed:    !output.Known,
			secret:      output.Secret,
			forceOutput: true,
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
		switch {
		case v.IsArray():
			return e.walkArray(v, path, elemType, alignTypes)
		case v.IsObject():
			// We need to walk structs in a strongly typed way, so we omit
			// them here.
			if typ == nil || typ.Kind() != reflect.Struct {
				return e.walkMap(v, path, elemType, alignTypes)
			}
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
		return e.walkArray(v, path, elemType, alignTypes)
	case reflect.Map:
		return e.walkMap(v, path, elemType, alignTypes)
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

func (e *ende) walkArray(
	v resource.PropertyValue, path resource.PropertyPath,
	elemType reflect.Type, alignTypes bool,
) resource.PropertyValue {
	results := []resource.PropertyValue{}
	if v.IsArray() {
		arr := v.ArrayValue()
		if len(arr) == 0 {
			var action int8
			if arr != nil {
				action = isEmptyArr
			}
			e.mark(change{path: path, emptyAction: action})
		}
		results = make([]resource.PropertyValue, len(arr))
		for i, v := range arr {
			path := append(path, i)
			results[i] = e.walk(v, path, elemType, alignTypes)
		}
	}
	return resource.NewArrayProperty(results)
}

func (e *ende) walkMap(
	v resource.PropertyValue, path resource.PropertyPath,
	elemType reflect.Type, alignTypes bool,
) resource.PropertyValue {
	result := resource.PropertyMap{}
	if v.IsObject() {
		obj := v.ObjectValue()
		result = make(resource.PropertyMap, len(obj))
		if len(obj) == 0 {
			var action int8
			if obj != nil {
				action = isEmptyMap
			}
			e.mark(change{path: path, emptyAction: action})
		}
		for k, v := range obj {
			path := append(path, string(k))
			result[k] = e.walk(v, path, elemType, alignTypes)
		}
	}
	return resource.NewObjectProperty(result)
}

func (e *ende) Encode(src any) (resource.PropertyMap, pmapper.MappingError) {
	props, err := encodeProperty(src, mapperOpts{IgnoreMissing: true})
	if err != nil {
		return nil, err
	}
	m := resource.NewObjectProperty(props)
	contract.Assertf(!m.ContainsUnknowns(),
		"NewPropertyMapFromMap cannot produce unknown values")
	contract.Assertf(!m.ContainsSecrets(),
		"NewPropertyMapFromMap cannot produce secrets")
	for _, s := range e.changes {
		v, ok := s.path.Get(m)
		if !ok && s.emptyAction == isNil {
			continue
		}

		if s.emptyAction != isNil && v.IsNull() {
			switch s.emptyAction {
			case isEmptyMap:
				v = resource.NewObjectProperty(resource.PropertyMap{})
			case isEmptyArr:
				v = resource.NewArrayProperty([]resource.PropertyValue{})
			default:
				panic(s.emptyAction)
			}
		}

		s.path.Set(m, s.apply(v))
	}
	return m.ObjectValue(), nil
}

const (
	isNil      = iota
	isEmptyMap = iota
	isEmptyArr = iota
)

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
