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

package tests

import (
	"context"
	"testing"

	p "github.com/pulumi/pulumi-go-provider"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestHandshake(t *testing.T) {
	t.Parallel()

	rootDir := "/root"
	programDir := "/root/program"
	mapperAddr := "127.0.0.1:12346"
	loaderAddr := "127.0.0.1:12347"
	resolverAddr := "127.0.0.1:12348"

	var gotRequest p.HandshakeRequest
	server, err := p.RawServer("test", "0.0.0-dev", p.Provider{
		Handshake: func(_ context.Context, req p.HandshakeRequest) (p.HandshakeResponse, error) {
			gotRequest = req
			return p.HandshakeResponse{
				SupportsAutonamingConfiguration: true,
			}, nil
		},
	})(nil)
	require.NoError(t, err)

	resp, err := server.Handshake(t.Context(), &pulumirpc.ProviderHandshakeRequest{
		EngineAddress:               "127.0.0.1:12345",
		RootDirectory:               &rootDir,
		ProgramDirectory:            &programDir,
		ConfigureWithUrn:            true,
		SupportsViews:               true,
		SupportsRefreshBeforeUpdate: true,
		InvokeWithPreview:           true,
		MapperTarget:                &mapperAddr,
		LoaderTarget:                &loaderAddr,
		ResolverTarget:              &resolverAddr,
	})
	require.NoError(t, err)

	assert.Equal(t, p.HandshakeRequest{
		EngineAddress:               "127.0.0.1:12345",
		RootDirectory:               &rootDir,
		ProgramDirectory:            &programDir,
		ConfigureWithUrn:            true,
		SupportsViews:               true,
		SupportsRefreshBeforeUpdate: true,
		InvokeWithPreview:           true,
		MapperAddress:               &mapperAddr,
		LoaderAddress:               &loaderAddr,
		ResolverAddress:             &resolverAddr,
	}, gotRequest)

	assert.Equal(t, &pulumirpc.ProviderHandshakeResponse{
		AcceptSecrets:                   true,
		AcceptResources:                 true,
		AcceptOutputs:                   true,
		SupportsAutonamingConfiguration: true,
	}, resp)
}

func TestHandshakeUnimplemented(t *testing.T) {
	t.Parallel()

	server, err := p.RawServer("test", "0.0.0-dev", p.Provider{})(nil)
	require.NoError(t, err)

	_, err = server.Handshake(t.Context(), &pulumirpc.ProviderHandshakeRequest{})
	assert.Equal(t, codes.Unimplemented, status.Code(err))
}
