// Copyright 2026, Pulumi Corporation.
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
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi-go-provider/integration"
)

type (
	AutoNamedResource struct{}
	AutoNamedBase     struct {
		Name string `pulumi:"name"`
	}
	AutoNamedArgs struct {
		AutoNamedBase
		Other string `pulumi:"other"`
	}
	AutoNamedState struct{ AutoNamedArgs }
)

// AutoName names a promoted field and a direct field. Name lower-cases its names and
// rejects names longer than 20 characters.
func (a *AutoNamedArgs) AutoName(autoname infer.SetAutoName) {
	autoname(&a.Name, infer.AutoNameConfig{
		Adopt: func(_ context.Context, args infer.AutoNameAdoptArgs) (string, error) {
			if len(args.Proposed) > 20 {
				return "", fmt.Errorf("%q is longer than 20 characters", args.Proposed)
			}
			return strings.ToLower(args.Proposed), nil
		},
	})
	autoname(&a.Other, infer.AutoNameConfig{})
}

func (*AutoNamedResource) Create(
	_ context.Context, req infer.CreateRequest[AutoNamedArgs],
) (infer.CreateResponse[AutoNamedState], error) {
	return infer.CreateResponse[AutoNamedState]{ID: "id", Output: AutoNamedState{req.Inputs}}, nil
}

// AutoNamedCustomCheck is auto-named through a call to [infer.DefaultCheck].
type AutoNamedCustomCheck struct{}

func (*AutoNamedCustomCheck) Check(
	ctx context.Context, req infer.CheckRequest,
) (infer.CheckResponse[AutoNamedArgs], error) {
	args, failures, err := infer.DefaultCheck[AutoNamedArgs](ctx, req.NewInputs)
	return infer.CheckResponse[AutoNamedArgs]{Inputs: args, Failures: failures}, err
}

func (*AutoNamedCustomCheck) Create(
	_ context.Context, req infer.CreateRequest[AutoNamedArgs],
) (infer.CreateResponse[AutoNamedState], error) {
	return infer.CreateResponse[AutoNamedState]{ID: "id", Output: AutoNamedState{req.Inputs}}, nil
}

func autoNameOpts(resources ...infer.InferredResource) infer.Options {
	return infer.Options{
		Resources: resources,
		ModuleMap: map[tokens.ModuleName]tokens.ModuleName{"tests": "index"},
	}
}

func autoNameProvider(t *testing.T) integration.Server {
	s, err := integration.NewServer(t.Context(), "test", semver.MustParse("1.0.0"),
		integration.WithProvider(infer.Provider(autoNameOpts(
			infer.Resource(&AutoNamedResource{}),
			infer.Resource(&AutoNamedCustomCheck{}),
		))))
	require.NoError(t, err)
	return s
}

func TestAutoNameCheck(t *testing.T) {
	t.Parallel()

	type m = map[string]property.Value
	seed := []byte("seed")

	tests := []struct {
		name       string
		inputs     property.Map
		state      property.Map
		autonaming *p.AutonamingOptions
		expected   p.CheckResponse
		// A custom Check encodes the decoded inputs, even when there are failures.
		customCheckInputs *property.Map
	}{
		{
			name: "user-values",
			inputs: property.NewMap(m{
				"name":  property.New("User-Name"),
				"other": property.New("user-other"),
			}),
			expected: p.CheckResponse{Inputs: property.NewMap(m{
				"name":  property.New("User-Name"),
				"other": property.New("user-other"),
			})},
		},
		{
			name: "generated",
			expected: p.CheckResponse{Inputs: property.NewMap(m{
				"name":  property.New("my-res-9f3d0b0a"),
				"other": property.New("My-Res-d719cd82"),
			})},
		},
		{
			name: "old-values",
			state: property.NewMap(m{
				"name":  property.New("old-name"),
				"other": property.New("old-other"),
			}),
			autonaming: &p.AutonamingOptions{Mode: p.AutonamingDisable},
			expected: p.CheckResponse{Inputs: property.NewMap(m{
				"name":  property.New("old-name"),
				"other": property.New("old-other"),
			})},
		},
		{
			name:       "propose",
			autonaming: &p.AutonamingOptions{ProposedName: "Proposed-Name", Mode: p.AutonamingPropose},
			expected: p.CheckResponse{Inputs: property.NewMap(m{
				"name":  property.New("proposed-name"),
				"other": property.New("Proposed-Name"),
			})},
		},
		{
			name: "propose-rejected",
			autonaming: &p.AutonamingOptions{
				ProposedName: "Proposed-Name-That-Is-Too-Long", Mode: p.AutonamingPropose,
			},
			expected: p.CheckResponse{
				Inputs: property.Map{},
				Failures: []p.CheckFailure{{
					Property: "name",
					Reason:   `"Proposed-Name-That-Is-Too-Long" is longer than 20 characters`,
				}},
			},
			customCheckInputs: new(property.NewMap(m{
				"name":  property.New(""),
				"other": property.New("Proposed-Name-That-Is-Too-Long"),
			})),
		},
		{
			name: "enforce",
			autonaming: &p.AutonamingOptions{
				ProposedName: "Enforced-Name-That-Is-Too-Long", Mode: p.AutonamingEnforce,
			},
			expected: p.CheckResponse{Inputs: property.NewMap(m{
				"name":  property.New("Enforced-Name-That-Is-Too-Long"),
				"other": property.New("Enforced-Name-That-Is-Too-Long"),
			})},
		},
		{
			name:       "disable",
			autonaming: &p.AutonamingOptions{Mode: p.AutonamingDisable},
			expected: p.CheckResponse{
				Inputs: property.Map{},
				Failures: []p.CheckFailure{
					{Property: "name", Reason: "auto-naming is disabled, so a value is required"},
					{Property: "other", Reason: "auto-naming is disabled, so a value is required"},
				},
			},
			customCheckInputs: new(property.NewMap(m{
				"name":  property.New(""),
				"other": property.New(""),
			})),
		},
	}

	for _, typ := range []string{"AutoNamedResource", "AutoNamedCustomCheck"} {
		for _, tt := range tests {
			t.Run(typ+"/"+tt.name, func(t *testing.T) {
				t.Parallel()
				resp, err := autoNameProvider(t).Check(p.CheckRequest{
					Urn:        urn(typ, "My-Res"),
					State:      tt.state,
					Inputs:     tt.inputs,
					RandomSeed: seed,
					Autonaming: tt.autonaming,
				})
				require.NoError(t, err)
				expected := tt.expected
				if typ == "AutoNamedCustomCheck" && tt.customCheckInputs != nil {
					expected.Inputs = *tt.customCheckInputs
				}
				assert.Equal(t, expected, resp)
			})
		}
	}
}

func TestAutoNameSchema(t *testing.T) {
	t.Parallel()

	resp, err := autoNameProvider(t).GetSchema(p.GetSchemaRequest{})
	require.NoError(t, err)

	var spec struct {
		Resources map[string]struct {
			Required       []string `json:"required"`
			RequiredInputs []string `json:"requiredInputs"`
		} `json:"resources"`
	}
	require.NoError(t, json.Unmarshal([]byte(resp.Schema), &spec))

	res := spec.Resources["test:index:AutoNamedResource"]
	assert.Equal(t, []string{"name", "other"}, res.Required)
	assert.Empty(t, res.RequiredInputs)
}

type (
	AutoNamedNested     struct{}
	AutoNamedNestedArgs struct {
		Inner AutoNamedArgs `pulumi:"inner"`
	}
)

func (*AutoNamedNested) Create(
	_ context.Context, req infer.CreateRequest[AutoNamedNestedArgs],
) (infer.CreateResponse[AutoNamedNestedArgs], error) {
	return infer.CreateResponse[AutoNamedNestedArgs]{ID: "id", Output: req.Inputs}, nil
}

func TestAutoNameRejectsNested(t *testing.T) {
	t.Parallel()

	s, err := integration.NewServer(t.Context(), "test", semver.MustParse("1.0.0"),
		integration.WithProvider(infer.Provider(autoNameOpts(infer.Resource(&AutoNamedNested{})))))
	require.NoError(t, err)

	_, err = s.GetSchema(p.GetSchemaRequest{})
	assert.EqualError(t, err, "1 error occurred:\n\t* failed to get schema for 'test:index:AutoNamedNested': "+
		"tests.AutoNamedNestedArgs.Inner: nested type tests.AutoNamedArgs cannot implement AutoNamed\n\n")
}

func TestAutoNameHandshake(t *testing.T) {
	t.Parallel()

	handshake := func(t *testing.T, opts infer.Options) *pulumirpc.ProviderHandshakeResponse {
		server, err := p.RawServer("test", "1.0.0", infer.Provider(opts))(nil)
		require.NoError(t, err)
		resp, err := server.Handshake(t.Context(), &pulumirpc.ProviderHandshakeRequest{})
		require.NoError(t, err)
		return resp
	}

	t.Run("auto-named", func(t *testing.T) {
		t.Parallel()
		resp := handshake(t, autoNameOpts(infer.Resource(&AutoNamedResource{}), infer.Resource(&Echo{})))
		assert.True(t, resp.GetSupportsAutonamingConfiguration())
	})

	t.Run("not-auto-named", func(t *testing.T) {
		t.Parallel()
		resp := handshake(t, autoNameOpts(infer.Resource(&Echo{})))
		assert.False(t, resp.GetSupportsAutonamingConfiguration())
	})
}
