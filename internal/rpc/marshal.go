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
		SkipNulls:        false,
		KeepUnknowns:     true,
		KeepResources:    true,
		KeepSecrets:      true,
		KeepOutputValues: true,
	})
	return resource.FromResourcePropertyValue(resource.NewProperty(rm)).AsMap(), err
}

// MarshalProperties marshals a PropertyMap into a structpb.Struct.
// This implementation is guaranteed to be lossless.
func MarshalProperties(m property.Map) (*structpb.Struct, error) {
	rm := resource.ToResourcePropertyValue(property.New(m)).ObjectValue()
	return plugin.MarshalProperties(rm, plugin.MarshalOptions{
		SkipNulls:        false,
		KeepUnknowns:     true,
		KeepSecrets:      true,
		KeepOutputValues: true,
		KeepResources:    true,
	})
}
