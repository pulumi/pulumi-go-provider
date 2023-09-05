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
	"context"
	"testing"

	provider "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	r "github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFieldGenerator(t *testing.T) {
	t.Parallel()
	type args struct {
		Fizz string  `pulumi:"a1,optional"`
		Bar  float64 `pulumi:"a2"`
	}
	type state struct {
		F1 int    `pulumi:"f1,optional"`
		F2 string `pulumi:"f2"`
	}

	tests := []struct {
		name   string
		wire   func(fs FieldSelector, a *args, s *state)
		assert func(t *testing.T, fg fieldGenerator)
	}{
		{
			name: "all deps",
			wire: func(fs FieldSelector, a *args, s *state) {
				allFields := fs.InputField(a)
				fs.OutputField(&s.F1).DependsOn(allFields)
			},
			assert: func(t *testing.T, fg fieldGenerator) {
				out := r.NewPropertyMapFromMap(map[string]interface{}{
					"f1": 0,
					"f2": "a string",
				})

				fg.MarkMap(false, false)(nil, r.PropertyMap{
					"a1": r.MakeSecret(r.NewStringProperty("")),
					"a2": r.NewNumberProperty(0.0),
				}, out)
				require.NoError(t, fg.err.ErrorOrNil())
				assert.True(t, out["f1"].IsSecret(), "f1")
				assert.False(t, out["f2"].IsSecret(), "f2")
			},
		},
		{
			name: "individual deps",
			wire: func(fs FieldSelector, a *args, s *state) {
				fs.OutputField(&s.F1).DependsOn(fs.InputField(&a.Fizz))
				fs.OutputField(&s.F2).DependsOn(fs.InputField(&a.Bar))
			},
			assert: func(t *testing.T, fg fieldGenerator) {
				out := r.NewPropertyMapFromMap(map[string]interface{}{
					"f1": 0,
					"f2": "a string",
				})
				in := r.NewPropertyMapFromMap(map[string]interface{}{
					"a1": r.NewStringProperty(""),
					"a2": r.NewNumberProperty(0),
				})
				test := func(fizz, bar bool) {
					out := out.Copy()
					in := in.Copy()
					if fizz {
						in["a1"] = r.MakeSecret(in["a1"])
					}
					if bar {
						in["a2"] = r.MakeSecret(in["a2"])
					}
					fg.MarkMap(false, false)(nil, in, out)
					assert.Equal(t, fizz, out["f1"].IsSecret())
					assert.Equal(t, bar, out["f2"].IsSecret())
				}

				for _, fizz := range []bool{true, false} {
					for _, bar := range []bool{true, false} {
						test(fizz, bar)
					}
				}
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			i, o := &args{}, &state{}
			fm := newFieldGenerator(i, o)
			tt.wire(fm, i, o)
			tt.assert(t, *fm)
		})
	}

}

type Context struct {
	context.Context
}

func (c Context) Log(_ diag.Severity, _ string)                  {}
func (c Context) Logf(_ diag.Severity, _ string, _ ...any)       {}
func (c Context) LogStatus(_ diag.Severity, _ string)            {}
func (c Context) LogStatusf(_ diag.Severity, _ string, _ ...any) {}
func (c Context) RuntimeInformation() provider.RunInfo           { return provider.RunInfo{} }

func TestDiff(t *testing.T) {
	t.Parallel()
	type I struct {
		Environment map[string]string `pulumi:"environment,optional"`
	}
	tests := []struct {
		olds r.PropertyMap
		news r.PropertyMap
		diff map[string]provider.DiffKind
	}{
		{
			olds: r.PropertyMap{
				"environment": r.NewObjectProperty(r.PropertyMap{
					"FOO": r.NewStringProperty("foo"),
				}),
			},
			news: r.PropertyMap{
				"environment": r.NewObjectProperty(r.PropertyMap{
					"FOO": r.NewStringProperty("bar"),
				}),
			},
			diff: map[string]provider.DiffKind{"environment.FOO": "update"},
		},
		{
			olds: r.PropertyMap{},
			news: r.PropertyMap{
				"environment": r.NewObjectProperty(r.PropertyMap{
					"FOO": r.NewStringProperty("bar"),
				}),
			},
			diff: map[string]provider.DiffKind{"environment": "add"},
		},
		{
			olds: r.PropertyMap{
				"environment": r.NewObjectProperty(r.PropertyMap{
					"FOO": r.NewStringProperty("bar"),
				}),
			},
			news: r.PropertyMap{},
			diff: map[string]provider.DiffKind{"environment": "delete"},
		},
		{
			olds: r.PropertyMap{
				"environment": r.NewObjectProperty(r.PropertyMap{
					"FOO": r.NewStringProperty("bar"),
				}),
				"output": r.NewNumberProperty(42),
			},
			news: r.PropertyMap{},
			diff: map[string]provider.DiffKind{"environment": "delete"},
		},
		{
			olds: r.PropertyMap{
				"output": r.NewNumberProperty(42),
			},
			news: r.PropertyMap{
				"environment": r.NewObjectProperty(r.PropertyMap{
					"FOO": r.NewStringProperty("bar"),
				}),
			},
			diff: map[string]provider.DiffKind{"environment": "add"},
		},
	}

	for _, test := range tests {
		diffRequest := provider.DiffRequest{
			ID:   "foo",
			Urn:  r.CreateURN("foo", "a:b:c", "", "proj", "stack"),
			Olds: test.olds,
			News: test.news,
		}
		resp, err := diff[struct{}, I, any](
			Context{context.Background()},
			diffRequest,
			&struct{}{},
			func(s string) bool { return false },
		)
		assert.NoError(t, err)
		assert.Len(t, resp.DetailedDiff, len(test.diff))
		for k, v := range resp.DetailedDiff {
			assert.Equal(t, test.diff[k], v.Kind)
		}
	}
}
