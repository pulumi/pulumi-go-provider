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

package introspect

import (
	"fmt"
	"reflect"

	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

func NewAnnotator(resource any) Annotator {
	return Annotator{
		Descriptions:        map[string]string{},
		Defaults:            map[string]any{},
		DefaultEnvs:         map[string][]string{},
		DeprecationMessages: map[string]string{},
		matcher:             NewFieldMatcher(resource),
	}
}

// Annotator implements the Annotator interface as defined in resource/resource.go.
type Annotator struct {
	Descriptions        map[string]string
	Defaults            map[string]any
	DefaultEnvs         map[string][]string
	Token               string
	Aliases             []string
	DeprecationMessages map[string]string

	matcher FieldMatcher
}

func (a *Annotator) mustGetField(i any) FieldTag {
	field, ok, err := a.matcher.GetField(i)
	if err != nil {
		panic(fmt.Sprintf("getting field data: %s", err.Error()))
	}
	if !ok {
		panic("could not annotate field: could not find field")
	}
	return field
}

func (a *Annotator) Describe(i any, description string) {
	field, ok, err := a.matcher.GetField(i)
	if err != nil {
		panic(fmt.Sprintf("Could not parse field tags: %s", err.Error()))
	}
	if !ok {
		// We want the syntax for passing a pointer to a field and a pointer to the whole
		// struct to be the same:
		//
		// a.Describe(&v, "...")
		// a.Describe(&v.Field, "..")
		//
		// But the struct is already a pointer, so we check if we have the type **V, and
		// if so dereference once.

		typ := reflect.TypeOf(i)
		if typ.Kind() == reflect.Pointer && typ.Elem().Kind() == reflect.Pointer {
			i = reflect.ValueOf(i).Elem().Interface()
		}
		if a.matcher.value.Addr().Interface() == i {
			a.Descriptions[""] = description
			return
		}
		panic("Could not annotate field: could not find field")
	}
	a.Descriptions[field.Name] = description
}

// SetDefault annotates a struct field with a default value. The default value must be a
// primitive type in the pulumi type system.
func (a *Annotator) SetDefault(i any, defaultValue any, env ...string) {
	field := a.mustGetField(i)
	a.Defaults[field.Name] = defaultValue
	a.DefaultEnvs[field.Name] = append(a.DefaultEnvs[field.Name], env...)
}

func (a *Annotator) SetToken(module tokens.ModuleName, token tokens.TypeName) {
	a.Token = formatToken(module, token)
}

func (a *Annotator) AddAlias(module tokens.ModuleName, token tokens.TypeName) {
	a.Aliases = append(a.Aliases, formatToken(module, token))
}

func (a *Annotator) Deprecate(i any, message string) {
	field, ok, err := a.matcher.GetField(i)
	if err != nil {
		panic(fmt.Sprintf("Could not parse field tags: %s", err.Error()))
	}
	if !ok {
		// We want the syntax for passing a pointer to a field and a pointer to the whole
		// struct to be the same:
		//
		// a.Deprecate(&v, "...")
		// a.Deprecate(&v.Field, "..")
		//
		// But the struct is already a pointer, so we check if we have the type **V, and
		// if so dereference once.

		typ := reflect.TypeOf(i)
		if typ.Kind() == reflect.Pointer && typ.Elem().Kind() == reflect.Pointer {
			i = reflect.ValueOf(i).Elem().Interface()
		}
		if a.matcher.value.Addr().Interface() == i {
			a.DeprecationMessages[""] = message
			return
		}
		panic("Could not annotate field: could not find field")
	}
	a.DeprecationMessages[field.Name] = message
}

// formatToken formats a (module, token) pair into a valid token string.
//
// Panics when module or token are invalid.
func formatToken(module tokens.ModuleName, token tokens.TypeName) string {
	if !tokens.IsQName(module.String()) {
		panic(fmt.Sprintf("Module (%q) must comply with %s, but does not", module, tokens.QNameRegexp))
	}
	if !tokens.IsName(token.String()) {
		panic(fmt.Sprintf("Token (%q) must comply with %s, but does not", token, tokens.NameRegexp))
	}
	return tokens.NewTypeToken(tokens.NewModuleToken("pkg", module), token).String()
}
