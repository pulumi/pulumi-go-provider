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

package infer

// The set of allowed enum underlying values.
type EnumKind interface {
	~string | ~float64 | ~bool | ~int
}

// An Enum in the Pulumi type system.
type Enum[T EnumKind] interface {
	// A list of all allowed values for the enum.
	Values() []EnumValue[T]
}

// An EnumValue represents an allowed value for an Enum.
type EnumValue[T any] struct {
	Name        string
	Value       T
	Description string
}

// A non-generic marker to determine that an enum value has been found.
type isEnumValue interface {
	isEnumValue()
}

func (EnumValue[T]) isEnumValue() {}
