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

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

type MyEnum string

const (
	MyFoo MyEnum = "foo"
	MyBar MyEnum = "bar"
)

func (MyEnum) Values() []EnumValue[MyEnum] {
	return []EnumValue[MyEnum]{
		{
			Value:       MyFoo,
			Description: "The foo value",
		},
		{
			Value:       MyBar,
			Description: "The bar value",
		},
	}
}

type NotAnEnum bool

func TestIsEnum(t *testing.T) {
	t.Parallel()

	cases := []struct {
		typ    reflect.Type
		token  string // Leave "" to indicate not an enum
		values []EnumValue[any]
	}{
		{
			typ:   reflect.TypeOf(MyFoo),
			token: "pkg:infer:MyEnum",
			values: []EnumValue[any]{
				{
					Value:       string(MyFoo),
					Description: "The foo value",
				},
				{
					Value:       string(MyBar),
					Description: "The bar value",
				},
			},
		},
		{
			typ:   reflect.TypeOf(new(MyEnum)),
			token: "pkg:infer:MyEnum",
			values: []EnumValue[any]{
				{
					Value:       string(MyFoo),
					Description: "The foo value",
				},
				{
					Value:       string(MyBar),
					Description: "The bar value",
				},
			},
		},
		{
			typ: reflect.TypeOf(NotAnEnum(false)),
		},
	}
	for _, c := range cases {
		c := c
		t.Run(c.typ.Name(), func(t *testing.T) {
			enum, ok := isEnum(c.typ)
			if c.token == "" {
				assert.False(t, ok)
				return
			}
			assert.True(t, ok)
			assert.Equal(t, c.token, enum.token)
			assert.Equal(t, c.values, enum.values)

		})
	}
}
