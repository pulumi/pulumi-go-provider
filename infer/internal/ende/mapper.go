package ende

import (
	"reflect"

	"github.com/pulumi/pulumi-go-provider/internal/introspect"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	pmapper "github.com/pulumi/pulumi/sdk/v3/go/common/util/mapper"
)

type mapper struct {
	IgnoreUnrecognized bool
	IgnoreMissing      bool
}

func (m mapper) decode(from resource.PropertyMap, to reflect.Value) pmapper.MappingError {
	return pmapper.New(&pmapper.Opts{
		IgnoreMissing:      m.IgnoreMissing,
		IgnoreUnrecognized: m.IgnoreUnrecognized,
	}).Decode(from.Mappable(), to.Addr().Interface())
}

func (m mapper) encode(from any) (resource.PropertyMap, pmapper.MappingError) {
	if from == nil {
		return nil, nil
	}

	fromV := reflect.ValueOf(from)
	fromT := fromV.Type()
	for fromT.Kind() == reflect.Ptr {
		fromT = fromT.Elem()
		fromV = fromV.Elem()
	}
	contract.Assertf(fromT.Kind() == reflect.Struct, "expect to encode a struct")

	mapCtx := &mapCtx{ty: fromT}
	pMap := mapCtx.encodeObj(fromV)
	if len(mapCtx.errors) > 0 {
		return nil, pmapper.NewMappingError(mapCtx.errors)
	}
	return pMap, nil
}

type mapCtx struct {
	ty     reflect.Type
	errors []error
}

func (m *mapCtx) encodeObj(from reflect.Value) resource.PropertyMap {
	visableFields := reflect.VisibleFields(from.Type())
	obj := make(resource.PropertyMap, len(visableFields))
	for _, f := range visableFields {
		tag, err := introspect.ParseTag(f)
		if err != nil {
			m.errors = append(m.errors,
				pmapper.NewFieldError(m.ty.String(), f.Name, err))
			continue
		}
		if tag.Internal {
			continue
		}
		key := resource.PropertyKey(tag.Name)
		value := m.encodeValue(from.FieldByIndex(f.Index))
		if value.IsNull() {
			continue
		}
		obj[key] = value
	}
	return obj
}

func (m *mapCtx) encodeValue(from reflect.Value) resource.PropertyValue {
	for from.Kind() == reflect.Ptr {
		if from.IsNil() {
			return resource.NewNullProperty()
		}
		from = from.Elem()
	}
	switch from.Kind() {
	case reflect.String:
		return resource.NewStringProperty(from.String())
	case reflect.Bool:
		return resource.NewBoolProperty(from.Bool())
	case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int,
		reflect.Float32, reflect.Float64:
		return resource.NewNumberProperty(from.Convert(reflect.TypeOf(float64(0))).Float())
	case reflect.Slice, reflect.Array:
		if from.Len() == 0 {
			return resource.NewNullProperty()
		}
		arr := make([]resource.PropertyValue, from.Len())
		for i := 0; i < from.Len(); i++ {
			arr[i] = m.encodeValue(from.Index(i))
		}
		return resource.NewArrayProperty(arr)
	case reflect.Struct:
		obj := m.encodeObj(from)
		return resource.NewObjectProperty(obj)
	case reflect.Map:
		wMap := make(resource.PropertyMap, from.Len())
		iter := from.MapRange()
		for iter.Next() {
			key := iter.Key()
			if key.Kind() != reflect.String {
				panic("unexpected key type")
			}
			value := m.encodeValue(iter.Value())
			if value.IsNull() {
				continue
			}
			wMap[resource.PropertyKey(key.String())] = value
		}
		if len(wMap) == 0 {
			return resource.NewNullProperty()
		}
		return resource.NewObjectProperty(wMap)
	case reflect.Interface:
		if from.IsNil() {
			return resource.NewNullProperty()
		}
		return m.encodeValue(from.Elem())
	default:
		panic("Unknown type: " + from.Type().String())
	}
}
