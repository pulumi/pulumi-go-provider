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

func TestInvoke(t *testing.T) {
	t.Parallel()

	t.Run("missing-arg", func(t *testing.T) {
		t.Parallel()
		prov := provider(t)
		resp, err := prov.Invoke(p.InvokeRequest{
			Token: "test:index:getJoin",
			Args:  property.Map{},
		})
		require.NoError(t, err)
		assert.Equal(t, 1, len(resp.Failures)) // Missing required field `elems`
	})

	t.Run("all-args", func(t *testing.T) {
		t.Parallel()
		prov := provider(t)
		resp, err := prov.Invoke(p.InvokeRequest{
			Token: "test:index:getJoin",
			Args: property.NewMap(map[string]property.Value{
				"elems": property.New([]property.Value{
					property.New("foo"),
					property.New("bar"),
				}),
				"sep": property.New("-"),
			}),
		})
		require.NoError(t, err)
		assert.Empty(t, resp.Failures)

		assert.Equal(t, property.NewMap(map[string]property.Value{
			"result": property.New("foo-bar"),
		}), resp.Return)
	})

	t.Run("default-args", func(t *testing.T) {
		t.Parallel()
		prov := provider(t)
		resp, err := prov.Invoke(p.InvokeRequest{
			Token: "test:index:getJoin",
			Args: property.NewMap(map[string]property.Value{
				"elems": property.New([]property.Value{
					property.New("foo"),
					property.New("bar"),
				}),
			}),
		})
		require.NoError(t, err)
		assert.Empty(t, resp.Failures)

		assert.Equal(t, property.NewMap(map[string]property.Value{
			"result": property.New("foo,bar"), // default value is ","
		}), resp.Return)
	})
	t.Run("zero-args", func(t *testing.T) {
		t.Parallel()
		prov := provider(t)
		resp, err := prov.Invoke(p.InvokeRequest{
			Token: "test:index:getJoin",
			Args: property.NewMap(map[string]property.Value{
				"elems": property.New([]property.Value{
					property.New("foo"),
					property.New("bar"),
				}),
				"sep": property.New(""),
			}),
		})
		require.NoError(t, err)
		assert.Empty(t, resp.Failures)

		assert.Equal(t, property.NewMap(map[string]property.Value{
			"result": property.New("foobar"), // The default doesn't apply here
		}), resp.Return)
	})

}
