// Copyright 2023, Pulumi Corporation.
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

func TestCheckConfig(t *testing.T) {
	t.Parallel()

	prov := providerWithConfig[Config](t)
	resp, err := prov.CheckConfig(p.CheckRequest{
		Inputs: property.NewMap(map[string]property.Value{
			"value":        property.New("foo"),
			"unknownField": property.New("bar"),
		}),
	})
	require.NoError(t, err)
	require.Len(t, resp.Failures, 0)

	// By default, check simply ensures that we can decode cleanly. It removes unknown
	// fields so that diff doesn't trigger on changes to unwatched arguments.
	assert.Equal(t, property.NewMap(map[string]property.Value{
		"value":                      property.New("foo"),
		"__pulumi-go-provider-infer": property.New(true),
	}), resp.Inputs)
}

func TestCheckConfigCustom(t *testing.T) {
	t.Parallel()

	test := func(t *testing.T, inputs, expected property.Map) {
		prov := providerWithConfig[*ConfigCustom](t)
		resp, err := prov.CheckConfig(p.CheckRequest{
			Urn:    urn("provider", "provider"),
			Inputs: inputs,
		})
		require.NoError(t, err)

		assert.Equal(t, expected, resp.Inputs)
	}

	t.Run("empty", func(t *testing.T) {
		t.Parallel()
		test(t, property.Map{}, property.NewMap(map[string]property.Value{
			"__pulumi-go-provider-infer": property.New(true),
		}))
	})
	t.Run("unknown", func(t *testing.T) {
		t.Parallel()
		test(t,
			property.NewMap(map[string]property.Value{"unknownField": property.New("bar")}),
			property.NewMap(map[string]property.Value{"__pulumi-go-provider-infer": property.New(true)}))
	})
	t.Run("number", func(t *testing.T) {
		t.Parallel()
		test(t,
			property.NewMap(map[string]property.Value{"number": property.New(42.0)}),
			property.NewMap(map[string]property.Value{
				"number":                     property.New(42.5),
				"__pulumi-go-provider-infer": property.New(true),
			}))
	})
}
