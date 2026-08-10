// Copyright 2025, Pulumi Corporation.
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

// Package rpc provides utilities for marshaling and unmarshaling of resource properties.
package rpc

import (
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"google.golang.org/protobuf/types/known/structpb"
)

// UnmarshalProperties unmarshals a structpb.Struct into a PropertyMap.
// This implementation is guaranteed to be lossless.
func UnmarshalProperties(s *structpb.Struct) (property.Map, error) {
	rm, err := plugin.UnmarshalProperties(s, plugin.MarshalOptions{
		KeepUnknowns:     true,
		KeepResources:    true,
		KeepSecrets:      true,
		KeepOutputValues: true,
	})
	return resource.FromResourcePropertyValue(resource.NewProperty(rm)).AsMap(), err
}

// UnmarshalPropertyValue unmarshals a single structpb.Value into a
// property.Value; a nil input unmarshals to the null value. This implementation
// is guaranteed to be lossless.
func UnmarshalPropertyValue(v *structpb.Value) (property.Value, error) {
	if v == nil {
		return property.Value{}, nil
	}
	rv, err := plugin.UnmarshalPropertyValue("", v, plugin.MarshalOptions{
		KeepUnknowns:     true,
		KeepResources:    true,
		KeepSecrets:      true,
		KeepOutputValues: true,
	})
	if err != nil || rv == nil {
		return property.Value{}, err
	}
	return resource.FromResourcePropertyValue(*rv), nil
}

// MarshalPropertyValue marshals a single property.Value into a structpb.Value;
// the null value marshals to nil, mirroring UnmarshalPropertyValue. This
// implementation is guaranteed to be lossless.
func MarshalPropertyValue(v property.Value) (*structpb.Value, error) {
	if v.IsNull() {
		return nil, nil
	}
	return plugin.MarshalPropertyValue("", resource.ToResourcePropertyValue(v), plugin.MarshalOptions{
		KeepUnknowns:     true,
		KeepSecrets:      true,
		KeepOutputValues: true,
		KeepResources:    true,
		KeepByteString:   true,
	})
}

// MarshalProperties marshals a PropertyMap into a structpb.Struct.
// This implementation is guaranteed to be lossless.
func MarshalProperties(m property.Map) (*structpb.Struct, error) {
	rm := resource.ToResourcePropertyValue(property.New(m)).ObjectValue()
	return plugin.MarshalProperties(rm, plugin.MarshalOptions{
		KeepUnknowns:     true,
		KeepSecrets:      true,
		KeepOutputValues: true,
		KeepResources:    true,
		KeepByteString:   true,
	})
}
