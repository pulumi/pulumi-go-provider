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
	"reflect"
	"strconv"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	r "github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/asset"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/sig"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer/types"
	"github.com/pulumi/pulumi-go-provider/internal/putil"
	rRapid "github.com/pulumi/pulumi-go-provider/internal/rapid/resource"
)

// testGetDependencies runs a property test that asserts on the flow between inputs and
// outputs for some I, O ∈ GoType and ∀ (old,new) ∈ I, out ∈ O.
func testGetDependencies[I any, O any](t *testing.T,
	wire func(FieldSelector, *I, *O),
	assert func(t *testing.T, oldInput, newInput, output r.PropertyMap),
) {
	var i I
	var o O
	wireDeps := func(f FieldSelector) {
		if wire != nil {
			wire(f, &i, &o)
		}
	}
	setDeps, err := getDependenciesRaw(
		&i, &o, wireDeps,
		false, /*isCreate*/
		true /*isPreview*/)
	require.NoError(t, err)

	inputT := rapid.Just(reflect.TypeOf(i))
	outputT := rapid.Just(reflect.TypeOf(o))

	getMap := func(t rRapid.Typed) r.PropertyMap {
		return t.Value.ObjectValue()
	}

	rapid.Check(t, func(rt *rapid.T) {
		oldInput := rapid.Map(rRapid.ValueOf(inputT), getMap).
			Draw(rt, "oldInput")
		newInput := rapid.Map(rRapid.ValueOf(inputT), getMap).
			Draw(rt, "newInput")
		output := rapid.Map(rRapid.ValueOf(outputT), getMap).
			Draw(rt, "output")

		setDeps(oldInput, newInput, output)

		assert(t, oldInput, newInput, output)
	})
}

func TestDefaultDependencies(t *testing.T) {
	t.Parallel()
	type input struct {
		I1 string            `pulumi:"i1"`
		I2 *int              `pulumi:"i2,optional"`
		I3 map[string]string `pulumi:"i3"`
	}

	type output struct {
		input

		O1 *string        `pulumi:"o1,optional"`
		O2 float64        `pulumi:"o2"`
		O3 map[string]int `pulumi:"o2"`
	}

	assert := func(t *testing.T, oldInput, newInput, output r.PropertyMap) {
		if newInput.ContainsUnknowns() {
			for k, v := range output {
				if newV, ok := newInput[k]; ok &&
					putil.DeepEquals(newV, v) {
					continue
				}
				assert.True(t, putil.IsComputed(v),
					"key: %q", string(k))
			}
		} else if !putil.DeepEquals(
			r.NewObjectProperty(oldInput),
			r.NewObjectProperty(newInput)) {
			// If there is a change, then every item item should be
			// computed, except items that mirror a known input.
			for k, v := range output {
				newV, ok := newInput[k]
				if !ok {
					assert.True(t, putil.IsComputed(v),
						"key: %q", string(k))
				} else if !putil.IsComputed(v) {
					assert.True(t, putil.DeepEquals(v, newV))
				}
			}
		}

		for k, v := range output {
			// An input of the same name is secret, so this should be too.
			if newInput[k].ContainsSecrets() {
				assert.Truef(t, putil.IsSecret(v),
					"key: %q", string(k))
			}
		}
	}

	testGetDependencies[input, output](t, nil, assert)
}

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
func (c Context) RuntimeInformation() p.RunInfo                  { return p.RunInfo{} }

func TestDiff(t *testing.T) {
	t.Parallel()
	type I struct {
		Environment map[string]string `pulumi:"environment,optional"`
	}
	tests := []struct {
		olds property.Map
		news property.Map
		diff map[string]p.DiffKind
	}{
		{
			olds: property.NewMap(map[string]property.Value{
				"environment": property.New(map[string]property.Value{
					"FOO": property.New("foo"),
				}),
			}),
			news: property.NewMap(map[string]property.Value{
				"environment": property.New(map[string]property.Value{
					"FOO": property.New("bar"),
				}),
			}),
			diff: map[string]p.DiffKind{"environment.FOO": "update"},
		},
		{
			olds: property.Map{},
			news: property.NewMap(map[string]property.Value{
				"environment": property.New(map[string]property.Value{
					"FOO": property.New("bar"),
				}),
			}),
			diff: map[string]p.DiffKind{"environment": "add"},
		},
		{
			olds: property.NewMap(map[string]property.Value{
				"environment": property.New(map[string]property.Value{
					"FOO": property.New("bar"),
				}),
			}),
			news: property.Map{},
			diff: map[string]p.DiffKind{"environment": "delete"},
		},
		{
			olds: property.NewMap(map[string]property.Value{
				"environment": property.New(map[string]property.Value{
					"FOO": property.New("bar"),
				}),
				"output": property.New(42.0),
			}),
			news: property.NewMap(map[string]property.Value{}),
			diff: map[string]p.DiffKind{"environment": "delete"},
		},
		{
			olds: property.NewMap(map[string]property.Value{
				"output": property.New(42.0),
			}),
			news: property.NewMap(map[string]property.Value{
				"environment": property.New(map[string]property.Value{
					"FOO": property.New("bar"),
				}),
			}),
			diff: map[string]p.DiffKind{"environment": "add"},
		},
	}

	for _, test := range tests {
		diffRequest := p.DiffRequest{
			ID:   "foo",
			Urn:  r.CreateURN("foo", "a:b:c", "", "proj", "stack"),
			Olds: test.olds,
			News: test.news,
		}
		resp, err := diff[struct{}, I, any](
			Context{context.Background()},
			diffRequest,
			&struct{}{},
			func(string) bool { return false },
		)
		assert.NoError(t, err)
		assert.Len(t, resp.DetailedDiff, len(test.diff))
		for k, v := range resp.DetailedDiff {
			assert.Equal(t, test.diff[k], v.Kind)
		}
	}
}

type testContext struct {
	context.Context

	t *testing.T
}

func (testContext) Log(diag.Severity, string)                {}
func (testContext) Logf(diag.Severity, string, ...any)       {}
func (testContext) LogStatus(diag.Severity, string)          {}
func (testContext) LogStatusf(diag.Severity, string, ...any) {}
func (ctx testContext) RuntimeInformation() p.RunInfo {
	ctx.t.Logf("No RuntimeInformation on a test context")
	ctx.t.FailNow()
	return p.RunInfo{}
}

type contextKey string

var migrationsKey = contextKey("migrations")

type CustomHydrateFromState[O any] struct{}

func (CustomHydrateFromState[O]) StateMigrations(ctx context.Context) []StateMigrationFunc[O] {
	return ctx.Value(migrationsKey).([]StateMigrationFunc[O])
}

func testHydrateFromState[O any](
	oldState, expected property.Map, expectedError error,
	migrations ...StateMigrationFunc[O],
) func(t *testing.T) {
	return func(t *testing.T) {
		t.Parallel()

		ctx := testContext{
			//nolint:revive
			Context: context.WithValue(context.Background(), migrationsKey, migrations),
		}

		enc, actual, err := hydrateFromState[CustomHydrateFromState[O], struct{}, O](ctx, oldState)
		if expectedError != nil {
			assert.ErrorIs(t, err, expectedError)
			return
		}
		m, err := enc.Encode(actual)
		require.NoErrorf(t, err, "We should be able to encode the result to a p.Map")
		assert.Equal(t, expected, r.FromResourcePropertyValue(r.NewProperty(m)).AsMap())
	}
}

// False positives on t.Run(name, testHydrateFromState[T](...))
//
//nolint:paralleltest
func TestHydrateFromState(t *testing.T) {
	t.Parallel()

	type numberMigrateTarget struct {
		Number int `pulumi:"number"`
	}
	type numberMigrateSource struct {
		Number string `pulumi:"number"`
	}

	t.Run("migrate type", testHydrateFromState[numberMigrateTarget](
		property.NewMap(map[string]property.Value{
			"number": property.New("42"),
		}),
		property.NewMap(map[string]property.Value{
			"number": property.New(42.0),
		}),
		nil,
		StateMigration(func(_ context.Context, old numberMigrateSource) (MigrationResult[numberMigrateTarget], error) {
			n, err := strconv.ParseInt(old.Number, 10, 64)
			if err != nil {
				return MigrationResult[numberMigrateTarget]{}, err
			}
			return MigrationResult[numberMigrateTarget]{
				Result: &numberMigrateTarget{
					Number: int(n),
				},
			}, nil
		}),
	))

	t.Run("migrate-raw", testHydrateFromState[numberMigrateTarget](
		property.NewMap(map[string]property.Value{
			"number": property.New("42"),
		}),
		property.NewMap(map[string]property.Value{
			"number": property.New(42.0),
		}),
		nil,
		StateMigration(func(_ context.Context, old property.Map) (MigrationResult[numberMigrateTarget], error) {
			n, err := strconv.ParseInt(old.Get("number").AsString(), 10, 64)
			if err != nil {
				return MigrationResult[numberMigrateTarget]{}, err
			}
			return MigrationResult[numberMigrateTarget]{
				Result: &numberMigrateTarget{
					Number: int(n),
				},
			}, nil
		}),
	))

	t.Run("ordering-success", testHydrateFromState[numberMigrateTarget](
		property.NewMap(map[string]property.Value{
			"number": property.New("0"),
		}),
		property.NewMap(map[string]property.Value{
			"number": property.New(1.0),
		}),
		nil,
		StateMigration(func(context.Context, property.Map) (MigrationResult[numberMigrateTarget], error) {
			return MigrationResult[numberMigrateTarget]{
				Result: &numberMigrateTarget{
					Number: int(1),
				},
			}, nil
		}),
		StateMigration(func(context.Context, property.Map) (MigrationResult[numberMigrateTarget], error) {
			panic("Should never be called")
		}),
	))

	t.Run("ordering", testHydrateFromState[numberMigrateTarget](
		property.NewMap(map[string]property.Value{
			"number": property.New("0"),
		}),
		property.NewMap(map[string]property.Value{
			"number": property.New(2.0),
		}),
		nil,
		StateMigration(func(context.Context, property.Map) (MigrationResult[numberMigrateTarget], error) {
			return MigrationResult[numberMigrateTarget]{
				Result: nil,
			}, nil
		}),
		StateMigration(func(context.Context, property.Map) (MigrationResult[numberMigrateTarget], error) {
			return MigrationResult[numberMigrateTarget]{
				Result: &numberMigrateTarget{
					Number: int(2),
				},
			}, nil
		}),
	))

	type hasAsset struct {
		AA types.AssetOrArchive `pulumi:"aa"`
	}
	testAsset, err := asset.FromText("pulumi")
	require.NoError(t, err)

	// testHydrateFromState decodes and encodes, so the asset should come back out as a plain asset
	// after having been decoded to an AssetOrArchive.
	t.Run("assets", testHydrateFromState[hasAsset](
		property.NewMap(map[string]property.Value{
			"aa": property.New(testAsset),
		}),
		property.NewMap(map[string]property.Value{
			"aa": property.New(map[string]property.Value{
				sig.Key: property.New(sig.AssetSig),
				"text":  property.New("pulumi"),
				"hash":  property.New(testAsset.Hash),
				"path":  property.New(""),
				"uri":   property.New(""),
			}),
		}),
		nil,
	))
}

type checkResource struct {
	P1 string `pulumi:"str,optional"`
}

const defaultValue = "default"

func (c *checkResource) Annotate(a Annotator) {
	a.SetDefault(&c.P1, defaultValue)
}

type checkResourceOutput struct{}

func (c checkResource) Create(context.Context, CreateRequest[checkResource],
) (resp CreateResponse[checkResourceOutput], err error) {
	return resp, nil
}

func TestCheck(t *testing.T) {
	t.Parallel()

	type m = map[string]property.Value

	for tcName, tc := range map[string]struct {
		input    property.Map
		expected string
	}{
		"applies default for missing value":     {property.Map{}, defaultValue},
		"applies default for empty value":       {property.NewMap(m{"str": property.New("")}), defaultValue},
		"no change when default is already set": {property.NewMap(m{"str": property.New(defaultValue)}), defaultValue},
		"respects non-default value":            {property.NewMap(m{"str": property.New("different")}), "different"},
	} {
		tc := tc

		t.Run("Check "+tcName, func(t *testing.T) {
			t.Parallel()
			res := Resource[checkResource]()
			checkResp, err := res.Check(context.Background(), p.CheckRequest{
				Urn:  "a:b:c",
				Olds: property.Map{},
				News: tc.input,
			})
			require.NoError(t, err)
			assert.Empty(t, checkResp.Failures)
			v, ok := checkResp.Inputs.GetOk("str")
			assert.True(t, ok)
			assert.Equal(t, tc.expected, v.AsString())
		})

		t.Run("DefaultCheck "+tcName, func(t *testing.T) {
			t.Parallel()
			in, failures, err := DefaultCheck[checkResource](context.Background(), tc.input)
			require.NoError(t, err)
			assert.Empty(t, failures)
			assert.Equal(t, tc.expected, in.P1)
		})
	}
}
