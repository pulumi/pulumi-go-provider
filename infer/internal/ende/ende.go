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
func Decode(m resource.PropertyMap, dst any) (Encoder, mapper.MappingError) {
	return decode(m, dst, false, false)
}

// DecodeTolerateMissing is like Decode, but doesn't return an error for a missing value.
func DecodeTolerateMissing(m resource.PropertyMap, dst any) (Encoder, mapper.MappingError) {
	return decode(m, dst, false, true)
}

func DecodeConfig(m resource.PropertyMap, dst any) (Encoder, mapper.MappingError) {
	return decode(m, dst, true, false)
}

func decode(
	m resource.PropertyMap, dst any, ignoreUnrecognized, allowMissing bool,
) (Encoder, mapper.MappingError) {
	e := new(ende)
	m = e.simplify(m, reflect.TypeOf(dst))
	target := reflect.ValueOf(dst)
	for target.Type().Kind() == reflect.Pointer && !target.IsNil() {
		target = target.Elem()
	}
	return Encoder{e}, mapper.New(&mapper.Opts{
		IgnoreUnrecognized: ignoreUnrecognized,
		IgnoreMissing:      allowMissing,
	}).Decode(m.Mappable(), target.Addr().Interface())

}

// An ENcoder DEcoder
type ende struct{ changes []path }

type secretPath struct {
	path resource.PropertyPath
}

type computedPath struct {
	path resource.PropertyPath
}

type outputPath struct {
	path         resource.PropertyPath
	known        bool           // true if this output's value is known.
	secret       bool           // true if this output's value is secret.
	dependencies []resource.URN // the dependencies associated with this output.
}

func (p outputPath) get() resource.PropertyPath { return p.path }
func (p outputPath) apply(v resource.PropertyValue) resource.PropertyValue {
	return resource.NewOutputProperty(resource.Output{
		Element:      v,
		Known:        p.known,
		Secret:       p.secret,
		Dependencies: p.dependencies,
	})
}

func (p secretPath) get() resource.PropertyPath { return p.path }
func (p secretPath) apply(v resource.PropertyValue) resource.PropertyValue {
	return resource.MakeSecret(v)
}

func (p computedPath) get() resource.PropertyPath { return p.path }
func (p computedPath) apply(v resource.PropertyValue) resource.PropertyValue {
	return resource.MakeComputed(v)
}

type path interface {
	get() resource.PropertyPath
	apply(resource.PropertyValue) resource.PropertyValue
}

func (e *ende) simplify(m resource.PropertyMap, dst reflect.Type) resource.PropertyMap {
	var walk func(
		resource.PropertyValue, resource.PropertyPath, reflect.Type,
	) resource.PropertyValue
	walk = func(
		v resource.PropertyValue, path resource.PropertyPath, typ reflect.Type,
	) resource.PropertyValue {
		for typ != nil && typ.Kind() == reflect.Pointer {
			typ = typ.Elem()
		}
		switch {
		case v.IsSecret():
			// To allow full fidelity reconstructing maps, we extract nested secrets
			// first. We then extract the top level secret. We need this ordering to
			// re-embed nested secrets.
			el := walk(v.SecretValue().Element, path, typ)
			e.changes = append(e.changes, secretPath{path})
			return el
		case v.IsComputed():
			el := walk(v.Input().Element, path, typ)
			e.changes = append(e.changes, computedPath{path})
			return el
		case v.IsOutput():
			v := v.OutputValue()
			e.changes = append(e.changes, outputPath{
				path:         path,
				known:        v.Known,
				secret:       v.Secret,
				dependencies: v.Dependencies,
			})
			// We assume that useful information is not encoded in the unknown
			// value behind an Output.
			if !v.Known {
				return walk(zeroValueOf(typ), path, typ)
			}
			return walk(v.Element, path, typ)
		case v.IsArray():
			arr := make([]resource.PropertyValue, len(v.ArrayValue()))
			var elem reflect.Type
			if typ != nil {
				switch typ.Kind() {
				case reflect.Array, reflect.Slice:
					elem = typ.Elem()
				}
			}
			for i, e := range v.ArrayValue() {
				arr[i] = walk(e, append(path, i), elem)
			}
			return resource.NewArrayProperty(arr)
		case v.IsObject():
			m := make(resource.PropertyMap, len(v.ObjectValue()))
			for k, v := range v.ObjectValue() {
				elem := fieldType(string(k), typ)
				m[k] = walk(v, append(path, string(k)), elem)
			}
			return resource.NewObjectProperty(m)
		default:
			return v
		}
	}

	newMap := make(resource.PropertyMap, len(m))
	for k, v := range m {
		newMap[k] = walk(v, resource.PropertyPath{string(k)}, fieldType(string(k), dst))
	}

	return newMap
}

func fieldType(name string, typ reflect.Type) reflect.Type {
	if typ == nil {
		return nil
	}
	if typ.Kind() == reflect.Map {
		return typ.Elem()
	}
	if typ.Kind() != reflect.Struct {
		return nil
	}

	for _, f := range reflect.VisibleFields(typ) {
		tag, err := introspect.ParseTag(f)
		if err != nil || tag.Internal {
			continue
		}
		if tag.Name == name {
			return f.Type
		}
	}
	return nil
}

func zeroValueOf(typ reflect.Type) resource.PropertyValue {
	var kind reflect.Kind
	if typ != nil {
		kind = typ.Kind()
	}
	switch kind {
	case reflect.Struct, reflect.Map:
		return resource.NewObjectProperty(resource.PropertyMap{})
	case reflect.String:
		return resource.NewStringProperty("")
	case reflect.Bool:
		return resource.NewBoolProperty(false)
	case reflect.Int, reflect.Int64, reflect.Float32, reflect.Float64:
		return resource.NewNumberProperty(0)
	case reflect.Array, reflect.Slice:
		return resource.NewArrayProperty([]resource.PropertyValue{})
	default:
		return resource.NewNullProperty()
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
	for _, s := range e.changes {
		path := s.get()
		v, ok := path.Get(m)
		if !ok {
			continue
		}
		path.Set(m, s.apply(v))
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

	changes := make([]path, 0, len(e.changes))

	for _, v := range e.changes {
		switch v := v.(type) {
		case outputPath:
			v.known = true
			changes = append(changes, v)
		case computedPath:
			// This no longer applies
		default:
			changes = append(changes, v)
		}
	}

	return Encoder{&ende{changes}}
}
