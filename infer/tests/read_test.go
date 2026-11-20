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
	rpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"

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
	Secret string `pulumi:"secret"` //nolint:gosec // exercises AlwaysSecret, not a credential
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

type (
	MaybeMissingRes  struct{}
	MaybeMissingArgs struct {
		Name string `pulumi:"name"`
	}
	MaybeMissingState struct {
		MaybeMissingArgs
		Out string `pulumi:"out"`
	}
)

func (*MaybeMissingRes) Create(
	context.Context, infer.CreateRequest[MaybeMissingArgs],
) (infer.CreateResponse[MaybeMissingState], error) {
	return infer.CreateResponse[MaybeMissingState]{ID: "found"}, nil
}

func (*MaybeMissingRes) Read(
	_ context.Context, req infer.ReadRequest[MaybeMissingArgs, MaybeMissingState],
) (infer.ReadResponse[MaybeMissingArgs, MaybeMissingState], error) {
	type resp = infer.ReadResponse[MaybeMissingArgs, MaybeMissingState]
	switch req.ID {
	case "missing":
		return resp{}, nil
	case "missing-echo-inputs":
		return resp{Inputs: req.Inputs}, nil
	case "no-id-with-state":
		return resp{
			Inputs: MaybeMissingArgs{Name: "read"},
			State:  MaybeMissingState{MaybeMissingArgs{Name: "read"}, "read-out"},
		}, nil
	default:
		return resp{
			ID:     req.ID,
			Inputs: MaybeMissingArgs{Name: "read"},
			State:  MaybeMissingState{MaybeMissingArgs{Name: "read"}, "read-out"},
		}, nil
	}
}

func TestReadMissingResource(t *testing.T) {
	t.Parallel()

	server, err := p.RawServer("test", "1.0.0", infer.Provider(infer.Options{
		Resources: []infer.InferredResource{infer.Resource(&MaybeMissingRes{})},
		ModuleMap: map[tokens.ModuleName]tokens.ModuleName{"tests": "index"},
	}))(nil)
	require.NoError(t, err)

	priorState, err := structpb.NewStruct(map[string]any{"name": "old", "out": "old-out"})
	require.NoError(t, err)
	priorInputs, err := structpb.NewStruct(map[string]any{"name": "old"})
	require.NoError(t, err)

	read := func(t *testing.T, id string, withPriorState bool) *rpc.ReadResponse {
		req := &rpc.ReadRequest{Id: id, Urn: string(urn("MaybeMissingRes", "r"))}
		if withPriorState {
			req.Properties = priorState
			req.Inputs = priorInputs
		}
		resp, err := server.Read(t.Context(), req)
		require.NoError(t, err)
		return resp
	}

	for _, id := range []string{"missing", "missing-echo-inputs"} {
		t.Run(id+"/get-or-import", func(t *testing.T) {
			t.Parallel()
			resp := read(t, id, false)
			assert.Empty(t, resp.GetId())
			assert.Nil(t, resp.GetProperties(),
				"nil properties make the engine report that the resource does not exist")
			assert.Nil(t, resp.GetInputs())
		})
		t.Run(id+"/refresh", func(t *testing.T) {
			t.Parallel()
			resp := read(t, id, true)
			assert.Empty(t, resp.GetId(), "an empty ID makes refresh delete the resource from state")
			assert.Nil(t, resp.GetProperties())
		})
	}

	t.Run("no-id-with-state/get-or-import", func(t *testing.T) {
		t.Parallel()
		resp := read(t, "no-id-with-state", false)
		assert.Empty(t, resp.GetId(), "the engine falls back to the requested ID")
		assert.Equal(t, map[string]any{"name": "read", "out": "read-out"}, resp.GetProperties().AsMap(),
			"a populated State is still returned when the ID is empty")
		assert.Equal(t, map[string]any{"name": "read"}, resp.GetInputs().AsMap())
	})

	t.Run("found/get-or-import", func(t *testing.T) {
		t.Parallel()
		resp := read(t, "found", false)
		assert.Equal(t, "found", resp.GetId())
		assert.Equal(t, map[string]any{"name": "read", "out": "read-out"}, resp.GetProperties().AsMap())
	})
}
