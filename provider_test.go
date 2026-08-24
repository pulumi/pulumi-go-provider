// Copyright 2022-2025, Pulumi Corporation.
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

package provider

import (
	"context"
	"testing"

	presource "github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	pconfig "github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	rpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"

	internalrpc "github.com/pulumi/pulumi-go-provider/internal/rpc"
)

func TestMarshalPreservesNulls(t *testing.T) {
	t.Parallel()
	p := &provider{}
	m := property.NewMap(map[string]property.Value{
		"null":  {},
		"value": property.New("foo"),
	})

	s, err := p.asStruct(m)
	require.NoError(t, err)

	round, err := p.getMap(s)
	require.NoError(t, err)
	assert.Equal(t, m, round)
}

func TestDiffResponseRPC(t *testing.T) {
	t.Parallel()
	t.Run("nested property paths are flattened to top-level keys", func(t *testing.T) {
		t.Parallel()
		resp := DiffResponse{
			HasChanges: true,
			DetailedDiff: map[string]PropertyDiff{
				"foo.bar.baz": {Kind: Update},
				"foo.qux":     {Kind: Update},
				"other":       {Kind: Update},
			},
		}

		rpcResp := resp.rpc()

		// Both "foo.bar.baz" and "foo.qux" should map to "foo"
		assert.ElementsMatch(t, []string{"foo", "other"}, rpcResp.Diffs)
		assert.Empty(t, rpcResp.Replaces)
		assert.Empty(t, rpcResp.Stables)
	})

	t.Run("different diff kinds populate correct fields", func(t *testing.T) {
		t.Parallel()
		resp := DiffResponse{
			HasChanges: true,
			DetailedDiff: map[string]PropertyDiff{
				"add":           {Kind: Add},
				"addReplace":    {Kind: AddReplace},
				"delete":        {Kind: Delete},
				"deleteReplace": {Kind: DeleteReplace},
				"update":        {Kind: Update},
				"updateReplace": {Kind: UpdateReplace},
				"stable":        {Kind: Stable},
			},
		}

		rpcResp := resp.rpc()

		// All kinds except Stable should be in Diffs
		assert.ElementsMatch(t, []string{
			"add", "addReplace", "delete", "deleteReplace", "update", "updateReplace",
		}, rpcResp.Diffs)

		// Only replace kinds should be in Replaces
		assert.ElementsMatch(t, []string{
			"addReplace", "deleteReplace", "updateReplace",
		}, rpcResp.Replaces)

		// Only Stable should be in Stables
		assert.ElementsMatch(t, []string{"stable"}, rpcResp.Stables)
	})

	t.Run("nested paths with replace kinds", func(t *testing.T) {
		t.Parallel()
		resp := DiffResponse{
			HasChanges: true,
			DetailedDiff: map[string]PropertyDiff{
				"resource.property.nested":  {Kind: UpdateReplace},
				"resource.other":            {Kind: Update},
				"different.path":            {Kind: AddReplace},
				"different.another.deep[0]": {Kind: Delete},
			},
		}

		rpcResp := resp.rpc()

		// "resource.property.nested" and "resource.other" -> "resource"
		// "different.path" and "different.another.deep[0]" -> "different"
		assert.ElementsMatch(t, []string{"resource", "different"}, rpcResp.Diffs)
		assert.ElementsMatch(t, []string{"resource", "different"}, rpcResp.Replaces)
		assert.Empty(t, rpcResp.Stables)
	})

	t.Run("empty detailed diff returns nil slices", func(t *testing.T) {
		t.Parallel()
		resp := DiffResponse{
			HasChanges:   true,
			DetailedDiff: map[string]PropertyDiff{},
		}

		rpcResp := resp.rpc()

		assert.Nil(t, rpcResp.Diffs)
		assert.Nil(t, rpcResp.Replaces)
		assert.Nil(t, rpcResp.Stables)
	})
}

func TestGetSchema(t *testing.T) {
	t.Parallel()

	t.Run("logged errors are included in returned error", func(t *testing.T) {
		t.Parallel()
		provider := Provider{
			GetSchema: func(ctx context.Context, req GetSchemaRequest) (GetSchemaResponse, error) {
				logger := GetLogger(ctx)
				logger.Error("first error")
				logger.Error("second error")
				return GetSchemaResponse{Schema: `{"name":"test"}`}, nil
			},
		}

		_, err := GetSchema(t.Context(), "test", "1.0.0", provider)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "first error")
		assert.Contains(t, err.Error(), "second error")
	})

	t.Run("GetSchema function error is included in returned error", func(t *testing.T) {
		t.Parallel()
		provider := Provider{
			GetSchema: func(ctx context.Context, req GetSchemaRequest) (GetSchemaResponse, error) {
				return GetSchemaResponse{}, assert.AnError
			},
		}

		_, err := GetSchema(t.Context(), "test", "1.0.0", provider)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "GetSchema failed")
		assert.ErrorIs(t, err, assert.AnError)
	})

	t.Run("both logged errors and function error are included", func(t *testing.T) {
		t.Parallel()
		provider := Provider{
			GetSchema: func(ctx context.Context, req GetSchemaRequest) (GetSchemaResponse, error) {
				logger := GetLogger(ctx)
				logger.Error("logged error")
				return GetSchemaResponse{}, assert.AnError
			},
		}

		_, err := GetSchema(t.Context(), "test", "1.0.0", provider)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "logged error")
		assert.Contains(t, err.Error(), "GetSchema failed")
		assert.ErrorIs(t, err, assert.AnError)
	})

	t.Run("GetRunInfo is accessible in GetSchema", func(t *testing.T) {
		t.Parallel()
		var capturedRunInfo RunInfo
		provider := Provider{
			GetSchema: func(ctx context.Context, req GetSchemaRequest) (GetSchemaResponse, error) {
				capturedRunInfo = GetRunInfo(ctx)
				return GetSchemaResponse{Schema: `{"name":"test"}`}, nil
			},
		}

		_, err := GetSchema(t.Context(), "test-package", "2.3.4", provider)
		require.NoError(t, err)
		assert.Equal(t, RunInfo{
			PackageName:       "test-package",
			Version:           "2.3.4",
			SupportsOldInputs: false,
		}, capturedRunInfo)
	})

	t.Run("non-error logs are not included in error", func(t *testing.T) {
		t.Parallel()
		provider := Provider{
			GetSchema: func(ctx context.Context, req GetSchemaRequest) (GetSchemaResponse, error) {
				logger := GetLogger(ctx)
				logger.Info("info message")
				logger.Warning("warning message")
				logger.Debug("debug message")
				return GetSchemaResponse{Schema: `{"name":"test"}`}, nil
			},
		}

		_, err := GetSchema(t.Context(), "test", "1.0.0", provider)
		require.NoError(t, err)
	})

	t.Run("success with valid schema", func(t *testing.T) {
		t.Parallel()
		provider := Provider{
			GetSchema: func(ctx context.Context, req GetSchemaRequest) (GetSchemaResponse, error) {
				return GetSchemaResponse{Schema: `{"name":"mypackage","version":"1.0.0"}`}, nil
			},
		}

		spec, err := GetSchema(t.Context(), "test", "1.0.0", provider)
		require.NoError(t, err)
		assert.Equal(t, "mypackage", spec.Name)
		assert.Equal(t, "1.0.0", spec.Version)
	})
}

// TestConstructRequestRPC verifies the proto ConstructRequest translates into
// the public ConstructRequest — including the execution-context and
// resource-option fields the engine sends — and that the reverse translation
// loses none of it: a request round-tripped through rpc() reads back equal.
func TestConstructRequestRPC(t *testing.T) {
	t.Parallel()

	req := &rpc.ConstructRequest{
		Project:          "proj",
		Stack:            "stack",
		Type:             "pkg:index:Comp",
		Name:             "comp",
		Organization:     "acme",
		StackTraceHandle: "handle",
		ReplaceWith:      []string{"urn:pulumi:stack::proj::pkg:index:Other::sibling"},
		Aliases: []*rpc.Alias{
			{Alias: &rpc.Alias_Urn{Urn: "urn:pulumi:stack::proj::pkg:index:Comp::old"}},
			{Alias: &rpc.Alias_Spec_{Spec: &rpc.Alias_Spec{
				Type:   "pkg:index:Old",
				Name:   "old",
				Parent: &rpc.Alias_Spec_NoParent{NoParent: true},
			}}},
			{Alias: &rpc.Alias_Spec_{Spec: &rpc.Alias_Spec{
				Type:   "pkg:index:Old",
				Parent: &rpc.Alias_Spec_ParentUrn{ParentUrn: "urn:pulumi:stack::proj::pkg:index:Parent::p"},
			}}},
		},
		ResourceHooks: &rpc.ConstructRequest_ResourceHooksBinding{
			BeforeCreate: []string{"bc"},
			AfterCreate:  []string{"ac"},
			BeforeUpdate: []string{"bu"},
			AfterUpdate:  []string{"au"},
			BeforeDelete: []string{"bd"},
			AfterDelete:  []string{"ad"},
			OnError:      []string{"oe"},
		},
		ReplacementTrigger:  structpb.NewStringValue("trigger"),
		AcceptsOutputValues: true,
	}

	got, err := newConstructRequest(req, internalrpc.UnmarshalProperties)
	require.NoError(t, err)

	assert.Equal(t, ConstructRequest{
		Urn:              presource.NewURN("stack", "proj", "", "pkg:index:Comp", "comp"),
		Config:           map[pconfig.Key]string{},
		ConfigSecretKeys: []pconfig.Key{},
		MonitorEndpoint:  "",
		Inputs:           property.Map{},
		Providers:        map[tokens.Package]ProviderReference{},
		Aliases: []presource.Alias{
			{URN: "urn:pulumi:stack::proj::pkg:index:Comp::old"},
			{Type: "pkg:index:Old", Name: "old", NoParent: true},
			{Type: "pkg:index:Old", Parent: "urn:pulumi:stack::proj::pkg:index:Parent::p"},
		},
		Dependencies:            []presource.URN{},
		AdditionalSecretOutputs: []string{},
		Organization:            "acme",
		StackTraceHandle:        "handle",
		ReplaceWith:             []presource.URN{"urn:pulumi:stack::proj::pkg:index:Other::sibling"},
		ResourceHooks: &ResourceHooksBinding{
			BeforeCreate: []string{"bc"},
			AfterCreate:  []string{"ac"},
			BeforeUpdate: []string{"bu"},
			AfterUpdate:  []string{"au"},
			BeforeDelete: []string{"bd"},
			AfterDelete:  []string{"ad"},
			OnError:      []string{"oe"},
		},
		ReplacementTrigger: property.New("trigger"),
	}, got)

	// The reverse translation must preserve everything the forward translation
	// reads: converting back to a proto request and re-parsing it yields the
	// same request.
	again, err := newConstructRequest(got.rpc(internalrpc.MarshalProperties), internalrpc.UnmarshalProperties)
	require.NoError(t, err)
	assert.Equal(t, got, again)
}
