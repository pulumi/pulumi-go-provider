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
	"sync"
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi-go-provider/integration"
	"github.com/pulumi/pulumi-go-provider/middleware/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

const identityDownloadURL = "github://api.github.com/example/pulumi-identity"

// identityModuleMap maps this test package's Go package name onto the "index" module,
// matching the rest of the suite.
var identityModuleMap = map[tokens.ModuleName]tokens.ModuleName{"tests": "index"}

// identityChild is a resource the component below registers itself, the way a component
// provider registers a resource of its own package from inside Construct.
type identityChild struct {
	pulumi.ResourceState
}

type IdentityComponentArgs struct{}

type IdentityComponent struct {
	pulumi.ResourceState
	IdentityComponentArgs
}

// NewIdentityComponent registers one child of its own package, stamped with the running
// provider's plugin identity, and one child of a foreign package, deliberately left
// alone.
func NewIdentityComponent(ctx *pulumi.Context, name string, args IdentityComponentArgs,
	opts ...pulumi.ResourceOption,
) (*IdentityComponent, error) {
	comp := &IdentityComponent{}
	if err := ctx.RegisterComponentResource(p.GetTypeToken(ctx), name, comp, opts...); err != nil {
		return nil, err
	}

	self := p.GetPluginIdentity(ctx.Context())

	child := &identityChild{}
	err := ctx.RegisterResource("test:index:IdentityChild", name+"-child", pulumi.Map{}, child,
		append(self.ResourceOptions(), pulumi.Parent(comp))...)
	if err != nil {
		return nil, err
	}

	foreign := &identityChild{}
	err = ctx.RegisterResource("other:index:Thing", name+"-foreign", pulumi.Map{}, foreign,
		pulumi.Parent(comp))
	if err != nil {
		return nil, err
	}

	return comp, nil
}

// registration is the subset of a RegisterResource call this test asserts on.
type registration struct {
	version     string
	downloadURL string
}

// TestConstructStampsOwnPluginIdentity asserts that a component can learn the identity of
// the plugin serving it, and that applying it records both the version and the download
// URL on a self-registered resource. Without them the engine writes a provider carrying
// neither, and resolves the plugin from github.com/pulumi/pulumi-<name>, which does not
// exist for a third-party provider.
func TestConstructStampsOwnPluginIdentity(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	seen := map[string]registration{}

	prov := identityProvider(t, &integration.MockResourceMonitor{
		NewResourceF: func(args integration.MockResourceArgs) (string, property.Map, error) {
			if rpc := args.RegisterRPC; rpc != nil {
				mu.Lock()
				seen[string(args.TypeToken)] = registration{
					version:     rpc.GetVersion(),
					downloadURL: rpc.GetPluginDownloadURL(),
				}
				mu.Unlock()
			}
			return args.ID, property.Map{}, nil
		},
	})

	_, err := prov.Construct(p.ConstructRequest{
		Urn:    childUrn("IdentityComponent", "test-component", "test-parent"),
		Parent: urn("Parent", "test-parent"),
		Inputs: property.Map{},
	})
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()

	own, ok := seen["test:index:IdentityChild"]
	require.True(t, ok, "the component's own child was never registered; seen: %v", seen)
	assert.Equal(t, "1.0.0", own.version,
		"a self-registered resource must record the version of the plugin that created it")
	assert.Equal(t, identityDownloadURL, own.downloadURL,
		"a self-registered resource must record where its plugin can be downloaded from")

	// The identity names one specific plugin, so it must not leak onto resources of
	// other packages — that would send the engine looking for their plugin at our
	// version and our URL.
	foreign, ok := seen["other:index:Thing"]
	require.True(t, ok, "the foreign child was never registered; seen: %v", seen)
	assert.Empty(t, foreign.version)
	assert.Empty(t, foreign.downloadURL)
}

// TestGetPluginIdentityWithoutDownloadURL covers a provider that never set one: the
// version still comes through, and the call does not fail.
func TestGetPluginIdentityWithoutDownloadURL(t *testing.T) {
	t.Parallel()

	var got p.PluginIdentity
	prov, err := integration.NewServer(t.Context(), "test", semver.MustParse("1.0.0"),
		integration.WithProvider(infer.Provider(infer.Options{
			ModuleMap: identityModuleMap,
			Components: []infer.InferredComponent{
				infer.ComponentF(func(ctx *pulumi.Context, name string, args IdentityComponentArgs,
					opts ...pulumi.ResourceOption,
				) (*IdentityComponent, error) {
					got = p.GetPluginIdentity(ctx.Context())
					comp := &IdentityComponent{}
					err := ctx.RegisterComponentResource(p.GetTypeToken(ctx), name, comp, opts...)
					return comp, err
				}),
			},
		})),
		integration.WithMocks(&integration.MockResourceMonitor{
			NewResourceF: func(args integration.MockResourceArgs) (string, property.Map, error) {
				return args.ID, property.Map{}, nil
			},
		}),
	)
	require.NoError(t, err)

	_, err = prov.Construct(p.ConstructRequest{
		Urn:    childUrn("IdentityComponent", "test-component", "test-parent"),
		Parent: urn("Parent", "test-parent"),
		Inputs: property.Map{},
	})
	require.NoError(t, err)

	assert.Equal(t, "test", got.Package)
	assert.Equal(t, "1.0.0", got.Version)
	assert.Empty(t, got.DownloadURL)
	applied, err := pulumi.NewResourceOptions(got.ResourceOptions()...)
	require.NoError(t, err)
	assert.Equal(t, "1.0.0", applied.Version)
	assert.Empty(t, applied.PluginDownloadURL, "no URL option is produced when the provider set none")
}

func identityProvider(t testing.TB, mocks pulumi.MockResourceMonitor) integration.Server {
	prov := infer.Provider(infer.Options{
		Metadata: schema.Metadata{
			PluginDownloadURL: identityDownloadURL,
		},
		ModuleMap: identityModuleMap,
		Components: []infer.InferredComponent{
			infer.ComponentF(NewIdentityComponent),
		},
	})
	s, err := integration.NewServer(t.Context(), "test", semver.MustParse("1.0.0"),
		integration.WithProvider(prov),
		integration.WithMocks(mocks),
	)
	require.NoError(t, err)
	return s
}
