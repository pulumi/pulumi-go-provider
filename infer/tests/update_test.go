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

func TestUpdateDefaultDeps(t *testing.T) {
	c := resource.MakeComputed
	s := resource.NewStringProperty
	n := resource.NewNumberProperty

	unwired := func(t *testing.T, newString resource.PropertyValue,
		expectedPreview, expectedUp resource.PropertyMap) {
		t.Run("preview", func(t *testing.T) {
			prov := provider()
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
			prov := provider()
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
		unwired(t, c(s("old-string")),
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
		unwired(t, s("new-string"),
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
		unwired(t, s("old-string"),
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
