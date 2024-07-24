package main

import (
	"strings"
	"testing"

	"github.com/blang/semver"
	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAutoName(t *testing.T) {
	t.Parallel()

	s := integration.NewServer("autoname", semver.MustParse("0.1.0"), provider())

	tests := []struct {
		name     string
		olds     resource.PropertyMap
		news     resource.PropertyMap
		validate func(*testing.T, resource.PropertyMap)
	}{
		{
			name: "create new auto-named resource",
			news: resource.PropertyMap{},
			validate: func(t *testing.T, result resource.PropertyMap) {
				name, ok := result["name"]
				require.True(t, ok, "could not find name in %q", result)
				assert.True(t, strings.HasPrefix(name.StringValue(), "name-"))
				assert.Len(t, name.StringValue(), 11)
				assert.Len(t, result, 1)
			},
		},
		{
			name: "create new manually named resource",
			news: resource.PropertyMap{
				"name": resource.NewProperty("custom-name"),
			},
			validate: func(t *testing.T, result resource.PropertyMap) {
				name, ok := result["name"]
				require.True(t, ok, "could not find name in %q", result)
				assert.Equal(t, "custom-name", name.StringValue())
			},
		},
		{
			name: "update an auto-named resource (same name)",
			news: resource.PropertyMap{},
			olds: resource.PropertyMap{
				"name": resource.NewProperty("name-123456"),
			},
			validate: func(t *testing.T, result resource.PropertyMap) {
				name, ok := result["name"]
				require.True(t, ok, "could not find name in %q", result)
				assert.Equal(t, "name-123456", name.StringValue())
			},
		},
		{
			name: "update an auto-named resource (new name)",
			news: resource.PropertyMap{
				"name": resource.NewProperty("custom-name"),
			},
			olds: resource.PropertyMap{
				"name": resource.NewProperty("name-123456"),
			},
			validate: func(t *testing.T, result resource.PropertyMap) {
				name, ok := result["name"]
				require.True(t, ok, "could not find name in %q", result)
				assert.Equal(t, "custom-name", name.StringValue())
			},
		},
		{
			name: "update a manually named resource (same name)",
			news: resource.PropertyMap{
				"name": resource.NewProperty("custom-name"),
			},
			olds: resource.PropertyMap{
				"name": resource.NewProperty("custom-name"),
			},
			validate: func(t *testing.T, result resource.PropertyMap) {
				name, ok := result["name"]
				require.True(t, ok, "could not find name in %q", result)
				assert.Equal(t, "custom-name", name.StringValue())
			},
		},
		{
			name: "update a manually named resource (different name)",
			news: resource.PropertyMap{
				"name": resource.NewProperty("custom-name1"),
			},
			olds: resource.PropertyMap{
				"name": resource.NewProperty("custom-name2"),
			},
			validate: func(t *testing.T, result resource.PropertyMap) {
				name, ok := result["name"]
				require.True(t, ok, "could not find name in %q", result)
				assert.Equal(t, "custom-name1", name.StringValue())
			},
		},
		{
			name: "convert from a named to an auto-named resource",
			news: resource.PropertyMap{},
			olds: resource.PropertyMap{
				"name": resource.NewProperty("custom-name"),
			},
			validate: func(t *testing.T, result resource.PropertyMap) {
				t.Skipf("It's not possible to drop custom-named resources without a side channel")
				name, ok := result["name"]
				require.True(t, ok, "could not find name in %q", result)

				assert.True(t, strings.HasPrefix(name.StringValue(), "name-"))
				assert.Len(t, name.StringValue(), 11)
				assert.Len(t, result, 1)
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			t.Helper()
			resp, err := s.Check(p.CheckRequest{
				Urn:  resource.NewURN("dev", "test", "", "autoname:index:User", "name"),
				News: tt.news,
				Olds: tt.olds,
			})
			require.NoError(t, err)
			require.Empty(t, resp.Failures)
			tt.validate(t, resp.Inputs)
		})
	}
}
