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

	replay "github.com/pulumi/providertest/replay"
	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
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

	jsonLog := `
{
    "method": "/pulumirpc.ResourceProvider/Call",
    "request": {
        "tok": "some-token",
        "args": {
            "k1": "s",
            "k2": 3.14
        },
        "argDependencies": {
            "k1": {
                "urns": ["urn1", "urn2"]
            }
        },
        "config": {
            "test:c1": "s",
            "test:c2": "3.14"
        },
        "configSecretKeys": ["test:c1"],
        "dryRun": true,
        "project": "some-project",
        "stack": "some-stack",
        "acceptsOutputValues": true
    },
    "response": {
        "return": {
          "r1":{
              "4dabf18193072939515e22adb298388d": "d0e6a833031e9bbcd3f4e8bde6ca49a4",
              "dependencies": ["urn1","urn2"]
          }
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
}`

	call := func(_ context.Context, req p.CallRequest) (p.CallResponse, error) {
		assert.Equal(t, resource.PropertyMap{
			"k1": resource.NewProperty("s"),
			"k2": resource.NewProperty(3.14),
		}, req.Args)
		assert.Equal(t, map[resource.PropertyKey][]resource.URN{
			"k1": {resource.URN("urn1"), resource.URN("urn2")},
		}, req.ArgDependencies)
		assert.Equal(t, tokens.ModuleMember("some-token"), req.Tok)
		assert.Equal(t, "some-project", req.Project)
		assert.Equal(t, "some-stack", req.Stack)
		assert.Equal(t, true, req.DryRun)
		assert.Equal(t, map[config.Key]string{
			config.MustParseKey("test:c1"): "s",
			config.MustParseKey("test:c2"): "3.14",
		}, req.Config)
		assert.Equal(t, []config.Key{config.MustParseKey("test:c1")}, req.ConfigSecretKeys)
		assert.Equal(t, true, req.AcceptsOutputValues)

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
	}

	replay.Replay(t, callProvider(t, call), jsonLog)
}

func TestCallWithMalformedRequest(t *testing.T) {

	jsonLog := `
{
    "method": "/pulumirpc.ResourceProvider/Call",
    "request": {
        "tok": "some-token",
        "args": {
            "r1": {
                "4dabf18193072939515e22adb298388d": "invalid"
            }
        },
        "project": "some-project",
        "stack": "some-stack",
        "config": {
            "INVALID": "s"
        },
        "acceptsOutputValues": true
    },
    "errors": ["*"],
    "metadata": {
        "kind": "resource",
        "mode": "client",
        "name": "asset"
    }
}`
	call := func(ctx context.Context, req p.CallRequest) (p.CallResponse, error) {
		assert.FailNow(t, "call was not expected to be called")
		return p.CallResponse{}, nil
	}

	replay.Replay(t, callProvider(t, call), jsonLog)
}

type callF func(_ context.Context, req p.CallRequest) (p.CallResponse, error)

func callProvider(t *testing.T, call callF) pulumirpc.ResourceProviderServer {
	s, err := p.RawServer("call", "0.1.0", p.Provider{
		Call: call,
	})(nil)
	require.NoError(t, err)
	return s
}
