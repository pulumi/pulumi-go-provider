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

	"github.com/pulumi/pulumi-go-provider/infer/annotate"
	"github.com/pulumi/pulumi-go-provider/internal/introspect"
)

func GetAnnotated(t reflect.Type) introspect.Annotator {
	// If we have type *R with value(i) = nil, NewAnnotator will fail. We need to get
	// value(i) = *R{}, so we reinflate the underlying value
	for t.Kind() == reflect.Pointer && t.Elem().Kind() == reflect.Pointer {
		t = t.Elem()
	}
	i := reflect.New(t).Elem()
	if i.Kind() == reflect.Pointer && i.IsNil() {
		i = reflect.New(i.Type().Elem())
	}

	if i.Kind() != reflect.Pointer {
		v := reflect.New(i.Type())
		v.Elem().Set(i)
		i = v
	}
	t = i.Type()

	merge := func(dst *introspect.Annotator, src introspect.Annotator) {
		for k, v := range src.Descriptions {
			(*dst).Descriptions[k] = v
		}
		for k, v := range src.Defaults {
			(*dst).Defaults[k] = v
		}
		for k, v := range src.DefaultEnvs {
			(*dst).DefaultEnvs[k] = v
		}
		dst.Token = src.Token
	}

	ret := introspect.Annotator{
		Descriptions: map[string]string{},
		Defaults:     map[string]any{},
		DefaultEnvs:  map[string][]string{},
	}
	if t.Elem().Kind() == reflect.Struct {
		for _, f := range reflect.VisibleFields(t.Elem()) {
			if f.Anonymous && f.IsExported() {
				r := GetAnnotated(f.Type)
				merge(&ret, r)
			}
		}
	}

	if r, ok := i.Interface().(annotate.Annotated); ok {
		a := introspect.NewAnnotator(r)
		r.Annotate(&a)
		merge(&ret, a)
	}

	return ret
}
