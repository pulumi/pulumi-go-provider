// Copyright 2016-2025, Pulumi Corporation.
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

package infer

import (
	"context"
	"testing"
	"time"

	provider "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/middleware/schema"
	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockComponentResource is a minimal implementation of pulumi.ComponentResource for testing
type MockComponentResource struct {
	pulumi.ResourceState
	MockComponentResourceInput
}

// MockComponentResourceInput is a minimal implementation of the input struct for testing
type MockComponentResourceInput struct{}

// NewMockComponentResource creates a new instance of MockComponentResource
func NewMockComponentResource(
	ctx *pulumi.Context,
	name string,
	inputs MockComponentResourceInput,
	options ...pulumi.ResourceOption,
) (*MockComponentResource, error) {
	return &MockComponentResource{}, nil
}

type MockResource struct{}

type MockResourceArgs struct{}

type MockResourceState struct{}

func (m MockResource) Create(
	ctx context.Context, req CreateRequest[MockResourceArgs],
) (CreateResponse[MockResourceState], error) {
	return CreateResponse[MockResourceState]{}, nil
}

type MockConfig struct{}

func (mc MockConfig) GetSchema(schema.RegisterDerivativeType) (pschema.ResourceSpec, error) {
	return pschema.ResourceSpec{}, nil
}

func (mc MockConfig) GetToken() (tokens.Type, error) {
	return "", nil
}

type (
	MockFunction       struct{}
	MockFunctionArgs   struct{}
	MockFunctionResult struct{}
)

func (mf MockFunction) Invoke(
	ctx context.Context,
	req FunctionRequest[MockFunctionArgs],
) (FunctionResponse[MockFunctionResult], error) {
	return FunctionResponse[MockFunctionResult]{}, nil
}

func TestNewDefaultProvider(t *testing.T) {
	t.Parallel()
	dp := NewProviderBuilder()
	assert.NotNil(t, dp)

	// Verify default metadata is set correctly
	expectedLangMap := map[string]any{
		"nodejs": map[string]any{
			"respectSchemaVersion": true,
		},
		"go": map[string]any{
			"generateResourceContainerTypes": true,
			"respectSchemaVersion":           true,
		},
		"python": map[string]any{
			"pyproject": map[string]any{
				"enabled": true,
			},
			"respectSchemaVersion": true,
		},
		"csharp": map[string]any{
			"respectSchemaVersion": true,
		},
	}

	assert.Equal(t, expectedLangMap, dp.metadata.LanguageMap)
}

func TestWithResources(t *testing.T) {
	t.Parallel()
	dp := NewProviderBuilder()

	resource1 := Resource(MockResource{})
	resource2 := Resource(MockResource{})

	dp.WithResources(resource1, resource2)

	assert.Equal(t, 2, len(dp.resources))
	assert.Equal(t, resource1, dp.resources[0])
	assert.Equal(t, resource2, dp.resources[1])

	// Test chaining
	resource3 := Resource(MockResource{})
	dp.WithResources(resource3)

	assert.Equal(t, 3, len(dp.resources))
}

func TestWithComponents(t *testing.T) {
	t.Parallel()
	dp := NewProviderBuilder()

	component1 := ComponentF(NewMockComponentResource)
	component2 := ComponentF(NewMockComponentResource)

	dp.WithComponents(component1, component2)

	assert.Equal(t, 2, len(dp.components))
	assert.Equal(t, component1, dp.components[0])
	assert.Equal(t, component2, dp.components[1])

	// Test chaining
	component3 := ComponentF(NewMockComponentResource)
	dp.WithComponents(component3)

	assert.Equal(t, 3, len(dp.components))
}

func TestWithFunctions(t *testing.T) {
	t.Parallel()
	dp := NewProviderBuilder()

	function1 := Function(MockFunction{})
	function2 := Function(MockFunction{})

	dp.WithFunctions(function1, function2)

	assert.Equal(t, 2, len(dp.functions))
	assert.Equal(t, function1, dp.functions[0])
	assert.Equal(t, function2, dp.functions[1])

	// Test chaining
	function3 := Function(MockFunction{})
	dp.WithFunctions(function3)

	assert.Equal(t, 3, len(dp.functions))

	// Test with multiple functions
	functions := []InferredFunction{Function(MockFunction{}), Function(MockFunction{})}
	dp.WithFunctions(functions...)

	assert.Equal(t, 5, len(dp.functions))
}

func TestWithConfig(t *testing.T) {
	t.Parallel()
	dp := NewProviderBuilder()

	config := Config(MockConfig{})

	dp.WithConfig(config)

	assert.Equal(t, config, dp.config)
}

func TestWithModuleMap(t *testing.T) {
	t.Parallel()
	dp := NewProviderBuilder()

	moduleMap := map[tokens.ModuleName]tokens.ModuleName{
		"module1": "mappedModule1",
	}

	dp.WithModuleMap(moduleMap)

	assert.Equal(t, moduleMap, dp.moduleMap)
}

func TestWithLanguageMap(t *testing.T) {
	t.Parallel()
	dp := NewProviderBuilder()

	languageMap := map[string]any{
		"go": map[string]any{
			"importBasePath": "github.com/example/package",
		},
	}

	dp.WithLanguageMap(languageMap)

	assert.Equal(t, languageMap, dp.metadata.LanguageMap)
}

func TestWithMetadataFields(t *testing.T) {
	t.Parallel()
	dp := NewProviderBuilder()

	description := "Test description"
	displayName := "Test Display Name"
	keywords := []string{"test", "provider"}
	homepage := "https://example.com"
	repository := "https://github.com/example/repo"
	publisher := "Test Publisher"
	logoURL := "https://example.com/logo.png"
	license := "MIT"
	pluginDownloadURL := "https://example.com/download"

	dp.WithDescription(description).
		WithDisplayName(displayName).
		WithKeywords(keywords...).
		WithHomepage(homepage).
		WithRepository(repository).
		WithPublisher(publisher).
		WithLogoURL(logoURL).
		WithLicense(license).
		WithPluginDownloadURL(pluginDownloadURL)

	assert.Equal(t, description, dp.metadata.Description)
	assert.Equal(t, displayName, dp.metadata.DisplayName)
	assert.Equal(t, keywords, dp.metadata.Keywords)
	assert.Equal(t, homepage, dp.metadata.Homepage)
	assert.Equal(t, repository, dp.metadata.Repository)
	assert.Equal(t, publisher, dp.metadata.Publisher)
	assert.Equal(t, logoURL, dp.metadata.LogoURL)
	assert.Equal(t, license, dp.metadata.License)
	assert.Equal(t, pluginDownloadURL, dp.metadata.PluginDownloadURL)
}

func TestWithNamespace(t *testing.T) {
	t.Parallel()
	dp := NewProviderBuilder()

	// Test with initial namespace
	namespace := "test-namespace"
	dp.WithNamespace(namespace)
	assert.Equal(t, namespace, dp.metadata.Namespace)

	// Test with updated namespace
	updatedNamespace := "updated-namespace"
	dp.WithNamespace(updatedNamespace)
	assert.Equal(t, updatedNamespace, dp.metadata.Namespace)

	finalNamespace := "final-namespace"
	dp = dp.WithNamespace(finalNamespace)
	assert.Equal(t, finalNamespace, dp.metadata.Namespace)

	opts := dp.BuildOptions()
	assert.Equal(t, finalNamespace, opts.Namespace)
}

func TestWithWrapped(t *testing.T) {
	t.Parallel()

	wrapped := provider.Provider{
		Create: func(_ context.Context, _ provider.CreateRequest) (provider.CreateResponse, error) {
			return provider.CreateResponse{ID: "foo"}, nil
		},
	}

	p, err := NewProviderBuilder().
		WithResources(Resource(MockResource{})).
		WithWrapped(wrapped).Build()
	require.NoError(t, err)

	resp, err := p.Create(context.Background(), provider.CreateRequest{
		Urn: resource.URN("urn:pulumi:x::y::z:a:b::c"),
	})
	assert.NoError(t, err)
	assert.Equal(t, "foo", resp.ID)
}

func TestBuild(t *testing.T) {
	t.Parallel()
	dp := NewProviderBuilder()

	resource := Resource(MockResource{})
	component := ComponentF(NewMockComponentResource)
	functions := Function(MockFunction{})
	config := Config(MockConfig{})
	moduleMap := map[tokens.ModuleName]tokens.ModuleName{
		"module1": "mappedModule1",
	}

	dp.WithResources(resource).
		WithComponents(component).
		WithFunctions(functions).
		WithConfig(config).
		WithModuleMap(moduleMap)

	options := dp.BuildOptions()

	{ // We can't compare these, so just zero them out
		dp.metadata.Update = nil
		options.Update = nil
	}
	assert.Equal(t, dp.metadata, options.Metadata)
	assert.Equal(t, dp.resources, options.Resources)
	assert.Equal(t, dp.components, options.Components)
	assert.Equal(t, dp.functions, options.Functions)
	assert.Equal(t, dp.config, options.Config)
	assert.Equal(t, dp.moduleMap, options.ModuleMap)
}

func TestValidate(t *testing.T) {
	t.Parallel()
	dp := NewProviderBuilder()

	// Should fail with no resources, components or functions
	err := dp.validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "at least one resource, component, or function is required")

	// Add a resource, should pass
	dp.WithResources(Resource(MockResource{}))
	err = dp.validate()
	assert.NoError(t, err)

	// Reset and test with component
	dp = NewProviderBuilder()
	dp.WithComponents(ComponentF(NewMockComponentResource))
	err = dp.validate()
	assert.NoError(t, err)

	// Reset and test with function
	dp = NewProviderBuilder()
	dp.WithFunctions(Function(MockFunction{}))
	err = dp.validate()
	assert.NoError(t, err)
}

//nolint:paralleltest // Running in parallel causes a data race.
func TestBuildAndRun(t *testing.T) {
	// 1. Create a provider without any components and ensure that it returns an error.
	_, err := NewProviderBuilder().Build()
	require.Error(t, err)

	// 2. Create a provider with a component and ensure that it starts and runs successfully.
	p, err := NewProviderBuilder().
		WithComponents(ComponentF(NewMockComponentResource)).
		Build()
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err = p.Run(ctx, "test-provider", "v0.0.1")

	assert.NoError(t, err, "provider startup should not fail")
}
