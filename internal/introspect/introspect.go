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
	"runtime"
	"strings"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

func StructToMap(i any) map[string]interface{} {
	typ := reflect.TypeOf(i)
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}

	contract.Assertf(typ.Kind() == reflect.Struct, "Expected a struct. Instead got %s (%v)", typ.Kind(), i)

	m := map[string]interface{}{}
	value := reflect.ValueOf(i)
	for value.Type().Kind() == reflect.Pointer {
		value = value.Elem()
	}
	for _, field := range reflect.VisibleFields(typ) {
		if !field.IsExported() {
			continue
		}

		tag, has := field.Tag.Lookup("pulumi")
		if !has {
			continue
		}

		m[tag] = value.FieldByIndex(field.Index).Interface()
	}
	return m
}

type ToPropertiesOptions struct {
	ComputedKeys []string
}

func FindProperties(r any) (map[string]FieldTag, error) {
	typ := reflect.TypeOf(r)
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	contract.Assertf(typ.Kind() == reflect.Struct, "Expected struct, found %s (%T)", typ.Kind(), r)
	m := map[string]FieldTag{}
	for _, f := range reflect.VisibleFields(typ) {
		info, err := ParseTag(f)
		if err != nil {
			return nil, err
		}
		if info.Internal {
			continue
		}
		m[info.Name] = info
	}
	return m, nil
}

// Get the token that represents a struct.
func GetToken(pkg tokens.Package, i any) (tokens.Type, error) {
	typ := reflect.TypeOf(i)
	if typ == nil {
		return "", fmt.Errorf("cannot get token of nil type")
	}

	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}

	var name string
	var mod string
	if typ.Kind() == reflect.Func {
		fn := runtime.FuncForPC(reflect.ValueOf(i).Pointer())
		parts := strings.Split(fn.Name(), ".")
		name = parts[len(parts)-1]
		mod = strings.Join(parts[:len(parts)-1], "/")
	} else {
		name = typ.Name()
		mod = strings.Trim(typ.PkgPath(), "*")
	}

	if name == "" {
		return "", fmt.Errorf("type %T has no name", i)
	}
	if mod == "" {
		return "", fmt.Errorf("type %T has no module path", i)
	}
	// Take off the pkg name, since that is supplied by `pkg`.
	mod = mod[strings.LastIndex(mod, "/")+1:]
	if mod == "main" {
		mod = "index"
	}
	m := tokens.NewModuleToken(pkg, tokens.ModuleName(mod))
	tk := tokens.NewTypeToken(m, tokens.TypeName(name))
	return tk, nil
}

// ParseTag gets tag information out of struct tags. It looks under the `pulumi` and
// `provider` tag namespaces.
func ParseTag(field reflect.StructField) (FieldTag, error) {
	pulumiTag, hasPulumiTag := field.Tag.Lookup("pulumi")
	providerTag, hasProviderTag := field.Tag.Lookup("provider")
	if hasProviderTag && !hasPulumiTag {
		return FieldTag{}, fmt.Errorf("you must put to the `pulumi` tag to use the `provider` tag")
	}
	if !hasPulumiTag || !field.IsExported() {
		return FieldTag{Internal: true}, nil
	}

	pulumi := map[string]bool{}
	pulumiArray := strings.Split(pulumiTag, ",")
	name := pulumiArray[0]
	for _, item := range pulumiArray[1:] {
		pulumi[item] = true
	}

	var explRef *ExplicitType
	provider := map[string]bool{}
	providerArray := strings.Split(providerTag, ",")
	if hasProviderTag {
		for _, item := range providerArray {
			if strings.HasPrefix(item, "type=") {
				const typeErrMsg = `expected "type=" value of "[pkg@version:]module:name", found "%s"`
				extType := strings.TrimPrefix(item, "type=")
				parts := strings.Split(extType, ":")
				switch len(parts) {
				case 2:
					explRef = &ExplicitType{
						Module: parts[0],
						Name:   parts[1],
					}
				case 3:
					external := strings.Split(parts[0], "@")
					if len(external) != 2 {
						return FieldTag{}, fmt.Errorf(typeErrMsg, extType)
					}
					s, err := semver.ParseTolerant(external[1])
					if err != nil {
						return FieldTag{}, fmt.Errorf(`"type=" version must be valid semver: %w`, err)
					}
					explRef = &ExplicitType{
						Pkg:     external[0],
						Version: "v" + s.String(),
						Module:  parts[1],
						Name:    parts[2],
					}
				default:
					return FieldTag{}, fmt.Errorf(typeErrMsg, extType)
				}
				continue
			}
			provider[item] = true
		}
	}

	return FieldTag{
		Name:             name,
		Optional:         pulumi["optional"],
		Secret:           provider["secret"],
		ReplaceOnChanges: provider["replaceOnChanges"],
		ExplicitRef:      explRef,
	}, nil
}

// An explicitly specified type ref token.
type ExplicitType struct {
	Pkg     string
	Version string
	Module  string
	Name    string
}

type FieldTag struct {
	Name        string        // The name of the field in the Pulumi type system.
	Optional    bool          // If the field is optional in the Pulumi type system.
	Internal    bool          // If the field should exist in the Pulumi type system.
	Secret      bool          // If the field is secret.
	ExplicitRef *ExplicitType // The name and version of the external type consumed in the field.
	// NOTE: ReplaceOnChanges will only be obeyed when the default diff implementation is used.
	ReplaceOnChanges bool // If changes in the field should force a replacement.
}

func NewFieldMatcher(i any) FieldMatcher {
	v := reflect.ValueOf(i)
	for v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	contract.Assertf(v.Kind() == reflect.Struct, "FieldMatcher must contain a struct, found a %s.", v.Type())
	return FieldMatcher{
		value: v,
	}
}

type FieldMatcher struct {
	value reflect.Value
}

func (f *FieldMatcher) GetField(field any) (FieldTag, bool, error) {
	hostType := f.value.Type()
	for _, i := range reflect.VisibleFields(hostType) {
		f := f.value.FieldByIndex(i.Index)
		fType := hostType.FieldByIndex(i.Index)
		if !fType.IsExported() {
			continue
		}
		if f.Addr().Interface() == field {
			f, error := ParseTag(fType)
			return f, true, error
		}
	}
	return FieldTag{}, false, nil
}
