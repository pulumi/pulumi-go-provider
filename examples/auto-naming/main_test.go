// Copyright 2025, Pulumi Corporation.  All rights reserved.

package main

import (
	"strings"
	"testing"

	"github.com/blang/semver"
	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAutoName(t *testing.T) {
	t.Parallel()

	s, err := integration.NewServer(t.Context(),
		"autoname",
		semver.MustParse("0.1.0"),
		integration.WithProvider(provider()),
	)
	require.NoError(t, err)

	tests := []struct {
		name     string
		state    property.Map
		inputs   property.Map
		validate func(*testing.T, property.Map)
	}{
		{
			name:   "create new auto-named resource",
			inputs: property.Map{},
			validate: func(t *testing.T, result property.Map) {
				name, ok := result.GetOk("name")
				require.True(t, ok, "could not find name in %q", result)
				assert.True(t, strings.HasPrefix(name.AsString(), "name-"))
				assert.Len(t, name.AsString(), 11)
				assert.Equal(t, 1, result.Len())
			},
		},
		{
			name: "create new manually named resource",
			inputs: property.NewMap(map[string]property.Value{
				"name": property.New("custom-name"),
			}),
			validate: func(t *testing.T, result property.Map) {
				name, ok := result.GetOk("name")
				require.True(t, ok, "could not find name in %q", result)
				assert.Equal(t, "custom-name", name.AsString())
			},
		},
		{
			name:   "update an auto-named resource (same name)",
			inputs: property.Map{},
			state: property.NewMap(map[string]property.Value{
				"name": property.New("name-123456"),
			}),
			validate: func(t *testing.T, result property.Map) {
				name, ok := result.GetOk("name")
				require.True(t, ok, "could not find name in %q", result)
				assert.Equal(t, "name-123456", name.AsString())
			},
		},
		{
			name: "update an auto-named resource (new name)",
			inputs: property.NewMap(map[string]property.Value{
				"name": property.New("custom-name"),
			}),
			state: property.NewMap(map[string]property.Value{
				"name": property.New("name-123456"),
			}),
			validate: func(t *testing.T, result property.Map) {
				name, ok := result.GetOk("name")
				require.True(t, ok, "could not find name in %q", result)
				assert.Equal(t, "custom-name", name.AsString())
			},
		},
		{
			name: "update a manually named resource (same name)",
			inputs: property.NewMap(map[string]property.Value{
				"name": property.New("custom-name"),
			}),
			state: property.NewMap(map[string]property.Value{
				"name": property.New("custom-name"),
			}),
			validate: func(t *testing.T, result property.Map) {
				name, ok := result.GetOk("name")
				require.True(t, ok, "could not find name in %q", result)
				assert.Equal(t, "custom-name", name.AsString())
			},
		},
		{
			name: "update a manually named resource (different name)",
			inputs: property.NewMap(map[string]property.Value{
				"name": property.New("custom-name1"),
			}),
			state: property.NewMap(map[string]property.Value{
				"name": property.New("custom-name2"),
			}),
			validate: func(t *testing.T, result property.Map) {
				name, ok := result.GetOk("name")
				require.True(t, ok, "could not find name in %q", result)
				assert.Equal(t, "custom-name1", name.AsString())
			},
		},
		{
			name:   "convert from a named to an auto-named resource",
			inputs: property.Map{},
			state: property.NewMap(map[string]property.Value{
				"name": property.New("custom-name"),
			}),
			validate: func(t *testing.T, result property.Map) {
				t.Skipf("It's not possible to drop custom-named resources without a side channel")
				name, ok := result.GetOk("name")
				require.True(t, ok, "could not find name in %q", result)

				assert.True(t, strings.HasPrefix(name.AsString(), "name-"))
				assert.Len(t, name.AsString(), 11)
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
				Urn:    resource.NewURN("dev", "test", "", "autoname:index:User", "name"),
				Inputs: tt.inputs,
				State:  tt.state,
			})
			require.NoError(t, err)
			require.Empty(t, resp.Failures)
			tt.validate(t, resp.Inputs)
		})
	}
}
