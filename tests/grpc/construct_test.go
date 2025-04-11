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
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func TestConstructLifecycle(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	cmd := exec.CommandContext(ctx, "go", "build", "-o", "construct_consumer/pulumi-resource-test", "./provider/.")
	require.NoError(t, cmd.Run(), strings.Join(cmd.Args, " "))

	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir: "construct_consumer",
	})
}

func TestConstruct(t *testing.T) {
	// t.SkipNow()

	replay.Replay(t, constructProvider(t), `
{
  "method": "/pulumirpc.ResourceProvider/Construct",
  "request": {
    "project": "test",
    "stack": "test",
	"config": {
		"test:c1": "s",
		"test:c2": "3.14"
	},
	"configSecretKeys": ["test:c1"],
    "dryRun": true,
    "parallel": 48,
    "monitorEndpoint": "127.0.0.1:59532",
    "type": "test:index:Component",
    "name": "test-component",
    "parent": "urn:pulumi:test::test::test:index:Parent::parent",
	"aliases": ["urn2"],
    "inputs": {
      "k1": "s"
    },
	"dependencies": ["urn3"],
    "inputDependencies": {
      "k1": {
        "urns": ["urn4", "urn5"]
      }
    },
	"additionalSecretOutputs": ["r1"],
    "protect": true,
    "providers": {
      "aws": "p1"
    },
    "customTimeouts": {
      "create": "1m",
      "update": "2m",
      "delete": "3m"
	},
	"deletedWith": "urn6",
	"deleteBeforeReplace": true,
	"ignoreChanges": ["k1"],
	"replaceOnChanges": ["k2"],
	"retainOnDelete": true,
    "acceptsOutputValues": true
  },
  "response": {
    "return": {
      "r1":{
        "4dabf18193072939515e22adb298388d": "d0e6a833031e9bbcd3f4e8bde6ca49a4",
        "dependencies": ["urn7","urn8"]
      }
    },
    "returnDependencies": {
      "r1": {
        "urns": ["urn7", "urn8"]
      }
    }
  },
  "metadata": {
    "kind": "resource",
    "mode": "client",
    "name": "test"
  }
}`)
}

type Component struct {
	pulumi.ResourceState
	R1 pulumi.StringOutput `pulumi:"r1"`
}

func constructProvider(t *testing.T) pulumirpc.ResourceProviderServer {
	s, err := p.RawServer("construct", "0.1.0", p.Provider{
		Construct: func(ctx context.Context, req p.ConstructRequest) (p.ConstructResponse, error) {

			assert.Equal(t, resource.URN("urn:pulumi:test::test::test:index:Parent$test:index:Component::test-component"), req.Urn)
			assert.Equal(t, resource.URN("urn:pulumi:test::test::test:index:Parent::parent"), req.Parent)

			assert.Equal(t, []resource.URN{"urn2"}, req.Options.Aliases)
			assert.Equal(t, []resource.URN{"urn3"}, req.Options.Dependencies)
			assert.Equal(t, true, req.Options.Protect)
			assert.Equal(t, map[string]string{"aws": "p1"}, req.Options.Providers)
			assert.Equal(t, map[resource.PropertyKey][]resource.URN{
				"k1": {resource.URN("urn4"), resource.URN("urn5")},
			}, req.Options.InputDependencies)
			assert.Equal(t, []resource.PropertyKey{"r1"}, req.Options.AdditionalSecretOutputs)
			assert.Equal(t, &resource.CustomTimeouts{Create: 60, Update: 120, Delete: 180}, req.Options.CustomTimeouts)
			assert.Equal(t, resource.URN("urn6"), req.Options.DeletedWith)
			assert.Equal(t, true, req.Options.DeleteBeforeReplace)
			assert.Equal(t, []string{"k1"}, req.Options.IgnoreChanges)
			assert.Equal(t, []string{"k2"}, req.Options.ReplaceOnChanges)

			assert.Equal(t, resource.PropertyMap{
				"k1": resource.NewProperty("s"),
			}, req.Inputs)
			assert.Equal(t, map[resource.PropertyKey][]resource.URN{
				"k1": {resource.URN("urn4"), resource.URN("urn5")},
			}, req.Options.InputDependencies)

			return p.ConstructResponse{
				Urn: resource.URN("urn:pulumi:test::test::test:index:Parent$test:index:Component::test-component"),
				State: resource.PropertyMap{
					"r1": resource.NewProperty(resource.Output{
						Element: resource.NewProperty("e1"),
						Dependencies: []resource.URN{
							"urn7", "urn8",
						},
					}),
				},
			}, nil
		},
	})(nil)
	require.NoError(t, err)
	return s
}
