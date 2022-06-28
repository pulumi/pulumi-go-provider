package introspect

import (
	"reflect"
	"unicode"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/mapper"
	"google.golang.org/protobuf/types/known/structpb"
)

func camelCase(name string) string {
	runes := []rune(name)
	return string(append([]rune{unicode.ToLower(rune(name[0]))}, runes[1:]...))
}

func StructToMap(i any) map[string]interface{} {
	typ := reflect.TypeOf(i)
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}

	contract.Assertf(typ.Kind() != reflect.Struct, "You need to give a struct")

	m := map[string]interface{}{}
	value := reflect.ValueOf(i)
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		m[camelCase(field.Name)] = value.Field(i).Interface()
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
