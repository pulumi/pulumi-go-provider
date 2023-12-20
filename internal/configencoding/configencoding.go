package configencoding

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"google.golang.org/protobuf/types/known/structpb"
)

type configEncoding struct {
	schema schema.ConfigSpec
}

func New(s schema.ConfigSpec) *configEncoding {
	return &configEncoding{schema: s}
}

func (*configEncoding) tryUnwrapSecret(encoded any) (any, bool) {
	m, ok := encoded.(map[string]any)
	if !ok {
		return nil, false
	}
	sig, ok := m["4dabf18193072939515e22adb298388d"]
	if !ok {
		return nil, false
	}
	ss, ok := sig.(string)
	if !ok {
		return nil, false
	}
	if ss != "1b47061264138c4ac30d75fd1eb44270" {
		return nil, false
	}
	value, ok := m["value"]
	return value, ok
}

func (enc *configEncoding) convertStringToPropertyValue(s string, prop schema.PropertySpec) (resource.PropertyValue, error) {
	// If the schema expects a string, we can just return this as-is.
	if prop.Type == "string" {
		return resource.NewStringProperty(s), nil
	}

	// Otherwise, we will attempt to deserialize the input string as JSON and convert the result into a Pulumi
	// property. If the input string is empty, we will return an appropriate zero value.
	if s == "" {
		return enc.zeroValue(prop.Type), nil
	}

	var jsonValue interface{}
	if err := json.Unmarshal([]byte(s), &jsonValue); err != nil {
		return resource.PropertyValue{}, err
	}

	opts := enc.unmarshalOpts()

	// Instead of using resource.NewPropertyValue, specialize it to detect nested json-encoded secrets.
	var replv func(encoded any) (resource.PropertyValue, bool)
	replv = func(encoded any) (resource.PropertyValue, bool) {
		encodedSecret, isSecret := enc.tryUnwrapSecret(encoded)
		if !isSecret {
			return resource.NewNullProperty(), false
		}

		v := resource.NewPropertyValueRepl(encodedSecret, nil, replv)
		if opts.KeepSecrets {
			v = resource.MakeSecret(v)
		}

		return v, true
	}

	return resource.NewPropertyValueRepl(jsonValue, nil, replv), nil
}

func (*configEncoding) zeroValue(typ string) resource.PropertyValue {
	switch typ {
	case "bool":
		return resource.NewPropertyValue(false)
	case "int", "float":
		return resource.NewPropertyValue(0)
	case "list", "set":
		return resource.NewPropertyValue([]interface{}{})
	default:
		return resource.NewPropertyValue(map[string]interface{}{})
	}
}

func (enc *configEncoding) unmarshalOpts() plugin.MarshalOptions {
	return plugin.MarshalOptions{
		Label:        "config",
		KeepUnknowns: true,
		SkipNulls:    true,
		RejectAssets: true,
	}
}

// Like plugin.UnmarshalPropertyValue but overrides string parsing with convertStringToPropertyValue.
func (enc *configEncoding) unmarshalPropertyValue(key resource.PropertyKey,
	v *structpb.Value,
) (*resource.PropertyValue, error) {
	opts := enc.unmarshalOpts()

	pv, err := plugin.UnmarshalPropertyValue(key, v, enc.unmarshalOpts())
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling property %q: %w", key, err)
	}

	prop, ok := enc.schema.Variables[string(key)]

	// Only apply JSON-encoded recognition for known fields.
	if !ok {
		return pv, nil
	}

	var jsonString string
	var jsonStringDetected, jsonStringSecret bool

	if pv.IsString() {
		jsonString = pv.StringValue()
		jsonStringDetected = true
	}

	if opts.KeepSecrets && pv.IsSecret() && pv.SecretValue().Element.IsString() {
		jsonString = pv.SecretValue().Element.StringValue()
		jsonStringDetected = true
		jsonStringSecret = true
	}

	if jsonStringDetected {
		v, err := enc.convertStringToPropertyValue(jsonString, prop)
		if err != nil {
			return nil, fmt.Errorf("error unmarshalling property %q: %w", key, err)
		}
		if jsonStringSecret {
			s := resource.MakeSecret(v)
			return &s, nil
		}
		return &v, nil
	}

	// Computed sentinels are coming in as always having an empty string, but the encoding coerses them to a zero
	// value of the appropriate type.
	if pv.IsComputed() {
		el := pv.V.(resource.Computed).Element
		if el.IsString() && el.StringValue() == "" {
			res := resource.MakeComputed(enc.zeroValue(prop.Type))
			return &res, nil
		}
	}

	return pv, nil
}

// Inline from plugin.UnmarshalProperties substituting plugin.UnmarshalPropertyValue.
func (enc *configEncoding) UnmarshalProperties(props *structpb.Struct) (resource.PropertyMap, error) {
	opts := enc.unmarshalOpts()

	result := make(resource.PropertyMap)

	// First sort the keys so we enumerate them in order (in case errors happen, we want determinism).
	var keys []string
	if props != nil {
		for k := range props.Fields {
			keys = append(keys, k)
		}
		sort.Strings(keys)
	}

	// And now unmarshal every field it into the map.
	for _, key := range keys {
		pk := resource.PropertyKey(key)
		v, err := enc.unmarshalPropertyValue(pk, props.Fields[key])
		if err != nil {
			return nil, err
		} else if v != nil {
			if opts.SkipNulls && v.IsNull() {
				continue
			}
			if opts.SkipInternalKeys && resource.IsInternalPropertyKey(pk) {
				continue
			}
			result[pk] = *v
		}
	}

	return result, nil
}

// Inverse of UnmarshalProperties, with additional support for secrets. Since the encoding cannot represent nested
// secrets, any nested secrets will be approximated by making the entire top-level property secret.
// func (enc *ConfigEncoding) MarshalProperties(props resource.PropertyMap) (*structpb.Struct, error) {
// 	opts := plugin.MarshalOptions{
// 		Label:        "config",
// 		KeepUnknowns: true,
// 		SkipNulls:    true,
// 		RejectAssets: true,
// 		KeepSecrets:  true,
// 	}

// 	copy := make(resource.PropertyMap)
// 	for k, v := range props {
// 		var err error
// 		copy[k], err = enc.jsonEncodePropertyValue(k, v)
// 		if err != nil {
// 			return nil, err
// 		}
// 	}
// 	return plugin.MarshalProperties(copy, opts)
// }

// func (enc *ConfigEncoding) jsonEncodePropertyValue(k resource.PropertyKey,
// 	v resource.PropertyValue,
// ) (resource.PropertyValue, error) {
// 	if v.ContainsUnknowns() {
// 		return resource.NewStringProperty(plugin.UnknownStringValue), nil
// 	}
// 	if v.ContainsSecrets() {
// 		encoded, err := enc.jsonEncodePropertyValue(k, propertyvalue.RemoveSecrets(v))
// 		if err != nil {
// 			return v, err
// 		}
// 		return resource.MakeSecret(encoded), err
// 	}
// 	_, knownKey := enc.schema.Variables[string(k)]
// 	switch {
// 	case knownKey && v.IsNull():
// 		return resource.NewStringProperty(""), nil
// 	case knownKey && !v.IsNull() && !v.IsString():
// 		encoded, err := json.Marshal(v.Mappable())
// 		if err != nil {
// 			return v, fmt.Errorf("JSON encoding error while marshalling property %q: %w", k, err)
// 		}
// 		return resource.NewStringProperty(string(encoded)), nil
// 	default:
// 		return v, nil
// 	}
// }
