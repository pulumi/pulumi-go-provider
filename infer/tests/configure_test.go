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

func TestConfigure(t *testing.T) {
	t.Parallel()

	prov := providerWithConfig[Config](t)
	err := prov.Configure(p.ConfigureRequest{
		Args: property.NewMap(map[string]property.Value{
			"value":        property.New("foo"),
			"unknownField": property.New("bar"),
		}),
	})
	require.NoError(t, err)

	resp, err := prov.Create(p.CreateRequest{
		Urn: urn("ReadConfig", "config"),
	})
	require.NoError(t, err)
	assert.Equal(t, property.NewMap(map[string]property.Value{
		"config": property.New("{\"Value\":\"foo\"}"),
	}), resp.Properties)
}

func TestConfigureCustom(t *testing.T) {
	t.Parallel()

	test := func(inputs, expected property.Map) func(t *testing.T) {
		return func(t *testing.T) {
			t.Parallel()

			prov := providerWithConfig[*ConfigCustom](t)
			err := prov.Configure(p.ConfigureRequest{
				Args: inputs,
			})
			require.NoError(t, err)

			resp, err := prov.Create(p.CreateRequest{
				Urn: urn("ReadConfigCustom", "config"),
			})
			require.NoError(t, err)
			assert.Equal(t, expected, resp.Properties)
		}
	}

	t.Run("empty", test( //nolint:paralleltest // test already calls t.Parallel.
		property.Map{},
		property.NewMap(map[string]property.Value{"config": property.New(`{"Number":null,"Squared":0}`)})))
	t.Run("unknown", test( //nolint:paralleltest // test already calls t.Parallel.
		property.NewMap(map[string]property.Value{"unknownField": property.New("bar")}),
		property.NewMap(map[string]property.Value{"config": property.New(`{"Number":null,"Squared":0}`)})))
	t.Run("number", test( //nolint:paralleltest // test already calls t.Parallel.
		property.NewMap(map[string]property.Value{"number": property.New(42.0)}),
		property.NewMap(map[string]property.Value{"config": property.New(`{"Number":42,"Squared":1764}`)})))
}
