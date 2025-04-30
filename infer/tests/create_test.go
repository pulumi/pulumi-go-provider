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

func TestCreate(t *testing.T) {
	t.Parallel()

	t.Run("unwired-preview", func(t *testing.T) {
		t.Parallel()
		prov := provider(t)
		resp, err := prov.Create(p.CreateRequest{
			Urn: urn("Echo", "preview"),
			Properties: property.NewMap(map[string]property.Value{
				"string": property.New("my string"),
				"int":    property.New(7.0),
				"strMap": property.New(map[string]property.Value{
					"fizz": property.New("buzz"),
					"foo":  property.New("bar"),
				}),
			}),
			Preview: true,
		})

		assert.NoError(t, err)
		assert.Equal(t, "preview-id", resp.ID)
		assert.Equal(t, property.NewMap(map[string]property.Value{
			"string": property.New("my string"),
			"int":    property.New(7.0),
			"strMap": property.New(map[string]property.Value{
				"fizz": property.New("buzz"),
				"foo":  property.New("bar"),
			}),
			"nameOut":   property.New(property.Computed),
			"stringOut": property.New(property.Computed),
			"intOut":    property.New(property.Computed),
		}), resp.Properties)
	})

	t.Run("unwired-up", func(t *testing.T) {
		t.Parallel()

		prov := provider(t)
		resp, err := prov.Create(p.CreateRequest{
			Urn: urn("Echo", "create"),
			Properties: property.NewMap(map[string]property.Value{
				"string": property.New("my string"),
				"int":    property.New(7.0),
				"strMap": property.New(map[string]property.Value{
					"fizz": property.New("buzz"),
					"foo":  property.New("bar"),
				}),
			}),
		})

		assert.NoError(t, err)
		assert.Equal(t, "create-id", resp.ID)
		assert.Equal(t, property.NewMap(map[string]property.Value{
			"string": property.New("my string"),
			"int":    property.New(7.0),
			"strMap": property.New(map[string]property.Value{
				"fizz": property.New("buzz"),
				"foo":  property.New("bar"),
			}),
			"nameOut":   property.New("create"),
			"stringOut": property.New("my string"),
			"intOut":    property.New(7.0),
			"strMapOut": property.New(map[string]property.Value{
				"fizz": property.New("buzz"),
				"foo":  property.New("bar"),
			}),
		}), resp.Properties)
	})

	t.Run("unwired-secrets", func(t *testing.T) {
		t.Parallel()

		prov := provider(t)
		resp, err := prov.Create(p.CreateRequest{
			Urn: urn("Echo", "create"),
			Properties: property.NewMap(map[string]property.Value{
				"string": property.New("my string").WithSecret(true),
				"int":    property.New(7.0),
				"strMap": property.New(map[string]property.Value{
					"fizz": property.New("buzz").WithSecret(true),
					"foo":  property.New("bar"),
				}),
			}),
		})

		assert.NoError(t, err)
		assert.Equal(t, "create-id", resp.ID)
		assert.Equal(t, property.NewMap(map[string]property.Value{
			"string": property.New("my string").WithSecret(true),
			"int":    property.New(7.0),
			"strMap": property.New(map[string]property.Value{
				"fizz": property.New("buzz").WithSecret(true),
				"foo":  property.New("bar"),
			}),
			"nameOut":   property.New("create"),
			"stringOut": property.New("my string"),
			"intOut":    property.New(7.0),
			"strMapOut": property.New(map[string]property.Value{
				"fizz": property.New("buzz"),
				"foo":  property.New("bar"),
			}),
		}), resp.Properties)
	})

	t.Run("unwired-secrets-mutated-input", func(t *testing.T) {
		t.Parallel()

		prov := provider(t)
		resp, err := prov.Create(p.CreateRequest{
			Urn: urn("Increment", "create"),
			Properties: property.NewMap(map[string]property.Value{
				"int":   property.New(3.0),
				"other": property.New(0.0).WithSecret(true),
			}),
		})

		assert.NoError(t, err)
		assert.Equal(t, "id-3", resp.ID)
		assert.Equal(t, property.NewMap(map[string]property.Value{
			"int":   property.New(4.0),
			"other": property.New(0.0).WithSecret(true),
		}), resp.Properties)
	})

	t.Run("wired-secrets", func(t *testing.T) {
		t.Parallel()

		prov := provider(t)
		resp, err := prov.Create(p.CreateRequest{
			Urn: urn("Wired", "preview"),
			Properties: property.NewMap(map[string]property.Value{
				"string": property.New(property.Computed),
				"int":    property.New(property.Computed).WithSecret(true),
			}),
			Preview: true,
		})
		assert.NoError(t, err)
		assert.Equal(t, "preview-id", resp.ID)
		assert.Equal(t, property.NewMap(map[string]property.Value{
			"name":         property.New("(preview)"),
			"stringPlus":   property.New(property.Computed),
			"stringAndInt": property.New(property.Computed).WithSecret(true).WithDependencies(nil),
		}), resp.Properties)
	})

	t.Run("wired-preview", func(t *testing.T) {
		t.Parallel()

		prov := provider(t)
		resp, err := prov.Create(p.CreateRequest{
			Urn: urn("Wired", "preview"),
			Properties: property.NewMap(map[string]property.Value{
				"string": property.New(property.Computed),
				"int":    property.New(property.Computed),
			}),
			Preview: true,
		})
		assert.NoError(t, err)
		assert.Equal(t, "preview-id", resp.ID)
		assert.Equal(t, property.NewMap(map[string]property.Value{
			"name":         property.New("(preview)"),
			"stringPlus":   property.New(property.Computed),
			"stringAndInt": property.New(property.Computed),
		}), resp.Properties)
	})

	t.Run("wired-up", func(t *testing.T) {
		t.Parallel()

		prov := provider(t)
		resp, err := prov.Create(p.CreateRequest{
			Urn: urn("Wired", "up"),
			Properties: property.NewMap(map[string]property.Value{
				"string": property.New("foo"),
				"int":    property.New(4.0),
			}),
		})
		assert.NoError(t, err)
		assert.Equal(t, "up-id", resp.ID)
		assert.Equal(t, property.NewMap(map[string]property.Value{
			"name":         property.New("(up)"),
			"stringPlus":   property.New("foo+"),
			"stringAndInt": property.New("foo-4"),
		}), resp.Properties)
	})
}
