// Copyright 2023, Pulumi Corporation.  All rights reserved.

package ende_test

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi-go-provider/infer/internal/ende"
)

func TestEnDeValue(t *testing.T) {
	t.Parallel()
	t.Run("asset", func(t *testing.T) {
		t.Parallel()

		text := "some-text"
		textAsset, err := resource.NewTextAsset(text)
		require.NoError(t, err)

		uri := "https://example.com/uri"
		uriAsset, err := resource.NewURIAsset(uri)
		require.NoError(t, err)

		path := "./mapper_test.go"
		pathAsset, err := resource.NewPathAsset(path)
		require.NoError(t, err)

		type hasAsset struct {
			Text pulumi.Asset `pulumi:"text"`
			URI  pulumi.Asset `pulumi:"uri"`
			Path pulumi.Asset `pulumi:"path"`

			Optional pulumi.Asset `pulumi:"optional,optional"`
		}

		initialMap := resource.PropertyMap{
			"text": resource.NewAssetProperty(textAsset),
			"uri":  resource.NewAssetProperty(uriAsset),
			"path": resource.NewAssetProperty(pathAsset),

			"optional": resource.NewNullProperty(),
		}

		target := new(hasAsset)
		e, mErr := ende.Decode(initialMap, target)
		require.NoError(t, mErr)

		assert.Equal(t, text, target.Text.Text())
		assert.Equal(t, uri, target.URI.URI())
		assert.Equal(t, path, target.Path.Path())

		actualMap, err := e.Encode(target)
		require.NoError(t, err)
		delete(initialMap, "optional")
		assert.Equal(t, initialMap, actualMap)
	})

	t.Run("output", func(t *testing.T) {
		t.Parallel()

		type nested struct {
			N string  `pulumi:"n"`
			F float64 `pulumi:"f"`
		}

		s := "some string"
		i := float64(42)
		m := map[string]any{"yes": true, "no": false}
		a := []string{"zero", "one", "two"}
		n := nested{
			N: "nested string",
			F: 1.2,
		}

		type hasOutputs struct {
			S infer.Output[string]          `pulumi:"s"`
			I infer.Output[int]             `pulumi:"i"`
			M infer.Output[map[string]bool] `pulumi:"m"`
			A infer.Output[[]string]        `pulumi:"a"`
			N infer.Output[nested]          `pulumi:"n"`
		}

		initialMap := resource.PropertyMap{
			"s": resource.NewStringProperty(s),
			"i": resource.NewNumberProperty(i),
			"m": resource.NewObjectProperty(resource.NewPropertyMapFromMap(m)),
			"a": resource.NewArrayProperty(fmap(a, resource.NewStringProperty)),
			"n": resource.NewObjectProperty(resource.NewPropertyMap(n)),
		}

		target := new(hasOutputs)

		e, mErr := ende.Decode(initialMap, target)
		require.NoError(t, mErr)

		assert.Equal(t, s, target.S.MustGetKnown())

		actualMap, err := e.Encode(target)
		require.NoError(t, err)
		assert.Equal(t, initialMap, actualMap)

	})
}

func fmap[T, U any](arr []T, f func(T) U) []U {
	out := make([]U, len(arr))
	for i, v := range arr {
		out[i] = f(v)
	}
	return out
}
