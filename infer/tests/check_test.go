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

package tests

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	p "github.com/pulumi/pulumi-go-provider"
)

func TestCheckDefaults(t *testing.T) {
	t.Parallel()

	// The property map we get when only default values are applied.
	//
	// These correspond to the Annotate definitions in ./provider.go.
	defaultNestedMap := property.New(map[string]property.Value{
		"b":    property.New(true),
		"f":    property.New(4.0),
		"i":    property.New(8.0),
		"pb":   property.New(true),
		"pf":   property.New(4.0),
		"pi":   property.New(8.0),
		"ps":   property.New("two"),
		"s":    property.New("two"),
		"pppi": property.New(64.0),
	})

	defaultMap := property.NewMap(map[string]property.Value{
		"pi":        property.New(2.0),
		"s":         property.New("one"),
		"nestedPtr": defaultNestedMap,
	})

	// Run the test with a set of expected inputs.
	against := func(inputs, expected property.Map) func(t *testing.T) {
		return func(t *testing.T) {
			t.Parallel()

			// This is a required input, so make sure it shows up.
			if _, ok := inputs.GetOk("nestedPtr"); !ok {
				inputs = inputs.Set("nestedPtr", property.New(property.Map{}))
			}

			prov := provider(t)
			resp, err := prov.Check(p.CheckRequest{
				Urn:    urn("WithDefaults", "check-defaults"),
				Inputs: inputs,
			})
			require.NoError(t, err)
			require.Len(t, resp.Failures, 0)

			assert.Equal(t, expected, resp.Inputs)
		}
	}

	type m = map[string]property.Value

	t.Run("empty", against(property.Map{}, defaultMap))         //nolint:paralleltest // against already calls t.Parallel.
	t.Run("required-under-optional", against(property.NewMap(m{ //nolint:paralleltest // against already calls t.Parallel.
		"optWithReq": property.New(m{
			"req": property.New("user-value"),
		}),
	}), defaultMap.Set("optWithReq", property.New(m{
		"req": property.New("user-value"),
		"opt": property.New("default-value"),
	}))))
	t.Run("some-values", against(property.NewMap(m{ //nolint:paralleltest // against already calls t.Parallel.
		"pi": property.New(3.0),
		"nestedPtr": property.New(m{
			"i": property.New(3.0),
		}),
	}),
		defaultMap.Set("pi", property.New(3.0)).Set(
			"nestedPtr", property.New(defaultNestedMap.AsMap().Set(
				"i", property.New(3.0),
			)),
		),
	))
	t.Run("set-optional-value-as-zero", against(property.NewMap(m{ //nolint:paralleltest,lll // against already calls t.Parallel.
		"pi": property.New(0.0), // We can set a pointer to its elements zero value.

		// We cannot set a element to its zero value, since that looks identical
		// to not setting it.
		//"s":  pString(""),
	}),
		defaultMap.Set("pi", property.New(0.0)),
	))

	for _, arrayName := range []string{"arrNested", "arrNestedPtr"} {
		arrayName := arrayName
		t.Run("behind-"+arrayName, against(property.NewMap(m{
			arrayName: property.New([]property.Value{
				property.New(m{"s": property.New("foo")}),
				property.New(property.Map{}),
				property.New(m{"s": property.New("bar")}),
			}),
		}),
			defaultMap.Set(arrayName, property.New([]property.Value{
				property.New(defaultNestedMap.AsMap().Set("s", property.New("foo"))),
				defaultNestedMap,
				property.New(defaultNestedMap.AsMap().Set("s", property.New("bar"))),
			})),
		))
	}

	for _, mapName := range []string{"mapNested", "mapNestedPtr"} { //nolint:paralleltest
		mapName := mapName
		t.Run("behind-"+mapName, against(property.NewMap(m{
			mapName: property.New(m{
				"one":   property.New(m{"s": property.New("foo")}),
				"two":   property.New(m{}),
				"three": property.New(m{"s": property.New("bar")}),
			}),
		}),
			defaultMap.Set(mapName, property.New(m{
				"one":   property.New(defaultNestedMap.AsMap().Set("s", property.New("foo"))),
				"two":   defaultNestedMap,
				"three": property.New(defaultNestedMap.AsMap().Set("s", property.New("bar"))),
			})),
		))
	}
}

func TestCheckDefaultsEnv(t *testing.T) {
	t.Setenv("STRING", "str")
	t.Setenv("INT", "1")
	t.Setenv("FLOAT64", "3.14")
	t.Setenv("BOOL", "T")

	prov := provider(t)
	resp, err := prov.Check(p.CheckRequest{
		Urn:    urn("ReadEnv", "check-env"),
		Inputs: property.Map{},
	})
	require.NoError(t, err)

	assert.Equal(t, property.NewMap(map[string]property.Value{
		"b":   property.New(true),
		"f64": property.New(3.14),
		"i":   property.New(1.0),
		"s":   property.New("str"),
	}), resp.Inputs)
}

func TestCheckDefaultsRecursive(t *testing.T) {
	t.Parallel()

	prov := provider(t)

	// If we just have a type without the recursive field nil, we don't recurse.
	resp, err := prov.Check(p.CheckRequest{
		Urn:    urn("Recursive", "check-env"),
		Inputs: property.Map{},
	})
	require.NoError(t, err)

	assert.Equal(t, property.NewMap(map[string]property.Value{
		"value": property.New("default-value"),
	}), resp.Inputs)

	// If the input type has hydrated recursive values, we should hydrate all non-nil
	// values.
	resp, err = prov.Check(p.CheckRequest{
		Urn: urn("Recursive", "check-env"),
		Inputs: property.NewMap(map[string]property.Value{
			"other": property.New(map[string]property.Value{
				"other": property.New(map[string]property.Value{
					"value": property.New("custom"),
					"other": property.New(map[string]property.Value{}),
				}),
			}),
		}),
	})
	require.NoError(t, err)

	assert.Equal(t, property.NewMap(map[string]property.Value{
		"value": property.New("default-value"),
		"other": property.New(map[string]property.Value{
			"value": property.New("default-value"),
			"other": property.New(map[string]property.Value{
				"value": property.New("custom"),
				"other": property.New(map[string]property.Value{
					"value": property.New("default-value"),
				}),
			}),
		}),
	}), resp.Inputs)
}

// TestCheckAlwaysAppliesSecrets checks that if a inferred provider resource has (1) a
// field marked as secret with a field annotation (`provider:"secret"`), and (2)
// implements [infer.CustomCheck] without calling [infer.DefaultDiff], the field is still
// marked as secret.
func TestCheckAlwaysAppliesSecrets(t *testing.T) {
	t.Parallel()

	prov := provider(t)
	resp, err := prov.Check(p.CheckRequest{
		Urn: urn("CustomCheckNoDefaults", "check-env"),
		Inputs: property.NewMap(map[string]property.Value{
			"input": property.New("value"),
		}),
	})
	require.NoError(t, err)

	assert.Equal(t, property.NewMap(map[string]property.Value{
		"input": property.New("value").WithSecret(true),
	}), resp.Inputs)
}
