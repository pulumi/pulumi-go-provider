package main

import (
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
	provider, err := provider()
	require.NoError(t, err)

	s, err := integration.NewServer(t.Context(),
		"autoname",
		semver.MustParse("0.1.0"),
		integration.WithProvider(provider),
	)
	require.NoError(t, err)

	named := func(name string) property.Map {
		return property.NewMap(map[string]property.Value{"name": property.New(name)})
	}

	tests := []struct {
		name       string
		state      property.Map
		inputs     property.Map
		autonaming *p.AutonamingOptions
		expected   p.CheckResponse
	}{
		{
			name:     "create new auto-named resource",
			expected: p.CheckResponse{Inputs: named("name-9f3d0b0a")},
		},
		{
			name:     "create new manually named resource",
			inputs:   named("custom-name"),
			expected: p.CheckResponse{Inputs: named("custom-name")},
		},
		{
			name:     "update an auto-named resource (same name)",
			state:    named("name-123456"),
			expected: p.CheckResponse{Inputs: named("name-123456")},
		},
		{
			name:     "update an auto-named resource (new name)",
			inputs:   named("custom-name"),
			state:    named("name-123456"),
			expected: p.CheckResponse{Inputs: named("custom-name")},
		},
		{
			name:     "update a manually named resource (different name)",
			inputs:   named("custom-name1"),
			state:    named("custom-name2"),
			expected: p.CheckResponse{Inputs: named("custom-name1")},
		},
		{
			// Old inputs do not record whether a name was generated, so a name that the
			// user removes is kept.
			name:     "remove a manual name",
			state:    named("custom-name"),
			expected: p.CheckResponse{Inputs: named("custom-name")},
		},
		{
			name:       "propose a name",
			autonaming: &p.AutonamingOptions{ProposedName: "proposed-name", Mode: p.AutonamingPropose},
			expected:   p.CheckResponse{Inputs: named("proposed-name")},
		},
		{
			name: "propose a name that is too long",
			autonaming: &p.AutonamingOptions{
				ProposedName: "proposed-name-that-is-too-long", Mode: p.AutonamingPropose,
			},
			expected: p.CheckResponse{
				Inputs: property.Map{},
				Failures: []p.CheckFailure{{
					Property: "name",
					Reason:   `"proposed-name-that-is-too-long" is longer than 20 characters`,
				}},
			},
		},
		{
			name: "enforce a name that is too long",
			autonaming: &p.AutonamingOptions{
				ProposedName: "enforced-name-that-is-too-long", Mode: p.AutonamingEnforce,
			},
			expected: p.CheckResponse{Inputs: named("enforced-name-that-is-too-long")},
		},
		{
			name:       "disable auto-naming",
			autonaming: &p.AutonamingOptions{Mode: p.AutonamingDisable},
			expected: p.CheckResponse{
				Inputs: property.Map{},
				Failures: []p.CheckFailure{{
					Property: "name",
					Reason:   "auto-naming is disabled, so a value is required",
				}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			resp, err := s.Check(p.CheckRequest{
				Urn:        resource.NewURN("dev", "test", "", "autoname:index:User", "name"),
				Inputs:     tt.inputs,
				State:      tt.state,
				RandomSeed: []byte("seed"),
				Autonaming: tt.autonaming,
			})
			require.NoError(t, err)
			assert.Equal(t, tt.expected, resp)
		})
	}
}
