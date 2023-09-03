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

package ende

import (
	"testing"

	r "github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testRoundTrip asserts that the result of pMap can be decoded onto T, and then lossly
// encoded back into a property map.
func testRoundTrip[T any](t *testing.T, pMap func() r.PropertyMap) {
	t.Run("", func(t *testing.T) {
		t.Parallel()
		var typeInfo T
		toDecode := pMap()
		encoder, err := Decode(toDecode, &typeInfo)
		require.NoError(t, err)

		assert.Equalf(t, pMap(), toDecode, "mutated decode map")

		reEncoded, err := encoder.Encode(typeInfo)
		require.NoError(t, err)
		assert.Equal(t, pMap(), reEncoded)
	})
}

func TestRoundtrip(t *testing.T) {
	t.Parallel()

	testRoundTrip[struct {
		Foo []any `pulumi:"foo"`
		Bar struct {
			Fizz []any `pulumi:"fizz"`
		} `pulumi:"bar"`
		Nested map[string]bool `pulumi:"nested"`
		Simple string          `pulumi:"simple"`
	}](t, func() r.PropertyMap {
		return r.PropertyMap{
			"foo": r.NewArrayProperty([]r.PropertyValue{
				r.NewStringProperty("foo"),
				r.MakeSecret(r.NewNumberProperty(3.14)),
			}),
			"bar": r.NewObjectProperty(r.PropertyMap{
				"fizz": r.MakeSecret(r.NewArrayProperty([]r.PropertyValue{
					r.NewStringProperty("buzz"),
					r.NewBoolProperty(false),
				})),
			}),
			"nested": r.MakeSecret(r.NewObjectProperty(r.PropertyMap{
				"value": r.MakeSecret(r.NewBoolProperty(true)),
			})),
			"simple": r.MakeSecret(r.NewStringProperty("secrets")),
		}
	})

	testRoundTrip[struct {
		Nested map[string][]bool `pulumi:"nested"`

		NestedObject struct {
			Value []string `pulumi:"value"`
		} `pulumi:"nestedObject"`
	}](t, func() r.PropertyMap {
		return r.PropertyMap{
			"nested": r.MakeSecret(r.NewObjectProperty(r.PropertyMap{
				"value": r.MakeSecret(r.NewArrayProperty([]r.PropertyValue{
					r.MakeSecret(r.NewBoolProperty(true)),
				})),
			})),
			"nestedObject": r.MakeSecret(r.NewObjectProperty(r.PropertyMap{
				"value": r.MakeSecret(r.NewArrayProperty([]r.PropertyValue{
					r.MakeSecret(r.NewStringProperty("foo")),
				})),
			})),
		}
	})
}
