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

package introspect_test

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi-go-provider/internal/introspect"
)

type MyStruct struct {
	Foo     string `pulumi:"foo,optional" provider:"secret,output"`
	Bar     int    `provider:"secret"`
	Fizz    *int   `pulumi:"fizz"`
	ExtType string `pulumi:"typ" provider:"type=example@1.2.3:m1:m2"`
}

func (m *MyStruct) Annotate(a infer.Annotator) {
	a.Describe(&m, "This is MyStruct, but also your struct.")
	a.Describe(&m.Fizz, "Fizz is not MyStruct.Foo.")
	a.SetDefault(&m.Foo, "Fizz")
	a.SetToken("myMod", "MyToken")
}

func TestParseTag(t *testing.T) {
	t.Parallel()
	typ := reflect.TypeOf(MyStruct{})

	cases := []struct {
		Field    string
		Expected introspect.FieldTag
		Error    string
	}{
		{
			Field: "Foo",
			Expected: introspect.FieldTag{
				Name:     "foo",
				Optional: true,
				Secret:   true,
			},
		},
		{
			Field: "Bar",
			Error: "you must put to the `pulumi` tag to use the `provider` tag",
		},
		{
			Field: "Fizz",
			Expected: introspect.FieldTag{
				Name: "fizz",
			},
		},
		{
			Field: "ExtType",
			Expected: introspect.FieldTag{
				Name: "typ",
				ExplicitRef: &introspect.ExplicitType{
					Pkg:     "example",
					Version: "v1.2.3",
					Module:  "m1",
					Name:    "m2",
				},
			},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.Field, func(t *testing.T) {
			t.Parallel()
			field, ok := typ.FieldByName(c.Field)
			assert.True(t, ok)
			tag, err := introspect.ParseTag(field)
			if c.Error != "" {
				assert.Equal(t, c.Error, err.Error())
			} else {
				assert.Equal(t, c.Expected, tag)
			}
		})
	}
}

func TestAnnotate(t *testing.T) {
	t.Parallel()

	s := &MyStruct{}

	a := introspect.NewAnnotator(s)

	s.Annotate(&a)

	assert.Equal(t, "Fizz", a.Defaults["foo"])
	assert.Equal(t, "Fizz is not MyStruct.Foo.", a.Descriptions["fizz"])
	assert.Equal(t, "This is MyStruct, but also your struct.", a.Descriptions[""])
	assert.Equal(t, "pkg:myMod:MyToken", a.Token)
}

func TestSetTokenValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		module, name string
		fail         bool
	}{
		{name: "foo"},
		{name: "FOO"},
		{name: "foo/bar", fail: true},
		{name: ":foo", fail: true},
		{module: "foo/bar"},
		{module: " foo", fail: true},
		{name: "angel😇", fail: true},
		{module: ":mod", fail: true},
	}

	for _, tt := range tests {
		tt := tt
		if tt.module == "" {
			tt.module = "mod"
		}
		if tt.name == "" {
			tt.name = "Res"
		}
		t.Run(fmt.Sprintf("%s-%s", tt.module, tt.name), func(t *testing.T) {
			t.Parallel()

			f := func() introspect.Annotator {
				s := &MyStruct{}
				a := introspect.NewAnnotator(s)
				a.SetToken(tt.module, tt.name)
				return a
			}

			if tt.fail {
				assert.Panics(t, func() { f() })
			} else {
				a := f()
				assert.Equal(t, a.Token, "pkg:"+tt.module+":"+tt.name)
			}
		})
	}
}

func TestAllFields(t *testing.T) {
	t.Parallel()

	type MyStruct struct {
		Foo     string `pulumi:"foo,optional" provider:"secret,output"`
		Fizz    *int   `pulumi:"fizz"`
		ExtType string
	}
	s := &MyStruct{}
	fm := introspect.NewFieldMatcher(s)

	fields, ok, err := fm.TargetStructFields(s)
	require.True(t, ok)
	assert.NoError(t, err)
	assert.Len(t, fields, 2)
}

func TestAllFieldsMiss(t *testing.T) {
	t.Parallel()

	type MyStruct struct {
		Foo     string `pulumi:"foo,optional" provider:"secret,output"`
		Fizz    *int   `pulumi:"fizz"`
		ExtType string
	}
	s := &MyStruct{}
	fm := introspect.NewFieldMatcher(s)

	_, ok, err := fm.TargetStructFields(&s.Fizz)
	require.False(t, ok)
	assert.NoError(t, err)
}
