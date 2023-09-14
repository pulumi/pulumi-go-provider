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
	"os"
	"reflect"
	"strconv"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	"github.com/pulumi/pulumi-go-provider/internal/introspect"
)

// The object that controls default application.
type defaultsWalker struct {
	// seen is the stack of types that defaultsWalker has descended into.
	seen []reflect.Type
}

// Mark that we are visiting a type.
//
// The return value of of this function should be immediately deferred:
//
//	defer d.visit(t)()
func (d *defaultsWalker) visit(t reflect.Type) func() {
	d.seen = append(d.seen, t)
	pop := func() {
		d.seen = d.seen[:len(d.seen)-1]
	}
	return pop
}

func (d *defaultsWalker) hydrate(t reflect.Type) bool {
	for _, v := range d.seen {
		if v == t {
			return false
		}
	}
	return true
}

// apply default values to structs.
func (d *defaultsWalker) apply(v reflect.Value) (bool, error) {
	t := v.Type()
	defer d.visit(t)()

	// We get the set of default types that could be applied to v.
	a := getAnnotated(t)
	fields := map[string]reflect.Value{}
	optional := map[string]bool{}
	for _, field := range reflect.VisibleFields(v.Type()) {
		tag, err := introspect.ParseTag(field)
		if err != nil {
			return false, err
		}
		if tag.Internal {
			continue
		}

		optional[tag.Name] = tag.Optional
		fields[tag.Name] = v.FieldByIndex(field.Index)
	}

	// We not apply the defaults we calculated:
	//
	// We start by attempting to apply environmental values in order. If no
	// environmental values are set to a non "" value, we then set from in-memory
	// values.

	// If v is a nil valued struct with defaults, we hydrate it and apply the
	// default. If not, we leave it nil.

	var didSet bool
defaultEnvs:
	for k, envVars := range a.DefaultEnvs {
		value, ok := fields[k]
		if ok && !value.IsZero() {
			continue
		}
		for _, env := range envVars {
			envValue := os.Getenv(env)
			if envValue == "" {
				continue
			}
			err := setDefaultValueFromEnv(value, envValue)
			if err != nil {
				return false, err
			}
			didSet = true
			continue defaultEnvs
		}
	}

	for k, inMemoryDefault := range a.Defaults {
		value, ok := fields[k]
		if ok && !value.IsZero() {
			continue
		}
		err := setDefaultFromMemory(value, inMemoryDefault)
		if err != nil {
			return false, err
		}
		didSet = true
	}

	// Default values only apply to primitive types, but this struct could have fields
	// that itself has default values. We need to traverse those.
	//
	// We only recurse on pulumi tagged fields.
	for k, v := range fields {
		// We do not apply defaults (or hydrate) a field `K: *T` when `K` is
		// optional and `*T` is nil. This shields us from the hydrating an
		// optional struct with a required value, which fails de-serialization.
		if optional[k] && isNilStructPtr(v) {
			continue
		}
		didSetRec, err := d.walk(v)
		if err != nil {
			return false, err
		}
		if didSetRec {
			didSet = true
		}
	}
	return didSet, nil
}

// isNilStructPtr checks if v is a nil pointer to a struct.
func isNilStructPtr(v reflect.Value) bool {
	for v.Kind() == reflect.Pointer && !v.IsNil() {
		v = v.Elem()
	}
	t := v.Type()
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return v.Kind() == reflect.Pointer && t.Kind() == reflect.Struct
}

// walk is responsible for calling applyDefaults on all structs in reachable from value.
func (d *defaultsWalker) walk(v reflect.Value) (didSet bool, _ error) {
	tDeferenced := v.Type()
	for tDeferenced.Kind() == reflect.Pointer {
		tDeferenced = tDeferenced.Elem()
	}
	switch tDeferenced.Kind() {
	// We apply defaults to each value in slice.
	case reflect.Slice:
		// Dereference to the underlying slice
		for v.Kind() == reflect.Pointer && !v.IsNil() {
			v = v.Elem()
		}
		// Either we have a type *[]T, **[]T, etc. and a pointer is nil,
		// or we have reached the slice and the slice itself is nil. Both
		// cases prevent us finding any more structs, so we are done.
		if v.IsNil() {
			return didSet, nil
		}
		for i := 0; i < v.Len(); i++ {
			didSetRec, err := d.walk(v.Index(i))
			if err != nil {
				return false, err
			}
			if didSetRec {
				didSet = true
			}
		}
		return didSet, nil

	case reflect.Array:
		// Dereference to the underlying slice
		for v.Kind() == reflect.Pointer {
			if v.IsNil() {
				return false, nil
			}
			v = v.Elem()
		}

		// Arrays cannot be nil, so we don't (and can't) perform a nil
		// check here.
		for i := 0; i < v.Len(); i++ {
			didSetRec, err := d.walk(v.Index(i))
			if err != nil {
				return false, err
			}
			if didSetRec {
				didSet = true
			}
		}
		return didSet, nil

	case reflect.Map:
		// Dereference to the underlying map
		for v.Kind() == reflect.Pointer && !v.IsNil() {
			v = v.Elem()
		}
		// Either we have a type *map[K]V, **map[K]V, etc. and a pointer
		// is nil, or we have reached the map and the map itself is
		// nil. Both cases prevent us finding any more structs, so we are
		// done.
		if v.IsNil() {
			return false, nil
		}
		iter := v.MapRange()
		for iter.Next() {
			value := reflect.New(iter.Value().Type()).Elem()
			value.Set(iter.Value())
			s := hydratedValue(value)
			didSetRec, err := d.walk(s)
			if err != nil {
				return false, err
			}
			if didSetRec {
				v.SetMapIndex(iter.Key(), s)
				didSet = true
			}
		}
		return didSet, nil
	case reflect.Struct:
		// Copying what Go SDKs do, we only populate structs that are accessible
		// by value or by filled pointers.
		structCopy := reflect.New(v.Type()).Elem()
		structCopy.Set(v)
		var s reflect.Value
		if d.hydrate(tDeferenced) {
			// We should hydrate the value and apply defaults to it.
			s = hydratedValue(structCopy)
		} else {
			// We should fill a live value, but not hydrate a new one.  Set s
			// to the copy, assuming that we will have a non-nil struct.
			s = structCopy

			// Now check that we have a non-nil struct.
			for structCopy.Kind() == reflect.Pointer {
				if structCopy.IsNil() {
					return false, nil
				}
				structCopy = structCopy.Elem()
			}
		}
		didSet, err := d.apply(derefNonNil(s))
		if err != nil {
			return false, err
		}
		if didSet {
			v.Set(s)
		}
		return didSet, nil

	// This is a primitive type. That means that:
	//
	// 1. It is not a struct.
	// 2. It cannot contain a struct.
	//
	// That means there are not any defaults to apply, so we're done.
	default:
		return false, nil
	}
}

// applyDefaults recursively applies the default values provided by [introspect.Annotator].
func applyDefaults[T any](value *T) error {
	v := reflect.ValueOf(value).Elem()
	contract.Assertf(v.CanSet(), "Cannot accept an un-editable pointer")

	var walker defaultsWalker

	_, err := walker.walk(v)
	return err
}

// setDefaultFromMemory applies an in-memory default value to a field that can accept one.
//
// field must be CanSet and value must either be a primitive, or point to one.
func setDefaultFromMemory(field reflect.Value, value any) error {
	if value == nil {
		return nil
	}
	// We will set field to a primitive value, we can freely provide hydration.
	field.Set(hydratedValue(field))
	field = derefNonNil(field)

	v := reflect.ValueOf(value)
	if v.CanConvert(field.Type()) {
		field.Set(v.Convert(field.Type()))
		return nil
	}
	return fmt.Errorf("cannot set field of type '%s' to default value %q (%[2]T)",
		field.Type(), value)
}

// setDefaultValueFromEnv applies a default value source from an environmental variable to
// a field.
//
// field must be CanSet and value must either be a primitive, or point to one.
func setDefaultValueFromEnv(field reflect.Value, value string) error {
	typ := field.Type()
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}

	switch typ.Kind() {
	case reflect.String:
		return setDefaultFromMemory(field, value)
	case reflect.Float64:
		f, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return err
		}
		return setDefaultFromMemory(field, f)
	case reflect.Int:
		i, err := strconv.ParseInt(value, 0, 64)
		if err != nil {
			return err
		}
		return setDefaultFromMemory(field, i)
	case reflect.Bool:
		b, err := strconv.ParseBool(value)
		if err != nil {
			return err
		}
		return setDefaultFromMemory(field, b)
	}
	return fmt.Errorf("unable to apply default value %q to field of type %s",
		value, typ)
}

// hydratedValue takes a (possibly ptr) and returns a fully hydrated value of the same
// type. For example:
//
//	T{}        -> T{}
//	T{N: 1}    -> T{N: 1}
//	&T{N: 2}   -> &T{N: 2}
//	(*T)(nil)  -> &T{}
//	(**T)(nil) -> &&T{}
func hydratedValue(value reflect.Value) reflect.Value {
	// root := *&T where T = typeof(value)
	//
	// This creates an addressable value of the same type as value
	root := value
	for value.Kind() == reflect.Pointer {
		// We have *T, so we need to construct a T and set the value to it.
		if value.IsNil() {
			// This is the reflect equivalent of:
			//
			//	var v typeof(value)
			//	*elem := value
			v := reflect.New(value.Type().Elem())
			value.Set(v)
		}
		value = value.Elem()
	}
	return root
}

func derefNonNil(value reflect.Value) reflect.Value {
	for value.Kind() == reflect.Pointer {
		value = value.Elem()
	}
	return value
}
