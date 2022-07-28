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
	"context"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-go-provider/internal/introspect"
	"github.com/pulumi/pulumi-go-provider/resource"
)

type MyStruct struct {
	Foo  string `pulumi:"foo,optional" provider:"secret,output"`
	Bar  int    `provider:"secret"`
	Fizz *int   `pulumi:"fizz"`
}

func (m *MyStruct) Annotate(a resource.Annotator) {
	a.Describe(&m, "This is MyStruct, but also your struct.")
	a.Describe(&m.Fizz, "Fizz is not MyStruct.Foo.")
	a.SetDefault(&m.Foo, "Fizz")
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
				Output:   true,
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
}

func Empty() {}

func BetweenStructs(FnInput) FnOutput                       { return FnOutput{} }
func FullValues(context.Context, FnInput) (FnOutput, error) { return FnOutput{}, nil }
func OnlyMetadata(context.Context) error                    { return nil }

type FnInput struct{}
type FnOutput struct{}

func TestFunctions(t *testing.T) {
	t.Parallel()
	cases := []struct {
		f          any
		in         reflect.Type
		hasContext bool
		out        reflect.Type
		canError   bool
	}{
		{
			f: Empty,
		},
		{
			f:   BetweenStructs,
			in:  reflect.TypeOf(FnInput{}),
			out: reflect.TypeOf(FnOutput{}),
		},
		{
			f:          OnlyMetadata,
			hasContext: true,
			canError:   true,
		},
	}

	for _, c := range cases { //nolint:paralleltest
		c := c
		typ := reflect.TypeOf(c.f)
		t.Run(typ.String(), func(t *testing.T) {
			t.Parallel()
			in, hasContext, err := introspect.InvokeInput(typ)
			require.NoError(t, err)
			assert.Equal(t, c.in, in)
			assert.Equal(t, c.hasContext, hasContext)

			out, canError, err := introspect.InvokeOutput(typ)
			require.NoError(t, err)
			assert.Equal(t, c.out, out)
			assert.Equal(t, c.canError, canError)

		})
	}
}
