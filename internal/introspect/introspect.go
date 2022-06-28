package introspect

import (
	"reflect"

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
	mapper := mapper.New(&mapper.Opts{
		IgnoreMissing:      true,
		IgnoreUnrecognized: true,
	})

	inputs, err := mapper.Encode(r)
	if err != nil {
		return nil, err
	}
	s, nErr := structpb.NewStruct(inputs)
	if nErr != nil {
		return nil, nErr
	}
	return s, nil
}
