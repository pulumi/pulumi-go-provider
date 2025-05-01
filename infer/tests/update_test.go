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

	p "github.com/pulumi/pulumi-go-provider"
)

//nolint:lll
func TestUpdateManualDeps(t *testing.T) {
	t.Parallel()

	test := func(
		testName, resource string,
		olds, newsPreview, newsUpdate, expectedPreview, expectedUp property.Map,
	) {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()
			t.Run("preview", func(t *testing.T) {
				t.Parallel()
				prov := provider(t)
				resp, err := prov.Update(p.UpdateRequest{
					ID:      "some-id",
					Urn:     urn(resource, "test"),
					State:   olds,
					Inputs:  newsPreview,
					Preview: true,
				})
				assert.NoError(t, err)
				assert.Equal(t, p.UpdateResponse{
					Properties: expectedPreview,
				}, resp)
			})
			t.Run("update", func(t *testing.T) {
				t.Parallel()
				prov := provider(t)
				resp, err := prov.Update(p.UpdateRequest{
					ID:    "some-id",
					Urn:   urn(resource, "test"),
					State: olds, Inputs: newsUpdate,
				})
				assert.NoError(t, err)
				assert.Equal(t, p.UpdateResponse{
					Properties: expectedUp,
				}, resp)
			})
		})
	}

	type m = map[string]property.Value

	test("unchanged", "Wired",
		property.NewMap(m{"name": property.New("some-id"), "stringAndInt": property.New("str-5"), "stringPlus": property.New("str+")}),  // Old Input
		property.NewMap(m{"string": property.New("str"), "int": property.New(5.0)}),                                                     // New Preview
		property.NewMap(m{"string": property.New("str"), "int": property.New(5.0)}),                                                     // New Update
		property.NewMap(m{"name": property.New("some-id"), "stringAndInt": property.New("str-5"), "stringPlus": property.New("str++")}), // Preview inputs
		property.NewMap(m{"name": property.New("some-id"), "stringAndInt": property.New("str-5"), "stringPlus": property.New("str++")})) // Full inputs

	test("int-computed", "Wired",
		property.NewMap(m{"name": property.New("some-id"), "stringAndInt": property.New("str-5"), "stringPlus": property.New("str+")}),            // Old Input
		property.NewMap(m{"string": property.New("str"), "int": property.New(property.Computed)}),                                                 // New Input
		property.NewMap(m{"string": property.New("str"), "int": property.New(10.0)}),                                                              // New Update
		property.NewMap(m{"name": property.New("some-id"), "stringAndInt": property.New(property.Computed), "stringPlus": property.New("str++")}), // Preview inputs
		property.NewMap(m{"name": property.New("some-id"), "stringAndInt": property.New("str-10"), "stringPlus": property.New("str++")}))          // Full inputs

	test("string-computed", "Wired",
		property.NewMap(m{"name": property.New("some-id"), "stringAndInt": property.New("str-5"), "stringPlus": property.New("str+")}),                      // Old Input
		property.NewMap(m{"string": property.New(property.Computed), "int": property.New(5.0)}),                                                             // New Input
		property.NewMap(m{"string": property.New("foo"), "int": property.New(5.0)}),                                                                         // New Update
		property.NewMap(m{"name": property.New("some-id"), "stringAndInt": property.New(property.Computed), "stringPlus": property.New(property.Computed)}), // Preview inputs
		property.NewMap(m{"name": property.New("some-id"), "stringAndInt": property.New("foo-5"), "stringPlus": property.New("foo++")}))                     // Full inputs

	test("int-changed", "WiredPlus",
		property.NewMap(m{ // Old Input
			"name": property.New("some-id"), "stringAndInt": property.New("str-5"), "stringPlus": property.New("str+"),
			"string": property.New("str"), "int": property.New(5.0),
		}),
		property.NewMap(m{"string": property.New("str"), "int": property.New(10.0)}), // New Input
		property.NewMap(m{"string": property.New("str"), "int": property.New(10.0)}), // New Update
		property.NewMap(m{ // Preview inputs: int changed -> stringAndInt is now computed
			"name": property.New("some-id"), "stringAndInt": property.New(property.Computed), "stringPlus": property.New("str++"),
			"string": property.New("str"), "int": property.New(10.0),
		}),
		property.NewMap(m{ // Full inputs
			"name": property.New("some-id"), "stringAndInt": property.New("str-10"), "stringPlus": property.New("str++"),
			"string": property.New("str"), "int": property.New(10.0),
		}))

	test("string-changed", "WiredPlus",
		property.NewMap(m{ // Old Input
			"name": property.New("some-id"), "stringAndInt": property.New("str-5"), "stringPlus": property.New("str+"),
			"string": property.New("old-str"), "int": property.New(5.0),
		}),
		property.NewMap(m{"string": property.New("new-str"), "int": property.New(5.0)}), // New Input
		property.NewMap(m{"string": property.New("new-str"), "int": property.New(5.0)}), // New Update
		property.NewMap(m{ // Preview inputs
			"name": property.New("some-id"), "stringAndInt": property.New(property.Computed), "stringPlus": property.New(property.Computed),
			"string": property.New("new-str"), "int": property.New(5.0),
		}),
		property.NewMap(m{ // Full inputs
			"name": property.New("some-id"), "stringAndInt": property.New("new-str-5"), "stringPlus": property.New("new-str++"),
			"string": property.New("new-str"), "int": property.New(5.0),
		}))
}

func TestUpdateDefaultDeps(t *testing.T) {
	t.Parallel()

	test := func(t *testing.T, newString property.Value,
		expectedPreview, expectedUp property.Map,
	) {
		t.Run("preview", func(t *testing.T) {
			t.Parallel()
			prov := provider(t)
			resp, err := prov.Update(p.UpdateRequest{
				ID:  "some-id",
				Urn: urn("Echo", "preview"),
				State: property.NewMap(map[string]property.Value{
					"string":    property.New("old-string"),
					"int":       property.New(1.0),
					"intOut":    property.New(1.0),
					"nameOut":   property.New("old-name"),
					"stringOut": property.New("old-string"),
				}),
				Inputs: property.NewMap(map[string]property.Value{
					// Became computed
					"string": newString,
					"int":    property.New(1.0),
				}),
				Preview: true,
			})
			assert.NoError(t, err)
			assert.Equal(t, p.UpdateResponse{
				Properties: expectedPreview,
			}, resp)
		})
		if newString.IsComputed() {
			// We can't run the update with newString if it's computed, since
			// providers should never receive computed values on a non-preview
			// update.
			return
		}
		t.Run("update", func(t *testing.T) {
			t.Parallel()
			prov := provider(t)
			resp, err := prov.Update(p.UpdateRequest{
				ID:  "some-id",
				Urn: urn("Echo", "update"),
				State: property.NewMap(map[string]property.Value{
					"string":    property.New("old-string"),
					"int":       property.New(1.0),
					"intOut":    property.New(1.0),
					"nameOut":   property.New("old-name"),
					"stringOut": property.New("old-string"),
				}),
				Inputs: property.NewMap(map[string]property.Value{
					"string": newString,
					"int":    property.New(1.0),
				}),
			})
			assert.NoError(t, err)
			assert.Equal(t, p.UpdateResponse{
				Properties: expectedUp,
			}, resp)
		})
	}
	t.Run("computed", func(t *testing.T) {
		t.Parallel()
		test(t, property.New(property.Computed),
			property.NewMap(map[string]property.Value{
				"string":    property.New(property.Computed),
				"int":       property.New(1.0),
				"stringOut": property.New(property.Computed),
				"intOut":    property.New(property.Computed),
				"nameOut":   property.New(property.Computed),
			}),
			property.NewMap(map[string]property.Value{
				"string":    property.New("old-string"),
				"int":       property.New(1.0),
				"stringOut": property.New("old-string"),
				"intOut":    property.New(1.0),
				"nameOut":   property.New("old-name"),
			}),
		)
	})
	t.Run("changed", func(t *testing.T) {
		t.Parallel()
		test(t, property.New("new-string"),
			property.NewMap(map[string]property.Value{
				"string":    property.New(property.Computed),
				"int":       property.New(1.0),
				"stringOut": property.New(property.Computed),
				"intOut":    property.New(property.Computed),
				"nameOut":   property.New(property.Computed),
			}),
			property.NewMap(map[string]property.Value{
				"string":    property.New("new-string"),
				"int":       property.New(1.0),
				"stringOut": property.New("new-string"),
				"intOut":    property.New(1.0),
				"nameOut":   property.New("old-name"),
			}),
		)
	})
	t.Run("unchanged", func(t *testing.T) {
		t.Parallel()
		test(t, property.New("old-string"),
			property.NewMap(map[string]property.Value{
				"string":    property.New("old-string"),
				"int":       property.New(1.0),
				"stringOut": property.New("old-string"),
				"intOut":    property.New(1.0),
				"nameOut":   property.New("old-name"),
			}),
			property.NewMap(map[string]property.Value{
				"string":    property.New("old-string"),
				"int":       property.New(1.0),
				"stringOut": property.New("old-string"),
				"intOut":    property.New(1.0),
				"nameOut":   property.New("old-name"),
			}),
		)
	})
}
