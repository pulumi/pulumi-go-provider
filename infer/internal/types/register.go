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
	"errors"
	"fmt"
	"reflect"

	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"

	"github.com/pulumi/pulumi-go-provider/internal/introspect"
	"github.com/pulumi/pulumi-go-provider/middleware/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

type Crawler func(
	t reflect.Type, isReference bool,
	fieldInfo *introspect.FieldTag,
	parent, field string,
) (drill bool, err error)

// crawlTypes recursively crawls T, calling the crawler on each new type it finds.
func crawlTypes[T any](crawler Crawler) error {
	var i T
	t := reflect.TypeOf(i)

	// Prohibit top-level "id" or "urn" fields.
	if t.Kind() == reflect.Struct {
		for _, f := range reflect.VisibleFields(t) {
			info, err := introspect.ParseTag(f)
			if err != nil {
				continue
			}
			if info.Name == "id" || info.Name == "urn" {
				return fmt.Errorf("%q is a reserved field name", info.Name)
			}
		}
	}

	// Drill will walk the types, calling crawl on types it finds.
	var drill func(reflect.Type, bool, *introspect.FieldTag) error
	drill = func(t reflect.Type, isReference bool, fieldInfo *introspect.FieldTag) error {
		nT, inputty, err := Underlying(t)
		if err != nil {
			return err
		}
		if inputty {
			t = nT
		}
		switch t.Kind() {
		case reflect.String, reflect.Float64, reflect.Int, reflect.Bool:
			// Primitive types could be enums
			_, err := crawler(t, isReference, fieldInfo, "", "")
			return err
		case reflect.Pointer, reflect.Array, reflect.Map, reflect.Slice:
			// Holds a reference to other types
			return drill(t.Elem(), false, fieldInfo)
		case reflect.Struct:
			var errs []error
		field:
			for _, f := range reflect.VisibleFields(t) {
				info, err := introspect.ParseTag(f)
				if err != nil {
					return err
				}
				// The type is internal or it is a reference to an external package
				if info.Internal || (info.ExplicitRef != nil && info.ExplicitRef.Pkg != "") {
					continue
				}

				fieldIsReference := false

				typ := f.Type
				for done := false; !done; {
					switch typ.Kind() {
					case reflect.Pointer, reflect.Array, reflect.Map, reflect.Slice:
						// Could hold a reference to other types
						typ = typ.Elem()
						fieldIsReference = true
					default:
						nT, inputty, err := Underlying(typ)
						if err != nil {
							errs = append(errs, err)
							continue field
						}
						if inputty {
							typ = nT
						} else {
							done = true
						}
					}
				}
				further, err := crawler(typ, fieldIsReference, &info, t.String(), f.Name)
				if err != nil {
					errs = append(errs, err)
					continue field
				}
				if further {
					err = drill(typ, fieldIsReference, &info)
					if err != nil {
						errs = append(errs, err)
						continue field
					}
				}
			}
			return errors.Join(errs...)
		default:
			return nil
		}
	}
	return drill(t, false, nil)
}

// registerTypes recursively examines fields of T, calling reg on the schematized type when appropriate.
func Register[T any](reg schema.RegisterDerivativeType) error {
	crawler := func(
		t reflect.Type, isReference bool, info *introspect.FieldTag,
		parent, field string,
	) (bool, error) {
		if nT, inputty, err := Underlying(t); err != nil {
			return false, err
		} else if inputty {
			t = nT
		}
		if t == reflect.TypeOf(resource.Asset{}) || t == reflect.TypeOf(resource.Archive{}) {
			return false, nil
		}
		if enum, ok := isEnum(t); ok {
			if info != nil && info.Optional && !isReference {
				return false, optionalNeedsPointerError{
					ParentStruct: parent,
					PropertyName: field,
					Kind:         "enum",
				}
			}

			tSpec := pschema.ComplexTypeSpec{}
			for _, v := range enum.values {
				tSpec.Enum = append(tSpec.Enum, pschema.EnumValueSpec{
					Name:        "",
					Description: v.Description,
					Value:       v.Value,
				})
			}
			tSpec.Type = schemaNameForType(t.Kind())
			// We never need to recurse into primitive types
			_ = reg(tokens.Type(enum.token), tSpec)
			return false, nil
		}
		if _, ok, err := ResourceReferenceToken(t, nil, true); ok {
			// This will have already been registered, so we don't need to recurse here
			return false, err
		}
		if t.Kind() == reflect.Struct {
			spec, err := ObjectSchema(t)
			if err != nil {
				return false, err
			}

			tk, err := GetTokenOf(t, nil)
			if err != nil {
				return false, err
			}

			if info != nil && info.Optional && !isReference {
				return false, optionalNeedsPointerError{
					ParentStruct: parent,
					PropertyName: field,
					Kind:         t.Kind().String(),
				}
			}

			return reg(tk, pschema.ComplexTypeSpec{ObjectTypeSpec: *spec}), nil
		}
		return true, nil
	}
	return crawlTypes[T](crawler)
}

type optionalNeedsPointerError struct {
	ParentStruct string
	PropertyName string
	Kind         string
}

func (err optionalNeedsPointerError) Error() string {
	const msg string = "%s.%s: cannot specify a optional %s without pointer indirection"
	return fmt.Sprintf(msg, err.ParentStruct, err.PropertyName, err.Kind)
}
