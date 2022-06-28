package introspect

import (
	"reflect"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
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

func ResourceToProperties(r any) (*structpb.Struct, error) {
	mapper := mapper.New(
		&mapper.Opts{IgnoreMissing: true, IgnoreUnrecognized: true},
	)

	props, err := mapper.Encode(r)
	if err != nil {
		return nil, err
	}

	return plugin.MarshalProperties(resource.NewPropertyMapFromMap(props), plugin.MarshalOptions{
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

func FindOutputProperties(r any) map[string]bool {
	typ := reflect.TypeOf(r)
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	contract.Assertf(typ.Kind() == reflect.Struct, "Expected struct, found %s (%T)", typ.Kind(), r)
	m := map[string]bool{}
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		tag, ok := f.Tag.Lookup("provider")
		if !ok {
			continue
		}
		name := f.Name
		pulumiTag, ok := f.Tag.Lookup("pulumi")
		if ok {
			name = strings.Split(pulumiTag, ",")[0]
		}
		for _, c := range strings.Split(tag, ",") {
			if c == "output" {
				m[name] = true
			}
		}
	}
	return m
}
