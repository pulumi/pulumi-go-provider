package ende

import (
	"fmt"
	"reflect"

	"github.com/pulumi/pulumi-go-provider/internal/introspect"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	pmapper "github.com/pulumi/pulumi/sdk/v3/go/common/util/mapper"
)

type mapperOpts struct {
	IgnoreUnrecognized bool
	IgnoreMissing      bool
}

func decodeProperty(from resource.PropertyMap, to reflect.Value, opts mapperOpts) pmapper.MappingError {
	contract.Assertf(to.Kind() == reflect.Ptr && !to.IsNil() && to.Elem().CanSet(),
		"Target %v must be a non-nil, settable pointer", to.Type())
	toType := to.Type().Elem()
	contract.Assertf(toType.Kind() == reflect.Struct && !to.IsNil(),
		"Target %v must be a struct type with `pulumi:\"x\"` tags to direct decoding", toType)

	ctx := &mapCtx{ty: toType, opts: opts}

	ctx.decodeStruct(from, to.Elem())

	if len(ctx.errors) > 0 {
		return pmapper.NewMappingError(ctx.errors)
	}
	return nil
}

func (m *mapCtx) decodeStruct(from resource.PropertyMap, to reflect.Value) {
	addressed := map[string]struct{}{}
	for _, f := range reflect.VisibleFields(to.Type()) {
		tag, err := introspect.ParseTag(f)
		if err != nil {
			m.errors = append(m.errors,
				pmapper.NewFieldError(m.ty.String(), f.Name, err))
			continue
		}
		if tag.Internal {
			continue
		}
		value, ok := from[resource.PropertyKey(tag.Name)]
		if ok {
			addressed[tag.Name] = struct{}{}
			m.decodeValue(tag.Name, value, to.FieldByIndex(f.Index))
		} else if !m.opts.IgnoreMissing && !tag.Optional {
			m.errors = append(m.errors, pmapper.NewMissingError(m.ty, f.Name))
		}
	}

	if !m.opts.IgnoreUnrecognized {
		for k := range from {
			if _, ok := addressed[string(k)]; ok {
				continue
			}
			m.errors = append(m.errors, pmapper.NewUnrecognizedError(m.ty, string(k)))
		}
	}
}

func (m *mapCtx) decodePrimitive(fieldName string, from any, to reflect.Value) {
	elem := hydrateMaybePointer(to)
	fromV := reflect.ValueOf(from)
	if !fromV.CanConvert(elem.Type()) {
		m.errors = append(m.errors, pmapper.NewWrongTypeError(m.ty,
			fieldName, elem.Type(), fromV.Type()))
		return
	}
	elem.Set(fromV.Convert(elem.Type()))
}

type EnDePropertyValue interface {
	DecodeFromPropertyValue(string, resource.PropertyValue, func(resource.PropertyValue, reflect.Value))
	EncodeToPropertyValue(func(any) resource.PropertyValue) resource.PropertyValue

	// This method might be called on a zero value instance:
	//
	//	t = reflect.New(t).Elem().Interface().(ende.EnDePropertyValue).UnderlyingSchemaType()
	//
	UnderlyingSchemaType() reflect.Type
}

var EnDePropertyValueType = reflect.TypeOf((*EnDePropertyValue)(nil)).Elem()

func (m *mapCtx) decodeValue(fieldName string, from resource.PropertyValue, to reflect.Value) {
	if to := to.Addr(); to.CanInterface() && to.Type().Implements(EnDePropertyValueType) {
		to.Interface().(EnDePropertyValue).DecodeFromPropertyValue(fieldName, from,
			func(from resource.PropertyValue, to reflect.Value) {
				m.decodeValue(fieldName, from, to)
			})
		return
	}

	switch {
	// Primitives
	case from.IsBool():
		m.decodePrimitive(fieldName, from.BoolValue(), to)
	case from.IsNumber():
		m.decodePrimitive(fieldName, from.NumberValue(), to)
	case from.IsString():
		m.decodePrimitive(fieldName, from.StringValue(), to)

	// Collections
	case from.IsArray():
		arr := from.ArrayValue()
		elem := hydrateMaybePointer(to)
		if elem.Kind() != reflect.Slice {
			m.errors = append(m.errors, pmapper.NewWrongTypeError(m.ty,
				fieldName, elem.Type(), reflect.TypeOf(arr)))
		}
		length := len(arr)
		elem.Set(reflect.MakeSlice(elem.Type(), length, length))
		for i, v := range arr {
			m.decodeValue(fmt.Sprintf("%s[%d]", fieldName, i), v, elem.Index(i))
		}
	case from.IsObject():
		obj := from.ObjectValue()
		elem := hydrateMaybePointer(to)
		switch elem.Kind() {
		case reflect.Struct:
			m.decodeStruct(obj, elem)
		case reflect.Map:
			if key := elem.Type().Key(); key.Kind() != reflect.String {
				m.errors = append(m.errors, pmapper.NewWrongTypeError(m.ty,
					fieldName, reflect.TypeOf(""), key))
			}
			elem.Set(reflect.MakeMapWithSize(elem.Type(), len(obj)))
			for k, v := range obj {
				place := reflect.New(elem.Type().Elem()).Elem()
				m.decodeValue(fmt.Sprintf("%s[%s]", fieldName, string(k)), v, place)
				elem.SetMapIndex(reflect.ValueOf(string(k)), place)
			}
		default:
			m.errors = append(m.errors, pmapper.NewWrongTypeError(m.ty,
				fieldName, elem.Type(), reflect.TypeOf(obj)))
		}

	// Markers
	case from.IsSecret():
		m.decodeValue(fieldName, from.SecretValue().Element, to)
	case from.IsComputed():
		m.decodeValue(fieldName, from.OutputValue().Element, to)

	// Special values
	case from.IsAsset():
		panic("Unhandled property kind: Asset")
	case from.IsArchive():
		panic("Unhandled property kind: Archive")
	case from.IsNull():
		// No-op
	default:
		contract.Failf("Unknown property kind: %#v", from)
	}
}

func hydrateMaybePointer(to reflect.Value) reflect.Value {
	contract.Assertf(to.CanSet(), "must be able to set to hydrate")
	for to.Kind() == reflect.Ptr {
		if to.IsNil() {
			to.Set(reflect.New(to.Type().Elem()))
		}
		to = to.Elem()
	}
	return to
}

func encodeProperty(from any, opts mapperOpts) (resource.PropertyMap, pmapper.MappingError) {
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

	mapCtx := &mapCtx{ty: fromT, opts: opts}
	pMap := mapCtx.encodeStruct(fromV)
	if len(mapCtx.errors) > 0 {
		return nil, pmapper.NewMappingError(mapCtx.errors)
	}
	return pMap, nil
}

type mapCtx struct {
	opts   mapperOpts
	ty     reflect.Type
	errors []error
}

func (m *mapCtx) encodeStruct(from reflect.Value) resource.PropertyMap {
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
		if value.IsNull() && tag.Optional {
			continue
		}
		obj[key] = value
	}
	return obj
}

func (m *mapCtx) encodeMap(from reflect.Value) resource.PropertyMap {
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
	return wMap
}

func (m *mapCtx) encodeValue(from reflect.Value) resource.PropertyValue {
	for from.Kind() == reflect.Ptr {
		if from.IsNil() {
			return resource.NewNullProperty()
		}
		from = from.Elem()
	}

	if reflect.PtrTo(from.Type()).Implements(EnDePropertyValueType) {
		v := reflect.New(from.Type())
		v.Elem().Set(from)
		return v.Interface().(EnDePropertyValue).EncodeToPropertyValue(func(a any) resource.PropertyValue {
			return m.encodeValue(reflect.ValueOf(&a).Elem())
		})
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
		if from.IsNil() {
			return resource.NewNullProperty()
		}
		arr := make([]resource.PropertyValue, from.Len())
		for i := 0; i < from.Len(); i++ {
			arr[i] = m.encodeValue(from.Index(i))
		}
		return resource.NewArrayProperty(arr)
	case reflect.Struct:
		return resource.NewObjectProperty(m.encodeStruct(from))
	case reflect.Map:
		if from.IsNil() {
			return resource.NewNullProperty()
		}
		obj := m.encodeMap(from)
		return resource.NewObjectProperty(obj)
	case reflect.Interface:
		if from.IsNil() {
			return resource.NewNullProperty()
		}
		return m.encodeValue(from.Elem())
	default:
		panic("Unknown type: " + from.Type().String())
	}
}
