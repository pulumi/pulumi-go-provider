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

package grpc

import (
	"context"
	"os/exec"
	"strings"
	"testing"

	"github.com/pulumi/providertest/replay"
	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func TestConstructLifecycle(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	cmd := exec.CommandContext(ctx, "go", "build", "-o", "construct_consumer/pulumi-resource-test", "./provider/.")
	require.NoError(t, cmd.Run(), strings.Join(cmd.Args, " "))

	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir: "construct_consumer",
	})
}

func TestConstruct(t *testing.T) {
	t.Parallel()

	jsonLog := `
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
	  "test": "urn:pulumi:test::test::pulumi:providers:test::my-provider::09e6d266-58b0-4452-8395-7bbe03011fad"
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
    "urn": "urn:pulumi:test::test::test:index:Parent$test:index:Component::test-component",
    "state": {
      "r1": {
        "4dabf18193072939515e22adb298388d": "d0e6a833031e9bbcd3f4e8bde6ca49a4",
        "dependencies": ["urn7","urn8"]
      }
    }
  },
  "metadata": {
    "kind": "resource",
    "mode": "client",
    "name": "test"
  }
}`

	construct := func(ctx context.Context, req p.ConstructRequest) (p.ConstructResponse, error) {
		_true := true
		assert.Equal(t,
			resource.URN("urn:pulumi:test::test::test:index:Parent$test:index:Component::test-component"),
			req.Urn,
		)
		assert.Equal(t, resource.URN("urn:pulumi:test::test::test:index:Parent::parent"), req.Parent)
		assert.Equal(t, map[config.Key]string{
			config.MustParseKey("test:c1"): "s",
			config.MustParseKey("test:c2"): "3.14",
		}, req.Config)
		assert.Equal(t, []config.Key{config.MustParseKey("test:c1")}, req.ConfigSecretKeys)
		assert.Equal(t, []resource.URN{"urn2"}, req.Aliases)
		assert.Equal(t, []resource.URN{"urn3"}, req.Dependencies)
		assert.Equal(t, &_true, req.Protect)
		assert.Equal(t, map[tokens.Package]p.ProviderReference{
			"test": {
				Urn: resource.URN("urn:pulumi:test::test::pulumi:providers:test::my-provider"),
				ID:  "09e6d266-58b0-4452-8395-7bbe03011fad",
			},
		}, req.Providers)
		assert.Equal(t, []string{"r1"}, req.AdditionalSecretOutputs)
		assert.Equal(t, &resource.CustomTimeouts{Create: 60, Update: 120, Delete: 180}, req.CustomTimeouts)
		assert.Equal(t, resource.URN("urn6"), req.DeletedWith)
		assert.Equal(t, &_true, req.DeleteBeforeReplace)
		assert.Equal(t, []string{"k1"}, req.IgnoreChanges)
		assert.Equal(t, []string{"k2"}, req.ReplaceOnChanges)

		assert.Equal(t, property.NewMap(map[string]property.Value{
			"k1": property.New("s").WithDependencies([]resource.URN{"urn4", "urn5"}),
		}), req.Inputs)

		return p.ConstructResponse{
			Urn: resource.URN("urn:pulumi:test::test::test:index:Parent$test:index:Component::test-component"),
			State: property.NewMap(map[string]property.Value{
				"r1": property.New(property.Computed).WithDependencies([]resource.URN{
					"urn7", "urn8",
				}),
			}),
		}, nil
	}
	replay.Replay(t, constructProvider(t, construct), jsonLog)
}

func TestConstructWithMalformedRequest(t *testing.T) {
	t.Parallel()

	jsonLog := `
{
  "method": "/pulumirpc.ResourceProvider/Construct",
  "request": {
    "type": "test:index:Component",
    "name": "test-component",
    "project": "test",
    "stack": "test",
    "config": {
        "INVALID": "s"
    },
    "configSecretKeys": ["INVALID"],
    "providers": {
        "test": "not::a:valid:urn::id"
    },
    "customTimeouts": {
      "create": "1d",
	  "update": "1d",
	  "delete": "1d"
	},
	"inputs": {
      "r1": {
        "4dabf18193072939515e22adb298388d": "invalid"
      }
    },
    "acceptsOutputValues": true
  },
  "errors": ["*"],
  "metadata": {
    "kind": "resource",
    "mode": "client",
    "name": "test"
  }
}`
	construct := func(ctx context.Context, req p.ConstructRequest) (p.ConstructResponse, error) {
		assert.FailNow(t, "construct was not expected to be called")
		return p.ConstructResponse{}, nil
	}

	replay.Replay(t, constructProvider(t, construct), jsonLog)

}

type Component struct {
	pulumi.ResourceState
	R1 pulumi.StringOutput `pulumi:"r1"`
}

type constructF func(ctx context.Context, req p.ConstructRequest) (p.ConstructResponse, error)

func constructProvider(t *testing.T, construct constructF) pulumirpc.ResourceProviderServer {
	s, err := p.RawServer("construct", "0.1.0", p.Provider{
		Construct: construct,
	})(nil)
	require.NoError(t, err)
	return s
}
