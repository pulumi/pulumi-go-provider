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

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	"github.com/pulumi/pulumi-go-provider/infer/internal/ende"
	"github.com/pulumi/pulumi-go-provider/internal/introspect"
)

// The object that controls default application.
type secretsWalker struct{ errs []error }

func (w *secretsWalker) walk(t reflect.Type, p resource.PropertyValue) (out resource.PropertyValue) {
	// If v is untyped, we have no information, so return.
	if t == nil {
		return p
	}

	// Ensure we are working in raw value types for p

	if ende.IsSecret(p) {
		p = ende.MakePublic(p)
		defer func() { out = ende.MakeSecret(p) }()
	}

	if ende.IsComputed(p) {
		p = ende.MakeKnown(p)
		defer func() { out = ende.MakeComputed(p) }()
	}

	// Ensure we are working in raw value types for t

	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	switch t.Kind() {
	// Structs can carry secret information, so this is where we add secrets.
	case reflect.Struct:
		if !p.IsObject() {
			return p // p and t mismatch, so return early
		}
		obj := p.ObjectValue()

		for _, field := range reflect.VisibleFields(t) {
			info, err := introspect.ParseTag(field)
			if err != nil {
				w.errs = append(w.errs, err)
				continue
			}

			v, ok := obj[resource.PropertyKey(info.Name)]
			if !ok {
				continue
			}
			field, ok := t.FieldByName(field.Name)
			contract.Assertf(ok,
				"fieldName must exist in t because introspect.FindProperties only returns fields in t")
			v = w.walk(field.Type, v)
			if info.Secret {
				v = ende.MakeSecret(v)
			}
			obj[resource.PropertyKey(info.Name)] = v
		}
		return resource.NewProperty(obj)
	// Collection types
	case reflect.Slice, reflect.Array:
		if !p.IsArray() {
			return p // p and t mismatch, so return early
		}
		arr := p.ArrayValue()
		for i, v := range arr {
			arr[i] = w.walk(t.Elem(), v)
		}
		return resource.NewProperty(arr)
	case reflect.Map:
		if !p.IsObject() {
			return p // p and t mismatch, so return early
		}
		m := p.ObjectValue()
		for k, v := range m {
			m[k] = w.walk(t.Elem(), v)
		}
		return resource.NewProperty(m)
	// Primitive types
	default:
		return p
	}

}
