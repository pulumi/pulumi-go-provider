// Copyright 2024, Pulumi Corporation.
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

package grpc

import (
	"context"
	"testing"

	replay "github.com/pulumi/providertest/replay"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/require"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/middleware/rpc"
)

type rawRPCProvider struct {
	pulumirpc.UnimplementedResourceProviderServer

	diff func(context.Context, *pulumirpc.DiffRequest) (*pulumirpc.DiffResponse, error)
}

func (r rawRPCProvider) Diff(ctx context.Context, req *pulumirpc.DiffRequest) (*pulumirpc.DiffResponse, error) {
	return r.diff(ctx, req)
}

func wrapRPCProvider(t *testing.T, provider rawRPCProvider) pulumirpc.ResourceProviderServer {
	s, err := p.RawServer("test", "1.0.0", rpc.Provider(provider))(nil)
	require.NoError(t, err)
	return s
}

// TestWrapRPCEmptyDiff ensures that hasDetailedDiff is correctly set even when there are
// no listed changes.
func TestWrapRPCEmptyDiff(t *testing.T) {
	t.Run("no-detailed-diff", func(t *testing.T) {
		replay.Replay(t, wrapRPCProvider(t, rawRPCProvider{
			diff: func(context.Context, *pulumirpc.DiffRequest) (*pulumirpc.DiffResponse, error) {
				return &pulumirpc.DiffResponse{
					Changes:         pulumirpc.DiffResponse_DIFF_SOME,
					HasDetailedDiff: false,
				}, nil
			},
		}), `{
  "method": "/pulumirpc.ResourceProvider/Diff",
  "request": {
    "id": "vpc-4b82e033",
    "urn": "urn:pulumi:testtags::tags-combinations-go::aws:ec2/defaultVpc:DefaultVpc::go-web-default-vpc",
    "olds": {},
    "news": {},
    "oldInputs": {}
  },
  "response": {"changes": "DIFF_SOME"}
}`)
	})
	t.Run("has-detailed-diff", func(t *testing.T) {
		replay.Replay(t, wrapRPCProvider(t, rawRPCProvider{
			diff: func(context.Context, *pulumirpc.DiffRequest) (*pulumirpc.DiffResponse, error) {
				return &pulumirpc.DiffResponse{
					Changes:         pulumirpc.DiffResponse_DIFF_SOME,
					HasDetailedDiff: true,
				}, nil
			},
		}), `{
  "method": "/pulumirpc.ResourceProvider/Diff",
  "request": {
    "id": "vpc-4b82e033",
    "urn": "urn:pulumi:testtags::tags-combinations-go::aws:ec2/defaultVpc:DefaultVpc::go-web-default-vpc",
    "olds": {},
    "news": {},
    "oldInputs": {}
  },
  "response": {
    "changes": "DIFF_SOME",
    "hasDetailedDiff": true
  }
}`)
	})
}
