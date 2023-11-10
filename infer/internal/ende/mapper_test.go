package ende

import (
	"reflect"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
			Uri  pulumi.Asset `pulumi:"uri"`
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
		mErr := decodeProperty(initialMap, reflect.ValueOf(target), mapperOpts{})
		require.NoError(t, mErr)

		assert.Equal(t, text, target.Text.Text())
		assert.Equal(t, uri, target.Uri.URI())
		assert.Equal(t, path, target.Path.Path())

		actualMap, err := encodeProperty(target, mapperOpts{})
		require.NoError(t, err)
		delete(initialMap, "optional")
		assert.Equal(t, initialMap, actualMap)
	})
}
