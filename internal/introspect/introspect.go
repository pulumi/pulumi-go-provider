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

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/mapper"
	"google.golang.org/protobuf/types/known/structpb"
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
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if !field.IsExported() {
			continue
		}

		tag, has := field.Tag.Lookup("pulumi")
		if !has {
			continue
		}

		m[tag] = value.Field(i).Interface()
	}
	return m
}

type ToPropertiesOptions struct {
	ComputedKeys []string
}

func ResourceToProperties(r any, opts *ToPropertiesOptions) (*structpb.Struct, error) {
	if opts == nil {
		opts = &ToPropertiesOptions{}
	}
	mapper := mapper.New(
		&mapper.Opts{IgnoreMissing: true, IgnoreUnrecognized: true},
	)

	props, err := mapper.Encode(r)
	if err != nil {
		return nil, err
	}

	propsMap := resource.NewPropertyMapFromMap(props)

	for _, computed := range opts.ComputedKeys {
		propsMap[resource.PropertyKey(computed)] = resource.MakeComputed(resource.NewStringProperty(""))
	}

	return plugin.MarshalProperties(propsMap, plugin.MarshalOptions{
		KeepUnknowns: true,
		SkipNulls:    true,
	})
}

func PropertiesToResource(s *structpb.Struct, res any) error {
	inputProps, err := plugin.UnmarshalProperties(s, plugin.MarshalOptions{
		SkipNulls:        true,
		SkipInternalKeys: true,
	})
	if err != nil {
		return err
	}
	inputs := inputProps.Mappable()

	return mapper.MapI(inputs, res)
}

func FindProperties(r any) (map[string]FieldTag, error) {
	typ := reflect.TypeOf(r)
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	contract.Assertf(typ.Kind() == reflect.Struct, "Expected struct, found %s (%T)", typ.Kind(), r)
	m := map[string]FieldTag{}
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
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

func FindOutputProperties(r any) (map[string]bool, error) {
	props, err := FindProperties(r)
	if err != nil {
		return nil, err
	}
	m := map[string]bool{}
	for k, v := range props {
		if v.Output {
			m[k] = true
		}
	}

	return m, nil
}

// Get the token that represents a struct.
func GetToken(pkg tokens.Package, i any) (tokens.Type, error) {
	typ := reflect.TypeOf(i)
	if typ == nil {
		return "", fmt.Errorf("Cannot get token of nil type")
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
		return "", fmt.Errorf("Type %T has no name", i)
	}
	if mod == "" {
		return "", fmt.Errorf("Type %T has no module path", i)
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
	if !hasPulumiTag {
		return FieldTag{Internal: true}, nil
	}

	pulumi := map[string]bool{}
	pulumiArray := strings.Split(pulumiTag, ",")
	name := pulumiArray[0]
	for _, item := range pulumiArray[1:] {
		pulumi[item] = true
	}

	provider := map[string]bool{}
	providerArray := strings.Split(providerTag, ",")
	if hasProviderTag {
		for _, item := range providerArray {
			provider[item] = true
		}
	}

	return FieldTag{
		Name:             name,
		Optional:         pulumi["optional"],
		Output:           provider["output"],
		Secret:           provider["secret"],
		ReplaceOnChanges: provider["replaceOnChanges"],
	}, nil
}

type FieldTag struct {
	Name     string // The name of the field in the Pulumi type system.
	Optional bool   // If the field is optional in the Pulumi type system.
	Internal bool   // If the field should exist in the Pulumi type system.
	Output   bool   // If the field is an output type in the pulumi type system.
	Secret   bool   // If the field is secret.
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
	for i := 0; i < hostType.NumField(); i++ {
		f := f.value.Field(i)
		fType := hostType.Field(i)
		if f.Addr().Interface() == field {
			f, error := ParseTag(fType)
			return f, true, error
		}
	}
	return FieldTag{}, false, nil
}
