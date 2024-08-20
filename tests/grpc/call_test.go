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
	"os/exec"
	"strings"
	"testing"

	"github.com/pulumi/providertest/replay"
	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	p "github.com/pulumi/pulumi-go-provider"
)

func TestCallLifecycle(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	cmd := exec.CommandContext(ctx, "go", "build", "-o", "call_consumer/pulumi-resource-test", "./provider/.")
	require.NoError(t, cmd.Run(), strings.Join(cmd.Args, " "))

	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir: "call_consumer",
	})
}

func TestCall(t *testing.T) {
	replay.Replay(t, callProvider(t), `{
    "method": "/pulumirpc.ResourceProvider/Call",
    "request": {
        "tok": "some-token",
        "args": {
		"k1": "s",
		"k2": 3.14
	},
        "stack": "some-stack"
    },
    "response": {
	"return": {
	  "r1": "04da6b54-80e4-46f7-96ec-b56ff0331ba9"
	},
	"returnDependencies": {
	  "r1": {
            "urns": ["urn1", "urn2"]
          }
	}
    },
    "metadata": {
        "kind": "resource",
        "mode": "client",
        "name": "asset"
    }
}`)
}

func callProvider(t *testing.T) pulumirpc.ResourceProviderServer {
	s, err := p.RawServer("call", "0.1.0", p.Provider{
		Call: func(_ context.Context, req p.CallRequest) (p.CallResponse, error) {
			assert.Equal(t, resource.PropertyMap{
				"k1": resource.NewProperty("s"),
				"k2": resource.NewProperty(3.14),
			}, req.Args)
			assert.Equal(t, tokens.ModuleMember("some-token"), req.Tok)
			assert.Equal(t, "some-stack", req.Context.Stack())

			return p.CallResponse{
				Return: resource.PropertyMap{
					"r1": resource.NewProperty(resource.Output{
						Element: resource.NewProperty("e1"),
						Dependencies: []resource.URN{
							"urn1", "urn2",
						},
					}),
				},
			}, nil
		},
	})(nil)
	require.NoError(t, err)
	return s
}
