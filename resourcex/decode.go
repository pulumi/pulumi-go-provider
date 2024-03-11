// Copyright 2016-2024, Pulumi Corporation.
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

package resourcex

import (
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// Decode decodes a property map into a JSON-like structure containing only values.
// Unknown values are decoded as nil, both in maps and arrays.
// Secrets are collapsed into their underlying values.
func Decode(props resource.PropertyMap) map[string]any {
	return decodeM(props)
}

// DecodeValue decodes a property value into its underlying value, recursively.
// Unknown values are decoded as nil, also in maps and arrays.
// Secrets are collapsed into their underlying values.
func DecodeValue(prop resource.PropertyValue) any {
	return decodeV(prop)
}

// decodeM returns a mapper-compatible object map, suitable for deserialization into structures.
func decodeM(props resource.PropertyMap) map[string]any {
	obj := make(map[string]any)
	for _, k := range props.StableKeys() {
		key := string(k)
		obj[key] = decodeV(props[k])
	}
	return obj
}

// decodeV returns a mapper-compatible object map, suitable for deserialization into structures.
func decodeV(v resource.PropertyValue) any {
	switch {
	case v.IsNull():
		return nil
	case v.IsBool():
		return v.BoolValue()
	case v.IsNumber():
		return v.NumberValue()
	case v.IsString():
		return v.StringValue()
	case v.IsArray():
		arr := make([]any, len(v.ArrayValue()))
		for i := 0; i < len(v.ArrayValue()); i++ {
			arr[i] = decodeV(v.ArrayValue()[i])
		}
		return arr
	case v.IsAsset():
		return decodeAsset(v.AssetValue())
	case v.IsArchive():
		contract.Failf("unsupported value type '%v'", v.TypeString())
		return nil
	case v.IsComputed():
		return nil // zero value for unknowns
	case v.IsOutput():
		if !v.OutputValue().Known {
			return nil // zero value for unknowns
		}
		return decodeV(v.OutputValue().Element)
	case v.IsSecret():
		return decodeV(v.SecretValue().Element)
	case v.IsResourceReference():
		contract.Failf("unsupported value type '%v'", v.TypeString())
		return nil
	case v.IsObject():
		return decodeM(v.ObjectValue())
	default:
		contract.Failf("unexpected value type '%v'", v.TypeString())
		return nil
	}
}

func decodeAsset(a *resource.Asset) any {
	if a == nil {
		return nil
	}
	result := map[string]any{
		resource.SigKey: resource.AssetSig,
	}
	if a.Hash != "" {
		result[resource.AssetHashProperty] = a.Hash
	}
	if a.Text != "" {
		result[resource.AssetTextProperty] = a.Text
	}
	if a.Path != "" {
		result[resource.AssetPathProperty] = a.Path
	}
	if a.URI != "" {
		result[resource.AssetURIProperty] = a.URI
	}
	return result
}
