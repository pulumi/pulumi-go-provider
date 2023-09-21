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
	ende "github.com/pulumi/pulumi-go-provider/infer/internal/ende"
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
			"nameOut":   str("create"),
			"stringOut": str("my string"),
			"intOut":    resource.NewNumberProperty(7.0),
			"strMapOut": resource.NewObjectProperty(resource.PropertyMap{
				"fizz": str("buzz"),
				"foo":  str("bar"),
			}),
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
			"int":   num(4.0),
			"other": sec(num(0.0)),
		}, resp.Properties)
	})

	t.Run("wired-secrets", func(t *testing.T) {
		prov := provider()
		c := resource.MakeComputed
		s := resource.NewStringProperty
		sec := ende.MakeSecret
		resp, err := prov.Create(p.CreateRequest{
			Urn: urn("Wired", "preview"),
			Properties: resource.PropertyMap{
				"string": c(resource.NewStringProperty("foo")),
				"int":    sec(c(resource.NewNumberProperty(4.0))),
			},
			Preview: true,
		})
		assert.NoError(t, err)
		assert.Equal(t, "preview-id", resp.ID)
		assert.Equal(t, resource.PropertyMap{
			"name":         s("(preview)"),
			"stringPlus":   c(s("")),
			"stringAndInt": sec(c(s(""))),
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
