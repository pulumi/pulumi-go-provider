// Copyright 2023, Pulumi Corporation.
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

	"github.com/pulumi/pulumi-go-provider/middleware/schema"
	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
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
			Name:        "foo",
			Value:       MyFoo,
			Description: "The foo value",
		},
		{
			Name:        "bar",
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
					Name:        "foo",
					Value:       string(MyFoo),
					Description: "The foo value",
				},
				{
					Name:        "bar",
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
				t.Parallel()
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

type Foo struct {
	Bar      *Bar   `pulumi:"bar"`
	Enum     MyEnum `pulumi:"enum"`
	Literal  string
	External External `pulumi:"external,optional" provider:"type=example@0.1.2:mod:Internal"`
}

// This type should never show up in the schema
type Internal struct{}

type External struct {
	Internal Internal `pulumi:"internal"`
}

type Bar struct {
	OtherEnum EnumByRef `pulumi:"other"`
	Foo       Foo       `pulumi:"foo"`
}

func TestCrawlTypes(t *testing.T) {
	t.Parallel()
	m := map[string]pschema.ComplexTypeSpec{}
	reg := func(typ tokens.Type, spec pschema.ComplexTypeSpec) bool {
		_, ok := m[typ.String()]
		if ok {
			return false
		}
		m[typ.String()] = spec
		return true
	}
	err := registerTypes[Foo](reg)
	assert.NoError(t, err)

	assert.Equal(t,
		map[string]pschema.ComplexTypeSpec{
			"pkg:infer:Bar": {
				ObjectTypeSpec: pschema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]pschema.PropertySpec{
						"foo": {
							TypeSpec: pschema.TypeSpec{
								Ref: "#/types/pkg:infer:Foo"},
						},
						"other": {
							TypeSpec: pschema.TypeSpec{
								Ref: "#/types/pkg:infer:EnumByRef"}}},
					Required: []string{"other", "foo"}}},
			"pkg:infer:EnumByRef": {
				ObjectTypeSpec: pschema.ObjectTypeSpec{
					Type: "number"},
				Enum: []pschema.EnumValueSpec{
					{
						Description: "approximate of PI",
						Value:       3.1415}}},
			"pkg:infer:Foo": {
				ObjectTypeSpec: pschema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]pschema.PropertySpec{
						"bar": {
							TypeSpec: pschema.TypeSpec{
								Ref: "#/types/pkg:infer:Bar"}},
						"enum": {
							TypeSpec: pschema.TypeSpec{
								Ref: "#/types/pkg:infer:MyEnum"}},
						"external": {
							TypeSpec: pschema.TypeSpec{
								Ref: "/example/v0.1.2/schema.json#/types/example:mod:Internal",
							},
						}},
					Required: []string{"bar", "enum"}}},
			"pkg:infer:MyEnum": {
				ObjectTypeSpec: pschema.ObjectTypeSpec{
					Type: "string"},
				Enum: []pschema.EnumValueSpec{
					{
						Description: "The foo value",
						Value:       "foo"},
					{
						Description: "The bar value",
						Value:       "bar"}}}},
		m)
}

type outer struct {
	Inner inner `pulumi:"inner"`
}
type inner struct {
	ID string `pulumi:"id"`
}

func TestReservedFields(t *testing.T) {
	t.Parallel()

	reg := func(typ tokens.Type, spec pschema.ComplexTypeSpec) bool {
		return true
	}
	err := registerTypes[outer](reg)
	assert.NoError(t, err, "id isn't reserved on nested fields")

	err = registerTypes[inner](reg)
	assert.ErrorContains(t, err, `"id" is a reserved field name`)
}

func noOpRegister() schema.RegisterDerivativeType {
	m := map[tokens.Type]struct{}{}
	return func(tk tokens.Type, typ pschema.ComplexTypeSpec) (unknown bool) {
		_, known := m[tk]
		m[tk] = struct{}{}
		return !known
	}
}

func registerOk[T any]() func(t *testing.T) {
	return func(t *testing.T) {
		t.Parallel()
		err := registerTypes[T](noOpRegister())
		assert.NoError(t, err)
	}
}

//nolint:paralleltest
func TestInvalidOptionalProperty(t *testing.T) {
	t.Parallel()

	type invalidContainsOptionalEnum struct {
		Foo MyEnum `pulumi:"name,optional"`
	}
	type validContainsEnum struct {
		Foo MyEnum `pulumi:"name"`
	}

	type testInner struct {
		Foo string `pulumi:"foo"`
	}
	type invalidContainsOptionalStruct struct {
		Field testInner `pulumi:"name,optional"`
	}

	t.Run("invalid optional enum", func(t *testing.T) {
		t.Parallel()
		err := registerTypes[invalidContainsOptionalEnum](noOpRegister())

		var actual optionalNeedsPointerError
		if assert.ErrorAs(t, err, &actual) {
			assert.Equal(t, optionalNeedsPointerError{
				ParentStruct: "infer.invalidContainsOptionalEnum",
				PropertyName: "Foo",
				Kind:         "enum",
			}, actual)
		}
	})

	t.Run("valid optional enum", registerOk[struct {
		Foo *MyEnum `pulumi:"name,optional"`
	}]())

	t.Run("valid enum", registerOk[struct {
		Foo MyEnum `pulumi:"name"`
	}]())

	t.Run("invalid optional struct", func(t *testing.T) {
		t.Parallel()
		err := registerTypes[invalidContainsOptionalStruct](noOpRegister())

		var actual optionalNeedsPointerError
		if assert.ErrorAs(t, err, &actual) {
			assert.Equal(t, optionalNeedsPointerError{
				ParentStruct: "infer.invalidContainsOptionalStruct",
				PropertyName: "Field",
				Kind:         "struct",
			}, actual)
		}
	})

	t.Run("valid optional struct", registerOk[struct {
		Field *testInner `pulumi:"name,optional"`
	}]())

	t.Run("valid struct", registerOk[struct {
		Field testInner `pulumi:"name"`
	}]())

	t.Run("optional scalar", registerOk[struct {
		S string `pulumi:"s,optional"`
	}]())

	t.Run("optional array", registerOk[struct {
		Arr []testInner `pulumi:"arr,optional"`
	}]())
}
