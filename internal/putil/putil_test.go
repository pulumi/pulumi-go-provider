// Copyright 2024, Pulumi Corporation.
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

package putil_test

import (
	"testing"

	"github.com/pulumi/pulumi-go-provider/internal/putil"
	rresource "github.com/pulumi/pulumi-go-provider/internal/rapid/resource"
	r "github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/urn"
	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

func TestRapidDeepEqual(t *testing.T) {
	t.Parallel()

	t.Run("reflexivity", rapid.MakeCheck(func(t *rapid.T) { //nolint:paralleltest
		value := rresource.PropertyValue(5).Draw(t, "value")
		assert.True(t, putil.DeepEquals(value, value))
	}))

	// Check that "distinct" values never equal themselves.
	t.Run("distinct-values", rapid.MakeCheck(func(t *rapid.T) { //nolint:paralleltest
		// keyFn is how [rapid] determines that values are distinct.
		//
		// We do this by calling [fmt.Stringer.String] on the normalized form of v.
		keyFn := func(v r.PropertyValue) string { return normalize(v).String() }
		values := rapid.SliceOfNDistinct(
			rresource.PropertyValue(5), 2, 2, keyFn,
		).Draw(t, "distinct")
		assert.False(t, putil.DeepEquals(values[0], values[1]))
	}))

	t.Run("nested-outputs", func(t *testing.T) {
		t.Parallel()

		v1 := r.NewProperty(r.Output{
			Element: r.NewProperty(r.Output{
				Element: r.PropertyValue{V: interface{}(nil)},
				Known:   false,
				Secret:  false,
				Dependencies: []urn.URN{
					"",
					"&\n[?", "\x7f=&\u202c\x7fᾩ:AȺ~",
					"!aAὂ̥\ue007~a\x01?۵%",
					"*\"\u2005\ue005-a=\v\u2008䍨~ᶞ٣_",
				},
			}),
			Known:  true,
			Secret: true,
			Dependencies: []urn.URN{
				"",
				"A#@\u008b%\x00$\u202e\U000e006a|ᛮ<:ፘ.᭴ो〦\U000fd7da!𞥋\v%\x7fڌ֝[A{ॊⅣ\nA1|_\u3000%a",
				"%",
			},
		})
		v2 := r.NewProperty(r.Output{
			Element: r.PropertyValue{V: interface{}(nil)},
			Known:   false,
			Secret:  true,
			Dependencies: []urn.URN{
				"",
				"̿A#꙲ .\u2029\u1680~;,*\ue000֍\ue001º`a貦",
				"_¦?!",
				"\u2007�",
				";\U000e003e\r𞥋\\𞓶\x02٦ǈ$=1𝛜ᾫ?",
				"{Aັ\\ǅ",
				"̆aǈⅺ*~",
			},
		})

		assert.True(t, putil.DeepEquals(v1, v2))
	})

	t.Run("folding-secret-computed", func(t *testing.T) {
		t.Parallel()

		assert.True(t, putil.DeepEquals(
			r.MakeComputed(r.MakeSecret(r.NewStringProperty("hi"))),
			r.MakeSecret(r.MakeComputed(r.NewStringProperty("hi")))))
		assert.False(t, putil.DeepEquals(
			r.MakeSecret(r.NewStringProperty("hi")),
			r.MakeComputed(r.NewStringProperty("hi"))))
	})
}

func normalize(p r.PropertyValue) r.PropertyValue {
	return r.ToResourcePropertyValue(r.FromResourcePropertyValue(p))
}

func TestParseProviderReference(t *testing.T) {
	t.Parallel()

	t.Run("valid", func(t *testing.T) {
		t.Parallel()

		type testCase struct {
			ref string
			urn r.URN
			id  r.ID
		}
		testCases := []testCase{
			{
				ref: "urn:pulumi:test::test::pulumi:providers:test::my-provider::09e6d266-58b0-4452-8395-7bbe03011fad",
				urn: r.URN("urn:pulumi:test::test::pulumi:providers:test::my-provider"),
				id:  r.ID("09e6d266-58b0-4452-8395-7bbe03011fad"),
			},
		}

		for _, tc := range testCases {
			urn, id, err := putil.ParseProviderReference(tc.ref)
			assert.NoError(t, err)
			assert.Equal(t, tc.urn, urn)
			assert.Equal(t, tc.id, id)
		}
	})

	t.Run("invalid", func(t *testing.T) {
		t.Parallel()

		testCases := []string{
			"p1",
			"not::a:valid:urn::id",
			"urn:pulumi:test::test::pulumi:providers:test::my-provider",
		}

		for _, tc := range testCases {
			_, _, err := putil.ParseProviderReference(tc)
			assert.Error(t, err, "expected an invalid reference: %s", tc)
		}
	})
}
