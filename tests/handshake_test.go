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
	mwrpc "github.com/pulumi/pulumi-go-provider/middleware/rpc"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		AcceptsByteString:           true,
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
		AcceptsByteString:           true,
	}, gotRequest)

	assert.Equal(t, &pulumirpc.ProviderHandshakeResponse{
		AcceptSecrets:                   true,
		AcceptResources:                 true,
		AcceptOutputs:                   true,
		AcceptsByteString:               true,
		SupportsAutonamingConfiguration: true,
	}, resp)
}

// Providers that don't implement Handshake still handshake successfully, advertising byte-string
// support by default; providers that set RejectNonUTF8 do not advertise it.
func TestHandshakeByteStringOptIn(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name        string
		provider    p.Provider
		wantAccepts bool
	}{
		{name: "default", provider: p.Provider{}, wantAccepts: true},
		{name: "reject non-UTF8", provider: p.Provider{
			Handshake: func(context.Context, p.HandshakeRequest) (p.HandshakeResponse, error) {
				return p.HandshakeResponse{RejectNonUTF8: true}, nil
			},
		}, wantAccepts: false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server, err := p.RawServer("test", "0.0.0-dev", tt.provider)(nil)
			require.NoError(t, err)

			resp, err := server.Handshake(t.Context(), &pulumirpc.ProviderHandshakeRequest{
				AcceptsByteString: true,
			})
			require.NoError(t, err)
			assert.Equal(t, &pulumirpc.ProviderHandshakeResponse{
				AcceptSecrets:     true,
				AcceptResources:   true,
				AcceptOutputs:     true,
				AcceptsByteString: tt.wantAccepts,
			}, resp)
		})
	}
}

type handshakeServer struct {
	pulumirpc.UnimplementedResourceProviderServer
	acceptsByteString bool

	gotRequest *pulumirpc.ProviderHandshakeRequest
}

func (s *handshakeServer) Handshake(_ context.Context, req *pulumirpc.ProviderHandshakeRequest,
) (*pulumirpc.ProviderHandshakeResponse, error) {
	s.gotRequest = req
	return &pulumirpc.ProviderHandshakeResponse{
		AcceptsByteString: s.acceptsByteString,
	}, nil
}

// The rpc middleware propagates byte-string support in both directions: the engine's acceptance is
// forwarded to the wrapped server (whose outputs are returned to the engine), and the wrapped
// server's acceptance is reported back so the engine knows not to send byte strings to a wrapped
// server that cannot decode them.
func TestMiddlewareHandshakeByteString(t *testing.T) {
	t.Parallel()

	for _, engineAccepts := range []bool{true, false} {
		for _, serverAccepts := range []bool{true, false} {
			server := &handshakeServer{acceptsByteString: serverAccepts}
			resp, err := mwrpc.Provider(server).Handshake(t.Context(), p.HandshakeRequest{
				AcceptsByteString: engineAccepts,
			})
			require.NoError(t, err)
			assert.Equal(t, engineAccepts, server.gotRequest.GetAcceptsByteString())
			assert.Equal(t, p.HandshakeResponse{RejectNonUTF8: !serverAccepts}, resp)
		}
	}
}

func TestByteString(t *testing.T) {
	t.Parallel()

	byteString := "\x00hello \x80\xfe\xff world"

	newServer := func(t *testing.T, create func(p.CreateRequest) (p.CreateResponse, error),
	) pulumirpc.ResourceProviderServer {
		server, err := p.RawServer("test", "0.0.0-dev", p.Provider{
			Create: func(_ context.Context, req p.CreateRequest) (p.CreateResponse, error) {
				return create(req)
			},
		})(nil)
		require.NoError(t, err)
		return server
	}

	t.Run("round-trip when the engine accepts byte strings", func(t *testing.T) {
		t.Parallel()

		var gotProperties property.Map
		server := newServer(t, func(req p.CreateRequest) (p.CreateResponse, error) {
			gotProperties = req.Properties
			return p.CreateResponse{ID: "id", Properties: req.Properties}, nil
		})

		_, err := server.Handshake(t.Context(), &pulumirpc.ProviderHandshakeRequest{
			AcceptsByteString: true,
		})
		require.NoError(t, err)

		properties, err := plugin.MarshalProperties(resource.PropertyMap{
			"data": resource.NewProperty(byteString),
		}, plugin.MarshalOptions{KeepByteString: true})
		require.NoError(t, err)

		resp, err := server.Create(t.Context(), &pulumirpc.CreateRequest{
			Urn:        "urn:pulumi:stack::proj::test:index:res::name",
			Properties: properties,
		})
		require.NoError(t, err)

		assert.Equal(t, property.NewMap(map[string]property.Value{
			"data": property.New(byteString),
		}), gotProperties)

		assert.Equal(t, map[string]any{
			resource.SigKey: resource.ByteStringSig,
			"value":         "AGhlbGxvIID+/yB3b3JsZA==",
		}, resp.GetProperties().AsMap()["data"])
	})

	t.Run("error when the engine does not accept byte strings", func(t *testing.T) {
		t.Parallel()

		server := newServer(t, func(p.CreateRequest) (p.CreateResponse, error) {
			return p.CreateResponse{ID: "id", Properties: property.NewMap(map[string]property.Value{
				"data": property.New(byteString),
			})}, nil
		})

		_, err := server.Handshake(t.Context(), &pulumirpc.ProviderHandshakeRequest{})
		require.NoError(t, err)

		_, err = server.Create(t.Context(), &pulumirpc.CreateRequest{
			Urn: "urn:pulumi:stack::proj::test:index:res::name",
		})
		assert.EqualError(t, err,
			`the value of property "data" is a string that contains non-UTF8 bytes, which the receiver does not support`)
	})
}
