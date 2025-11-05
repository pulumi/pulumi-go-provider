// Copyright 2025, Pulumi Corporation.
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

package rpc

import (
	"context"
	"testing"

	structpb "github.com/golang/protobuf/ptypes/struct"
	p "github.com/pulumi/pulumi-go-provider"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"
)

// mockResourceProviderServer records RPC requests and returns custom responses for verification
type mockResourceProviderServer struct {
	pulumirpc.UnimplementedResourceProviderServer

	capturedReq, cannedResp any
}

func (m *mockResourceProviderServer) GetSchema(
	ctx context.Context, req *pulumirpc.GetSchemaRequest,
) (*pulumirpc.GetSchemaResponse, error) {
	m.capturedReq = req
	return m.cannedResp.(*pulumirpc.GetSchemaResponse), nil
}

func (m *mockResourceProviderServer) CheckConfig(
	ctx context.Context, req *pulumirpc.CheckRequest,
) (*pulumirpc.CheckResponse, error) {
	m.capturedReq = req
	return m.cannedResp.(*pulumirpc.CheckResponse), nil
}

func (m *mockResourceProviderServer) Configure(
	ctx context.Context, req *pulumirpc.ConfigureRequest,
) (*pulumirpc.ConfigureResponse, error) {
	m.capturedReq = req
	return m.cannedResp.(*pulumirpc.ConfigureResponse), nil
}

func (m *mockResourceProviderServer) Cancel(
	ctx context.Context, req *emptypb.Empty,
) (*emptypb.Empty, error) {
	m.capturedReq = req
	return &emptypb.Empty{}, nil
}

func (m *mockResourceProviderServer) Diff(
	ctx context.Context, req *pulumirpc.DiffRequest,
) (*pulumirpc.DiffResponse, error) {
	m.capturedReq = req
	return m.cannedResp.(*pulumirpc.DiffResponse), nil
}

func (m *mockResourceProviderServer) DiffConfig(
	ctx context.Context, req *pulumirpc.DiffRequest,
) (*pulumirpc.DiffResponse, error) {
	m.capturedReq = req
	return m.cannedResp.(*pulumirpc.DiffResponse), nil
}

func (m *mockResourceProviderServer) Update(
	ctx context.Context, req *pulumirpc.UpdateRequest,
) (*pulumirpc.UpdateResponse, error) {
	m.capturedReq = req
	return m.cannedResp.(*pulumirpc.UpdateResponse), nil
}

func (m *mockResourceProviderServer) Delete(
	ctx context.Context, req *pulumirpc.DeleteRequest,
) (*emptypb.Empty, error) {
	m.capturedReq = req
	return m.cannedResp.(*emptypb.Empty), nil
}

// wrapProvider wraps a raw RPC provider in a p.Provider and converts it back to an RPC server
func wrapProvider(t *testing.T, mock pulumirpc.ResourceProviderServer) pulumirpc.ResourceProviderServer {
	wrapped := Provider(mock)
	server, err := p.RawServer("test", "1.0.0", wrapped)(nil)
	require.NoError(t, err)
	return server
}

func testPassthrough[Req, Resp any](
	t *testing.T, req Req, resp Resp,
	getCall func(pulumirpc.ResourceProviderServer) func(context.Context, Req) (Resp, error),
) {
	// Set up mock
	mock := &mockResourceProviderServer{cannedResp: resp}
	server := wrapProvider(t, mock)

	call := getCall(server)

	// Call the wrapped server
	actualResp, err := call(t.Context(), req)
	require.NoError(t, err)

	// Verify the request was passed through unchanged
	assert.Equal(t, req, mock.capturedReq.(Req), "Request should be passed through to underlying provider unchanged")

	// Verify the response was passed through unchanged
	assert.Equal(t, resp, actualResp, "Response should be passed through from underlying provider unchanged")
}

func TestDiffPassthrough(t *testing.T) {
	t.Parallel()
	testPassthrough(t,
		&pulumirpc.DiffRequest{
			Id:  "resource-id-123",
			Urn: "urn:pulumi:stack::project::pkg:type:Resource::myres",
			Olds: &structpb.Struct{Fields: map[string]*structpb.Value{
				"count": {Kind: &structpb.Value_NumberValue{NumberValue: 1}},
			}},
			News: &structpb.Struct{Fields: map[string]*structpb.Value{
				"count": {Kind: &structpb.Value_NumberValue{NumberValue: 2}},
			}},
			OldInputs: &structpb.Struct{Fields: map[string]*structpb.Value{
				"input": {Kind: &structpb.Value_StringValue{StringValue: "old"}},
			}},
			IgnoreChanges: []string{"tags", "labels"},
			Name:          "myres",
			Type:          "pkg:type:Resource",
		},
		&pulumirpc.DiffResponse{
			Changes:             pulumirpc.DiffResponse_DIFF_SOME,
			Replaces:            []string{"count"},
			DeleteBeforeReplace: true,
			Diffs:               []string{"count"},
			HasDetailedDiff:     true,
			DetailedDiff: map[string]*pulumirpc.PropertyDiff{
				"count": {Kind: pulumirpc.PropertyDiff_UPDATE_REPLACE, InputDiff: false},
			},
		},
		func(s pulumirpc.ResourceProviderServer) func(
			context.Context, *pulumirpc.DiffRequest,
		) (*pulumirpc.DiffResponse, error) {
			return s.Diff
		},
	)
}

func TestDiffConfigPassthrough(t *testing.T) {
	t.Parallel()
	testPassthrough(t,
		&pulumirpc.DiffRequest{
			Id:  "config-id-456",
			Urn: "urn:pulumi:stack::project::pulumi:pulumi:Stack::project-stack",
			Olds: &structpb.Struct{Fields: map[string]*structpb.Value{
				"apiKey": {Kind: &structpb.Value_StringValue{StringValue: "old-key"}},
			}},
			News: &structpb.Struct{Fields: map[string]*structpb.Value{
				"apiKey": {Kind: &structpb.Value_StringValue{StringValue: "new-key"}},
			}},
			OldInputs: &structpb.Struct{Fields: map[string]*structpb.Value{
				"region": {Kind: &structpb.Value_StringValue{StringValue: "us-west-2"}},
			}},
			IgnoreChanges: []string{"metadata"},
			Name:          "project-stack",
			Type:          "pulumi:pulumi:Stack",
		},
		&pulumirpc.DiffResponse{
			Changes:             pulumirpc.DiffResponse_DIFF_SOME,
			Replaces:            []string{"apiKey"},
			DeleteBeforeReplace: false,
			Diffs:               []string{"apiKey"},
			HasDetailedDiff:     true,
			DetailedDiff: map[string]*pulumirpc.PropertyDiff{
				"apiKey": {Kind: pulumirpc.PropertyDiff_UPDATE_REPLACE, InputDiff: true},
			},
		},
		func(s pulumirpc.ResourceProviderServer) func(
			context.Context, *pulumirpc.DiffRequest,
		) (*pulumirpc.DiffResponse, error) {
			return s.DiffConfig
		},
	)
}

func TestUpdatePassthrough(t *testing.T) {
	t.Parallel()
	testPassthrough(t,
		&pulumirpc.UpdateRequest{
			Id:  "update-id-789",
			Urn: "urn:pulumi:stack::project::pkg:type:Resource::myres",
			Olds: &structpb.Struct{Fields: map[string]*structpb.Value{
				"version": {Kind: &structpb.Value_NumberValue{NumberValue: 1}},
			}},
			News: &structpb.Struct{Fields: map[string]*structpb.Value{
				"version": {Kind: &structpb.Value_NumberValue{NumberValue: 2}},
			}},
			OldInputs: &structpb.Struct{Fields: map[string]*structpb.Value{
				"config": {Kind: &structpb.Value_StringValue{StringValue: "production"}},
			}},
			Timeout:       600.0,
			IgnoreChanges: []string{"tags"},
			Preview:       false,
			Name:          "myres",
			Type:          "pkg:type:Resource",
		},
		&pulumirpc.UpdateResponse{
			Properties: &structpb.Struct{Fields: map[string]*structpb.Value{
				"version": {Kind: &structpb.Value_NumberValue{NumberValue: 2}},
				"status":  {Kind: &structpb.Value_StringValue{StringValue: "updated"}},
			}},
		},
		func(s pulumirpc.ResourceProviderServer) func(
			context.Context, *pulumirpc.UpdateRequest,
		) (*pulumirpc.UpdateResponse, error) {
			return s.Update
		},
	)
}

func TestDeletePassthrough(t *testing.T) {
	t.Parallel()
	testPassthrough(t,
		&pulumirpc.DeleteRequest{
			Id:  "delete-id-101",
			Urn: "urn:pulumi:stack::project::pkg:type:Resource::myres",
			Properties: &structpb.Struct{Fields: map[string]*structpb.Value{
				"data": {Kind: &structpb.Value_StringValue{StringValue: "important"}},
			}},
			OldInputs: &structpb.Struct{Fields: map[string]*structpb.Value{
				"retentionPolicy": {Kind: &structpb.Value_StringValue{StringValue: "keep"}},
			}},
			Timeout: 300.0,
			Name:    "myres",
			Type:    "pkg:type:Resource",
		},
		&emptypb.Empty{},
		func(s pulumirpc.ResourceProviderServer) func(
			context.Context, *pulumirpc.DeleteRequest,
		) (*emptypb.Empty, error) {
			return s.Delete
		},
	)
}
