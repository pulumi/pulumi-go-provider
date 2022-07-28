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

type EnumByRef float64

const (
	PiRef EnumByRef = 3.1415
)

func (*EnumByRef) Values() []EnumValue[EnumByRef] {
	return []EnumValue[EnumByRef]{
		{
			Value:       PiRef,
			Description: "approximate of PI",
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
			typ: reflect.TypeOf(NotAnEnum(false)),
		},
		{
			typ:   reflect.TypeOf(PiRef),
			token: "pkg:infer:EnumByRef",
			values: []EnumValue[any]{
				{
					Value:       float64(PiRef),
					Description: "approximate of PI",
				},
			},
		},
		{
			typ:   reflect.TypeOf(new(**EnumByRef)),
			token: "pkg:infer:EnumByRef",
			values: []EnumValue[any]{
				{
					Value:       float64(PiRef),
					Description: "approximate of PI",
				},
			},
		},
	}
	for _, c := range cases {
		for _, ptr := range []bool{true, false} {
			c := c
			if ptr {
				c.typ = reflect.PointerTo(c.typ)
			}
			t.Run(c.typ.String(), func(t *testing.T) {
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
}
