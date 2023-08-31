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

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	p "github.com/pulumi/pulumi-go-provider"
)

func TestCheckConfig(t *testing.T) {
	t.Parallel()
	pString := resource.NewStringProperty
	type pMap = resource.PropertyMap

	prov := providerWithConfig[Config]()
	resp, err := prov.CheckConfig(p.CheckRequest{
		News: pMap{
			"value":        pString("foo"),
			"unknownField": pString("bar"),
		},
	})
	require.NoError(t, err)

	// By default, check simply ensures that we can decode cleanly. It removes unknown
	// fields so that diff doesn't trigger on changes to unwatched arguments.
	assert.Equal(t, pMap{
		"value": pString("foo"),
	}, resp.Inputs)
}

func TestCheckConfigCustom(t *testing.T) {
	t.Parallel()
	pString := resource.NewStringProperty
	pNumber := resource.NewNumberProperty
	type pMap = resource.PropertyMap
	type pValue = resource.PropertyValue

	test := func(inputs, expected pMap) func(t *testing.T) {
		return func(t *testing.T) {
			prov := providerWithConfig[*ConfigCustom]()
			resp, err := prov.CheckConfig(p.CheckRequest{
				Urn:  urn("provider", "provider"),
				News: inputs,
			})
			require.NoError(t, err)

			assert.Equal(t, expected, resp.Inputs)
		}
	}

	t.Run("empty", test(nil, pMap{}))
	t.Run("unknown", test(
		pMap{"unknownField": pString("bar")},
		pMap{}))
	t.Run("number", test(
		pMap{"number": pNumber(42)},
		pMap{"number": pNumber(42.5)}))
}
