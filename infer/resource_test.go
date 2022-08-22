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

	r "github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
)

func TestExtractSecrets(t *testing.T) {
	t.Parallel()
	m := r.PropertyMap{
		"foo": r.NewArrayProperty([]r.PropertyValue{
			r.NewStringProperty("foo"),
			r.MakeSecret(r.NewNumberProperty(3.14)),
		}),
		"bar": r.NewObjectProperty(r.PropertyMap{
			"fizz": r.MakeSecret(r.NewArrayProperty([]r.PropertyValue{
				r.NewStringProperty("buzz"),
				r.NewBoolProperty(false),
			})),
		}),
		"nested": r.MakeSecret(r.NewObjectProperty(r.PropertyMap{
			"value": r.MakeSecret(r.NewBoolProperty(true)),
		})),
		"simple": r.MakeSecret(r.NewStringProperty("secrets")),
	}
	m, secrets := extractSecrets(m)
	assert.Equal(t,
		r.PropertyMap{
			"foo": r.NewArrayProperty([]r.PropertyValue{
				r.NewStringProperty("foo"),
				r.NewNumberProperty(3.14),
			}),
			"bar": r.NewObjectProperty(r.PropertyMap{
				"fizz": r.NewArrayProperty([]r.PropertyValue{
					r.NewStringProperty("buzz"),
					r.NewBoolProperty(false),
				}),
			}),
			"nested": r.NewObjectProperty(r.PropertyMap{
				"value": r.NewBoolProperty(true),
			}),
			"simple": r.NewStringProperty("secrets"),
		}, m)

	assert.ElementsMatch(t, []r.PropertyPath{
		[]any{"foo", 1},
		[]any{"bar", "fizz"},
		[]any{"nested"},
		[]any{"nested", "value"},
		[]any{"simple"},
	}, secrets)
}

func TestInsertSecrets(t *testing.T) {
	t.Parallel()
	m := r.PropertyMap{
		"foo": r.NewArrayProperty([]r.PropertyValue{
			r.NewStringProperty("foo"),
			r.NewNumberProperty(3.14),
		}),
		"bar": r.NewObjectProperty(r.PropertyMap{
			"fizz": r.NewArrayProperty([]r.PropertyValue{
				r.NewStringProperty("buzz"),
				r.NewBoolProperty(false),
			}),
		}),
		"nested": r.NewObjectProperty(r.PropertyMap{
			"value": r.NewBoolProperty(true),
		}),
		"simple": r.NewStringProperty("secrets"),
	}
	secrets := []r.PropertyPath{
		[]any{"foo", 1},
		[]any{"bar", "fizz"},
		[]any{"nested", "value"},
		[]any{"nested"},
		[]any{"simple"},
	}

	m = insertSecrets(m, secrets)
	assert.Equal(t, r.PropertyMap{
		"foo": r.NewArrayProperty([]r.PropertyValue{
			r.NewStringProperty("foo"),
			r.MakeSecret(r.NewNumberProperty(3.14)),
		}),
		"bar": r.NewObjectProperty(r.PropertyMap{
			"fizz": r.MakeSecret(r.NewArrayProperty([]r.PropertyValue{
				r.NewStringProperty("buzz"),
				r.NewBoolProperty(false),
			})),
		}),
		"nested": r.MakeSecret(r.NewObjectProperty(r.PropertyMap{
			"value": r.MakeSecret(r.NewBoolProperty(true)),
		})),
		"simple": r.MakeSecret(r.NewStringProperty("secrets")),
	}, m)
}

func TestNestedSecrets(t *testing.T) {
	t.Parallel()
	original := r.PropertyMap{
		"nested": r.MakeSecret(r.NewObjectProperty(r.PropertyMap{
			"value": r.MakeSecret(r.NewArrayProperty([]r.PropertyValue{
				r.MakeSecret(r.NewBoolProperty(true)),
			})),
		})),
	}
	m, secrets := extractSecrets(original.Copy())
	assert.Equal(t,
		r.PropertyMap{
			"nested": r.NewObjectProperty(r.PropertyMap{
				"value": r.NewArrayProperty([]r.PropertyValue{
					r.NewBoolProperty(true),
				}),
			}),
		}, m)
	assert.Equal(t, []r.PropertyPath{
		[]any{"nested", "value", 0},
		[]any{"nested", "value"},
		[]any{"nested"},
	}, secrets)

	m = insertSecrets(m, secrets)

	assert.Equal(t, original, m)
}

type outerStruct struct {
	Foo   string                 `pulumi:"foo"`
	Bar   int                    `pulumi:"bar"`
	Pi    float64                `pulumi:"pi"`
	Fizz  []string               `pulumi:"fizz"`
	Inner *innerStruct           `pulumi:"inner"`
	Data  map[string]innerStruct `pulumi:"data"`
}

type innerStruct struct {
	Fizz string  `pulumi:"fizz,optional"`
	Bar  float64 `pulumi:"bar"`
}

func TestTypedUnknowns(t *testing.T) {
	t.Parallel()
	m := r.PropertyMap{
		"foo": r.MakeOutput(r.NewStringProperty("")),
		"bar": r.MakeOutput(r.NewStringProperty("")),
		"pi": r.NewOutputProperty(r.Output{
			Element: r.NewNumberProperty(3.14159),
			Known:   true,
		}),
		"fizz":  r.MakeOutput(r.NewStringProperty("")),
		"inner": r.MakeOutput(r.NewStringProperty("")),
		"data":  r.MakeOutput(r.NewStringProperty("")),
	}
	m = typeUnknowns(r.NewObjectProperty(m), reflect.TypeOf(new(outerStruct))).ObjectValue()
	assert.True(t, m["foo"].OutputValue().Element.IsString(), "String")
	assert.True(t, m["bar"].OutputValue().Element.IsNumber(), "Number")
	assert.Equal(t, 3.14159, m["pi"].OutputValue().Element.NumberValue(), "pi")
	assert.True(t, m["fizz"].OutputValue().Element.IsArray(), "Array")
	assert.True(t, m["inner"].OutputValue().Element.IsObject(), "Object")
	assert.Len(t, m["inner"].OutputValue().Element.ObjectValue(), 1)
	assert.True(t, m["data"].OutputValue().Element.IsObject(), "Map")
}
