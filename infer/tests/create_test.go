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

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	p "github.com/pulumi/pulumi-go-provider"
)

func TestCreate(t *testing.T) {
	t.Run("unwired-preview", func(t *testing.T) {
		prov := provider()
		resp, err := prov.Create(p.CreateRequest{
			Urn: urn("Echo", "preview"),
			Properties: resource.NewPropertyMapFromMap(map[string]interface{}{
				"string": "my string",
				"int":    7,
				"strMap": map[string]string{
					"fizz": "buzz",
					"foo":  "bar",
				},
			}),
			Preview: true,
		})

		assert.NoError(t, err)
		assert.Equal(t, "preview-id", resp.ID)
		c := resource.MakeComputed
		assert.Equal(t, resource.PropertyMap{
			"string": resource.NewStringProperty("my string"),
			"int":    resource.NewNumberProperty(7.0),
			"strMap": resource.NewObjectProperty(resource.PropertyMap{
				"fizz": resource.NewStringProperty("buzz"),
				"foo":  resource.NewStringProperty("bar"),
			}),
			"nameOut":   c(resource.NewStringProperty("")),
			"stringOut": c(resource.NewStringProperty("")),
			"intOut":    c(resource.NewNumberProperty(0)),
		}, resp.Properties)
	})

	t.Run("unwired-up", func(t *testing.T) {
		prov := provider()
		resp, err := prov.Create(p.CreateRequest{
			Urn: urn("Echo", "create"),
			Properties: resource.NewPropertyMapFromMap(map[string]interface{}{
				"string": "my string",
				"int":    7,
				"strMap": map[string]string{
					"fizz": "buzz",
					"foo":  "bar",
				},
			}),
		})

		assert.NoError(t, err)
		assert.Equal(t, "create-id", resp.ID)
		assert.Equal(t, resource.PropertyMap{
			"string": resource.NewStringProperty("my string"),
			"int":    resource.NewNumberProperty(7.0),
			"strMap": resource.NewObjectProperty(resource.PropertyMap{
				"fizz": resource.NewStringProperty("buzz"),
				"foo":  resource.NewStringProperty("bar"),
			}),
			"nameOut":   resource.NewStringProperty("create"),
			"stringOut": resource.NewStringProperty("my string"),
			"intOut":    resource.NewNumberProperty(7.0),
			"strMapOut": resource.NewObjectProperty(resource.PropertyMap{
				"fizz": resource.NewStringProperty("buzz"),
				"foo":  resource.NewStringProperty("bar"),
			}),
		}, resp.Properties)
	})

	t.Run("unwired-secrets", func(t *testing.T) {
		prov := provider()
		sec := resource.MakeSecret
		str := resource.NewStringProperty
		resp, err := prov.Create(p.CreateRequest{
			Urn: urn("Echo", "create"),
			Properties: resource.PropertyMap{
				"string": sec(str("my string")),
				"int":    resource.NewNumberProperty(7.0),
				"strMap": resource.NewObjectProperty(resource.PropertyMap{
					"fizz": sec(str("buzz")),
					"foo":  str("bar"),
				}),
			},
		})

		assert.NoError(t, err)
		assert.Equal(t, "create-id", resp.ID)
		assert.Equal(t, resource.PropertyMap{
			"string": sec(str("my string")),
			"int":    resource.NewNumberProperty(7.0),
			"strMap": resource.NewObjectProperty(resource.PropertyMap{
				"fizz": sec(str("buzz")),
				"foo":  str("bar"),
			}),
			"nameOut":   sec(str("create")),
			"stringOut": sec(str("my string")),
			"intOut":    sec(resource.NewNumberProperty(7.0)),
			"strMapOut": sec(resource.NewObjectProperty(resource.PropertyMap{
				"fizz": str("buzz"),
				"foo":  str("bar"),
			})),
		}, resp.Properties)
	})

	t.Run("unwired-secrets-mutated-input", func(t *testing.T) {
		prov := provider()
		sec := resource.MakeSecret
		num := resource.NewNumberProperty
		resp, err := prov.Create(p.CreateRequest{
			Urn: urn("Increment", "create"),
			Properties: resource.PropertyMap{
				"int":   num(3.0),
				"other": sec(num(0.0)),
			},
		})

		assert.NoError(t, err)
		assert.Equal(t, "id-3", resp.ID)
		assert.Equal(t, resource.PropertyMap{
			"int":   sec(num(4.0)),
			"other": sec(num(0.0)),
		}, resp.Properties)
	})

	t.Run("wired-preview", func(t *testing.T) {
		prov := provider()
		c := resource.MakeComputed
		s := resource.NewStringProperty
		resp, err := prov.Create(p.CreateRequest{
			Urn: urn("Wired", "preview"),
			Properties: resource.PropertyMap{
				"string": c(resource.NewStringProperty("foo")),
				"int":    c(resource.NewNumberProperty(4.0)),
			},
			Preview: true,
		})
		assert.NoError(t, err)
		assert.Equal(t, "preview-id", resp.ID)
		assert.Equal(t, resource.PropertyMap{
			"name":         s("(preview)"),
			"stringPlus":   c(s("")),
			"stringAndInt": c(s("")),
		}, resp.Properties)

	})

	t.Run("wired-up", func(t *testing.T) {
		prov := provider()
		s := resource.NewStringProperty
		resp, err := prov.Create(p.CreateRequest{
			Urn: urn("Wired", "up"),
			Properties: resource.PropertyMap{
				"string": resource.NewStringProperty("foo"),
				"int":    resource.NewNumberProperty(4.0),
			},
		})
		assert.NoError(t, err)
		assert.Equal(t, "up-id", resp.ID)
		assert.Equal(t, resource.PropertyMap{
			"name":         s("(up)"),
			"stringPlus":   s("foo+"),
			"stringAndInt": s("foo-4"),
		}, resp.Properties)
	})
}

func TestCreateDefaults(t *testing.T) {
	t.Parallel()

	// Helper bindings for constructing property maps
	pInt := func(i int) resource.PropertyValue {
		return resource.NewNumberProperty(float64(i))
	}
	pFloat := resource.NewNumberProperty
	pBool := resource.NewBoolProperty
	pString := resource.NewStringProperty
	type pMap = resource.PropertyMap
	type pValue = resource.PropertyValue

	// The property map we get when only default values are applied.
	//
	// These correspond to the Annotate definitions in ./provider.go.
	defaultNestedMap := func() pValue {
		return pValue{V: pMap{
			"b":    pBool(true),
			"f":    pFloat(4),
			"i":    pInt(8),
			"pb":   pBool(true),
			"pf":   pFloat(4),
			"pi":   pInt(8),
			"ps":   pString("two"),
			"s":    pString("two"),
			"pppi": pInt(64)}}
	}
	defaultMap := func() pMap {
		return pMap{
			"pi":        pInt(2),
			"s":         pString("one"),
			"nested":    defaultNestedMap(),
			"nestedPtr": defaultNestedMap(),
		}
	}

	// A helper function for construction test inputs.
	with := func(origin func() pValue, mutation func(pMap)) pValue {
		v := origin().V.(pMap)
		mutation(v)
		return pValue{V: v}
	}

	withDefault := func(mutation func(pMap)) pMap {
		return with(func() pValue {
			return pValue{V: defaultMap()}
		}, mutation).V.(pMap)
	}

	// Run the test with a set of expected inputs.
	against := func(inputs, expected pMap) func(t *testing.T) {
		return func(t *testing.T) {
			t.Parallel()

			prov := provider()
			resp, err := prov.Create(p.CreateRequest{
				Urn:        urn("WithDefaults", "check-defaults"),
				Properties: inputs,
			})
			require.NoError(t, err)

			assert.Equal(t, expected, resp.Properties)
		}
	}

	t.Run("empty", against(nil, defaultMap()))
	t.Run("some-values", against(pMap{
		"pi": pInt(3),
		"nestedPtr": pValue{V: pMap{
			"i": pInt(3),
		}},
	},
		withDefault(func(m pMap) {
			m["pi"] = pInt(3)
			m["nestedPtr"] = with(defaultNestedMap, func(m pMap) {
				m["i"] = pInt(3)
			})
		}),
	))
	t.Run("set-optional-value-as-zero", against(pMap{
		"pi": pInt(0), // We can set a pointer to its elements zero value.

		// We cannot set a element to its zero value, since that looks identical
		// to not setting it.
		//"s":  pString(""),
	},
		withDefault(func(m pMap) {
			m["pi"] = pInt(0)
		}),
	))

	for _, arrayName := range []string{"arrNested", "arrNestedPtr"} {
		arrayName := arrayName
		array := resource.PropertyKey(arrayName)
		t.Run("behind-"+arrayName, against(pMap{
			array: pValue{V: []pValue{
				pValue{V: pMap{"s": pString("foo")}},
				pValue{V: pMap{}},
				pValue{V: pMap{"s": pString("bar")}},
			}},
		},
			withDefault(func(m pMap) {
				m[array] = pValue{V: []pValue{
					with(defaultNestedMap, func(m pMap) {
						m["s"] = pString("foo")
					}),
					defaultNestedMap(),
					with(defaultNestedMap, func(m pMap) {
						m["s"] = pString("bar")
					}),
				}}
			}),
		))
	}

	for _, mapName := range []string{"mapNested", "mapNestedPtr"} {
		mapName := mapName
		mapK := resource.PropertyKey(mapName)
		t.Run("behind-"+mapName, against(pMap{
			mapK: pValue{V: pMap{
				"one":   pValue{V: pMap{"s": pString("foo")}},
				"two":   pValue{V: pMap{}},
				"three": pValue{V: pMap{"s": pString("bar")}},
			}},
		},
			withDefault(func(m pMap) {
				m[mapK] = pValue{V: pMap{
					"one": with(defaultNestedMap, func(m pMap) {
						m["s"] = pString("foo")
					}),
					"two": defaultNestedMap(),
					"three": with(defaultNestedMap, func(m pMap) {
						m["s"] = pString("bar")
					}),
				}}
			}),
		))
	}
}
