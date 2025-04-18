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
	"reflect"
	"testing"

	r "github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/archive"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/asset"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/sig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/pulumi/pulumi-go-provider/infer/types"
	rType "github.com/pulumi/pulumi-go-provider/internal/rapid/reflect"
	rResource "github.com/pulumi/pulumi-go-provider/internal/rapid/resource"
)

// testRoundTrip asserts that the result of pMap can be decoded onto T, and then
// losslessly encoded back into a property map.
func testRoundTrip[T any](t *testing.T, pMap func() r.PropertyMap) {
	t.Run("", func(t *testing.T) {
		t.Parallel()
		pMap := r.FromResourcePropertyValue(r.NewProperty(pMap())).AsMap()
		encoder, typeInfo, err := Decode[T](pMap)
		require.NoError(t, err)

		reEncoded, err := encoder.Encode(typeInfo)
		require.NoError(t, err)
		assert.Equal(t, pMap, r.FromResourcePropertyValue(r.NewProperty(reEncoded)).AsMap())
	})
}

func TestRapidRoundTrip(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		typed := rResource.ValueOf(rType.Struct(5)).Draw(t, "top-level")
		pMap := func() r.PropertyMap { return typed.Value.ObjectValue().Copy() }
		goValue := reflect.New(typed.Type).Interface()

		toDecode := pMap()
		encoder, err := decode(toDecode, goValue,
			false /*ignoreUnrecognized*/, false /*allowMissing*/)
		require.NoError(t, err)

		assert.Equalf(t, pMap(), toDecode, "mutated decode map")

		reEncoded, err := encoder.Encode(goValue)
		require.NoError(t, err)
		assert.Equal(t, pMap(), reEncoded)

	})
}

// Test that we round trip against our strongly typed interface.
func TestRoundtripIn(t *testing.T) {
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

func TestDecodeAssets(t *testing.T) {
	t.Parallel()

	type foo struct {
		AA types.AssetOrArchive `pulumi:"aa"`
	}

	simplify := func(v any) r.PropertyMap {
		m := r.NewPropertyMap(v)
		e := ende{}
		return e.simplify(m, reflect.TypeOf(v))
	}

	assertDecodedFoo := func(kind string, m r.PropertyMap) {
		key := r.PropertyKey(kind)

		require.True(t, m["aa"].IsObject())
		obj := m["aa"].ObjectValue()
		require.True(t, obj.HasValue(key))
		require.Len(t, obj, 1)

		require.True(t, obj[key].IsObject())
		arch := obj[key].ObjectValue()
		require.True(t, arch.HasValue("path"))
	}

	t.Run("asset", func(t *testing.T) {
		t.Parallel()

		asset := asset.Asset{
			Path: "asset://foo",
		}
		f := foo{
			AA: types.AssetOrArchive{Asset: &asset},
		}

		mNew := simplify(f)

		assertDecodedFoo(AssetSignature, mNew)
	})

	t.Run("archive", func(t *testing.T) {
		t.Parallel()

		archive := archive.Archive{
			Path: "/data",
		}
		f := foo{
			AA: types.AssetOrArchive{Archive: &archive},
		}

		mNew := simplify(f)

		assertDecodedFoo(ArchiveSignature, mNew)
	})

	type bar struct {
		Foo foo `pulumi:"foo"`
	}

	t.Run("nested", func(t *testing.T) {
		t.Parallel()

		asset := asset.Asset{
			Path: "asset://foo",
		}
		f := foo{
			AA: types.AssetOrArchive{Asset: &asset},
		}
		b := bar{Foo: f}

		mNew := simplify(b)

		require.True(t, mNew["foo"].IsObject())
		inner := mNew["foo"].ObjectValue()
		assertDecodedFoo(AssetSignature, inner)
	})
}

func TestEncodeAsset(t *testing.T) {
	t.Parallel()

	t.Run("standard asset", func(t *testing.T) {
		t.Parallel()

		a, err := asset.FromText("pulumi")
		require.NoError(t, err)
		aa := types.AssetOrArchive{Asset: a}

		encoder := Encoder{new(ende)}

		properties, err := encoder.Encode(aa)
		require.NoError(t, err)

		assert.Equal(t,
			r.PropertyMap{
				sig.Key: r.NewStringProperty(sig.AssetSig),
				"hash":  r.NewStringProperty(a.Hash),
				"text":  r.NewStringProperty("pulumi"),
				"path":  r.NewStringProperty(""),
				"uri":   r.NewStringProperty(""),
			},
			properties)
	})

	t.Run("standard archive", func(t *testing.T) {
		t.Parallel()

		a, err := archive.FromPath(t.TempDir())
		require.NoError(t, err)
		aa := types.AssetOrArchive{Archive: a}

		encoder := Encoder{new(ende)}

		properties, err := encoder.Encode(aa)
		require.NoError(t, err)

		assert.Equal(t,
			r.PropertyMap{
				sig.Key: r.NewStringProperty(sig.ArchiveSig),
				"hash":  r.NewStringProperty(a.Hash),
				"path":  r.NewStringProperty(a.Path),
				"uri":   r.NewStringProperty(""),
			},
			properties)
	})

	t.Run("invalid AssetOrArchive with archive and asset", func(t *testing.T) {
		t.Parallel()

		a, err := asset.FromText("pulumi")
		require.NoError(t, err)

		b, err := archive.FromPath(t.TempDir())
		require.NoError(t, err)

		aa := types.AssetOrArchive{
			Asset:   a,
			Archive: b,
		}

		encoder := Encoder{new(ende)}

		assert.Panics(t, func() {
			_, _ = encoder.Encode(aa)
		})
	})
}
