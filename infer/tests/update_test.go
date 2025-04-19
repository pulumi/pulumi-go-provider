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

	p "github.com/pulumi/pulumi-go-provider"
)

func TestUpdateManualDeps(t *testing.T) {
	t.Parallel()

	type m = resource.PropertyMap
	c := resource.MakeComputed
	s := resource.NewStringProperty
	n := resource.NewNumberProperty

	test := func(
		testName, resource string,
		olds, newsPreview, newsUpdate, expectedPreview, expectedUp m,
	) {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()
			t.Run("preview", func(t *testing.T) {
				t.Parallel()
				prov := provider(t)
				resp, err := prov.Update(p.UpdateRequest{
					ID:   "some-id",
					Urn:  urn(resource, "test"),
					Olds: olds, News: newsPreview,
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
					ID:   "some-id",
					Urn:  urn(resource, "test"),
					Olds: olds, News: newsUpdate,
				})
				assert.NoError(t, err)
				assert.Equal(t, p.UpdateResponse{
					Properties: expectedUp,
				}, resp)
			})
		})
	}

	test("unchanged", "Wired",
		m{"name": s("some-id"), "stringAndInt": s("str-5"), "stringPlus": s("str+")},  // Old Input
		m{"string": s("str"), "int": n(5)},                                            // New Preview
		m{"string": s("str"), "int": n(5)},                                            // New Update
		m{"name": s("some-id"), "stringAndInt": s("str-5"), "stringPlus": s("str++")}, // Preview inputs
		m{"name": s("some-id"), "stringAndInt": s("str-5"), "stringPlus": s("str++")}) // Full inputs

	test("int-computed", "Wired",
		m{"name": s("some-id"), "stringAndInt": s("str-5"), "stringPlus": s("str+")},     // Old Input
		m{"string": s("str"), "int": c(n(5))},                                            // New Input
		m{"string": s("str"), "int": n(10)},                                              // New Update
		m{"name": s("some-id"), "stringAndInt": c(s("str-5")), "stringPlus": s("str++")}, // Preview inputs
		m{"name": s("some-id"), "stringAndInt": s("str-10"), "stringPlus": s("str++")})   // Full inputs

	test("string-computed", "Wired",
		m{"name": s("some-id"), "stringAndInt": s("str-5"), "stringPlus": s("str+")},        // Old Input
		m{"string": c(s("str")), "int": n(5)},                                               // New Input
		m{"string": s("foo"), "int": n(5)},                                                  // New Update
		m{"name": s("some-id"), "stringAndInt": c(s("str-5")), "stringPlus": c(s("str++"))}, // Preview inputs
		m{"name": s("some-id"), "stringAndInt": s("foo-5"), "stringPlus": s("foo++")})       // Full inputs

	test("int-changed", "WiredPlus",
		m{ // Old Input
			"name": s("some-id"), "stringAndInt": s("str-5"), "stringPlus": s("str+"),
			"string": s("str"), "int": n(5),
		},
		m{"string": s("str"), "int": n(10)}, // New Input
		m{"string": s("str"), "int": n(10)}, // New Update
		m{ // Preview inputs: int changed -> stringAndInt is now computed
			"name": s("some-id"), "stringAndInt": c(s("str-10")), "stringPlus": s("str++"),
			"string": s("str"), "int": n(10),
		},
		m{ // Full inputs
			"name": s("some-id"), "stringAndInt": s("str-10"), "stringPlus": s("str++"),
			"string": s("str"), "int": n(10),
		})

	test("string-changed", "WiredPlus",
		m{ // Old Input
			"name": s("some-id"), "stringAndInt": s("str-5"), "stringPlus": s("str+"),
			"string": s("old-str"), "int": n(5),
		},
		m{"string": s("new-str"), "int": n(5)}, // New Input
		m{"string": s("new-str"), "int": n(5)}, // New Update
		m{ // Preview inputs
			"name": s("some-id"), "stringAndInt": c(s("new-str-5")), "stringPlus": c(s("new-str++")),
			"string": s("new-str"), "int": n(5),
		},
		m{ // Full inputs
			"name": s("some-id"), "stringAndInt": s("new-str-5"), "stringPlus": s("new-str++"),
			"string": s("new-str"), "int": n(5),
		})
}

func TestUpdateDefaultDeps(t *testing.T) {
	t.Parallel()
	c := resource.MakeComputed
	s := resource.NewStringProperty
	n := resource.NewNumberProperty

	test := func(t *testing.T, newString resource.PropertyValue,
		expectedPreview, expectedUp resource.PropertyMap) {
		t.Run("preview", func(t *testing.T) {
			t.Parallel()
			prov := provider(t)
			resp, err := prov.Update(p.UpdateRequest{
				ID:  "some-id",
				Urn: urn("Echo", "preview"),
				Olds: resource.PropertyMap{
					"string":    s("old-string"),
					"int":       n(1),
					"intOut":    n(1),
					"nameOut":   s("old-name"),
					"stringOut": s("old-string"),
				},
				News: resource.PropertyMap{
					// Became computed
					"string": newString,
					"int":    n(1),
				},
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
				ID:  "some-id",
				Urn: urn("Echo", "update"),
				Olds: resource.PropertyMap{
					"string":    s("old-string"),
					"int":       n(1),
					"intOut":    n(1),
					"nameOut":   s("old-name"),
					"stringOut": s("old-string"),
				},
				News: resource.PropertyMap{
					// Became computed
					"string": newString,
					"int":    n(1),
				},
			})
			assert.NoError(t, err)
			assert.Equal(t, p.UpdateResponse{
				Properties: expectedUp,
			}, resp)
		})
	}
	t.Run("computed", func(t *testing.T) {
		t.Parallel()
		test(t, c(s("old-string")),
			resource.PropertyMap{
				"string":    c(s("old-string")),
				"int":       n(1),
				"stringOut": c(s("old-string")),
				"intOut":    c(n(1)),
				"nameOut":   c(s("old-name")),
			},
			resource.PropertyMap{
				"string":    s("old-string"),
				"int":       n(1),
				"stringOut": s("old-string"),
				"intOut":    n(1),
				"nameOut":   s("old-name"),
			},
		)
	})
	t.Run("changed", func(t *testing.T) {
		t.Parallel()
		test(t, s("new-string"),
			resource.PropertyMap{
				"string":    c(s("old-string")),
				"int":       n(1),
				"stringOut": c(s("old-string")),
				"intOut":    c(n(1)),
				"nameOut":   c(s("old-name")),
			},
			resource.PropertyMap{
				"string":    s("new-string"),
				"int":       n(1),
				"stringOut": s("new-string"),
				"intOut":    n(1),
				"nameOut":   s("old-name"),
			},
		)
	})
	t.Run("unchanged", func(t *testing.T) {
		t.Parallel()
		test(t, s("old-string"),
			resource.PropertyMap{
				"string":    s("old-string"),
				"int":       n(1),
				"stringOut": s("old-string"),
				"intOut":    n(1),
				"nameOut":   s("old-name"),
			},
			resource.PropertyMap{
				"string":    s("old-string"),
				"int":       n(1),
				"stringOut": s("old-string"),
				"intOut":    n(1),
				"nameOut":   s("old-name"),
			},
		)
	})
}
