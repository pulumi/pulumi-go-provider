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
	"errors"
	"reflect"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	"github.com/pulumi/pulumi-go-provider/internal/introspect"
	"github.com/pulumi/pulumi-go-provider/internal/putil"
)

func applySecrets[I any](inputs resource.PropertyMap) resource.PropertyMap {
	var walker secretsWalker
	result := walker.walk(typeFor[I](), resource.NewProperty(inputs))
	contract.AssertNoErrorf(errors.Join(walker.errs...),
		`secretsWalker only produces errors when the type it walks has invalid property tags
I can't have invalid property tags because we have gotten to runtime, and it would have failed at
schema generation time already.`)
	return result.ObjectValue()
}

// The object that controls secrets application.
type secretsWalker struct{ errs []error }

func (w *secretsWalker) walk(t reflect.Type, p resource.PropertyValue) (out resource.PropertyValue) {
	// If t is nil, we have no type information, so return.
	if t == nil {
		return p
	}

	// Ensure we are working in raw value types for p

	if putil.IsSecret(p) {
		p = putil.MakePublic(p)
		defer func() { out = putil.MakeSecret(p) }()
	}

	if putil.IsComputed(p) {
		p = putil.MakeKnown(p)
		defer func() { out = putil.MakeComputed(p) }()
	}

	// Ensure we are working in raw value types for t

	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	// Here is where we attempt to apply secrets from type information.
	//
	// If the shape of p does not match the type of t, we will simply return
	// p. Because secrets are applied in Check (which may have failed), we can't
	// assume that p conforms to the type of t.

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

			if info.Internal {
				continue
			}

			v, ok := obj[resource.PropertyKey(info.Name)]
			if !ok {
				// Since info.Name is missing from obj, we don't need to
				// worry about if field should be secret.
				continue
			}
			v = w.walk(field.Type, v)
			if info.Secret {
				v = putil.MakeSecret(v)
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
	// Primitive types can't have tags, so there's nothing to apply here
	default:
		return p
	}

}
