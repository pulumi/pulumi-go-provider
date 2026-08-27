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
	"testing"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/stretchr/testify/require"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi-go-provider/integration"
)

type (
	AlwaysSecretRes  struct{}
	AlwaysSecretArgs struct {
		Name string `pulumi:"name"`
	}
)

type AlwaysSecretState struct {
	AlwaysSecretArgs
	Secret string `pulumi:"secret"`
}

func (*AlwaysSecretRes) WireDependencies(
	f infer.FieldSelector, args *AlwaysSecretArgs, state *AlwaysSecretState,
) {
	f.OutputField(&state.Secret).AlwaysSecret()
}

func (*AlwaysSecretRes) Create(
	context.Context, infer.CreateRequest[AlwaysSecretArgs],
) (infer.CreateResponse[AlwaysSecretState], error) {
	return infer.CreateResponse[AlwaysSecretState]{
		ID: "id",
		Output: AlwaysSecretState{
			AlwaysSecretArgs: AlwaysSecretArgs{Name: "created"},
			Secret:           "created-secret",
		},
	}, nil
}

func (*AlwaysSecretRes) Read(
	context.Context, infer.ReadRequest[AlwaysSecretArgs, AlwaysSecretState],
) (infer.ReadResponse[AlwaysSecretArgs, AlwaysSecretState], error) {
	return infer.ReadResponse[AlwaysSecretArgs, AlwaysSecretState]{
		ID:     "id",
		Inputs: AlwaysSecretArgs{Name: "read"},
		State: AlwaysSecretState{
			AlwaysSecretArgs: AlwaysSecretArgs{Name: "read"},
			Secret:           "read-secret",
		},
	}, nil
}

// TestReadAlwaysSecret verifies that AlwaysSecret outputs are marked secret on Read
// responses, including import-like Reads with no prior state.
//
// https://github.com/pulumi/pulumi-go-provider/issues/582
func TestReadAlwaysSecret(t *testing.T) {
	t.Parallel()

	prov := infer.Provider(infer.Options{
		Resources: []infer.InferredResource{infer.Resource(&AlwaysSecretRes{})},
		ModuleMap: map[tokens.ModuleName]tokens.ModuleName{"tests": "index"},
	})

	s, err := integration.NewServer(t.Context(),
		"test", semver.MustParse("1.0.0"), integration.WithProvider(prov))
	require.NoError(t, err)

	resourceURN := urn("AlwaysSecretRes", "test")
	inputs := property.NewMap(map[string]property.Value{"name": property.New("input")})

	created, err := s.Create(p.CreateRequest{Urn: resourceURN, Properties: inputs})
	require.NoError(t, err)
	require.Equal(t, property.New("created-secret").WithSecret(true),
		created.Properties.Get("secret"))

	refreshed, err := s.Read(p.ReadRequest{
		Urn: resourceURN, ID: created.ID,
		Properties: created.Properties, Inputs: inputs,
	})
	require.NoError(t, err)
	require.Equal(t, property.New("read-secret").WithSecret(true),
		refreshed.Properties.Get("secret"),
		"Read with prior state marks AlwaysSecret outputs secret")

	imported, err := s.Read(p.ReadRequest{Urn: resourceURN, ID: created.ID})
	require.NoError(t, err)
	require.Equal(t, property.New("read-secret").WithSecret(true),
		imported.Properties.Get("secret"),
		"import-like Read with no prior state marks AlwaysSecret outputs secret")
}
