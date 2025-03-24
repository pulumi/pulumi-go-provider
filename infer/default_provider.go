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
	"fmt"

	provider "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/middleware/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

type DefaultProvider struct {
	name, version string
	metadata      schema.Metadata
	resources     []InferredResource
	components    []InferredComponent
	functions     []InferredFunction
	config        InferredConfig
	moduleMap     map[tokens.ModuleName]tokens.ModuleName
}

// NewDefaultProvider creates an inferred provider which fills as many defaults as possible.
func NewDefaultProvider() *DefaultProvider {
	defaultMetadata := schema.Metadata{
		LanguageMap: map[string]any{
			"nodejs": map[string]any{
				"respectSchemaVersion": true,
			},
			"go": map[string]any{
				"generateResourceContainerTypes": true,
				"respectSchemaVersion":           true,
			},
			"python": map[string]any{
				"requires": map[string]any{
					"pulumi": ">=3.0.0,<4.0.0",
				},
				"respectSchemaVersion": true,
			},
			"csharp": map[string]any{
				"packageReferences": map[string]any{
					"Pulumi": "3.*",
				},
				"respectSchemaVersion": true,
			},
		},
	}

	return &DefaultProvider{
		metadata: defaultMetadata,
	}
}

// WithResources adds the given custom resources to the provider.
func (dp *DefaultProvider) WithResources(resources ...InferredResource) *DefaultProvider {
	dp.resources = append(dp.resources, resources...)
	return dp
}

// WithComponents adds the given components to the provider.
func (dp *DefaultProvider) WithComponents(components ...InferredComponent) *DefaultProvider {
	dp.components = append(dp.components, components...)
	return dp
}

// WithFunctions adds the given functions to the provider.
func (dp *DefaultProvider) WithFunctions(functions ...InferredFunction) *DefaultProvider {
	dp.functions = append(dp.functions, functions...)
	return dp
}

// WithConfig adds the given config to the provider.
func (dp *DefaultProvider) WithConfig(config InferredConfig) *DefaultProvider {
	dp.config = config
	return dp
}

// WithModuleMap adds the given module map to the provider.
func (dp *DefaultProvider) WithModuleMap(moduleMap map[tokens.ModuleName]tokens.ModuleName) *DefaultProvider {
	dp.moduleMap = moduleMap
	return dp
}

func (dp *DefaultProvider) WithLanguageMap(languageMap map[string]any) *DefaultProvider {
	dp.metadata.LanguageMap = languageMap
	return dp
}

func (dp *DefaultProvider) WithDescription(description string) *DefaultProvider {
	dp.metadata.Description = description
	return dp
}

func (dp *DefaultProvider) WithDisplayName(displayName string) *DefaultProvider {
	dp.metadata.DisplayName = displayName
	return dp
}

func (dp *DefaultProvider) WithKeywords(keywords ...string) *DefaultProvider {
	dp.metadata.Keywords = append(dp.metadata.Keywords, keywords...)
	return dp
}

func (dp *DefaultProvider) WithHomepage(homepage string) *DefaultProvider {
	dp.metadata.Homepage = homepage
	return dp
}

func (dp *DefaultProvider) WithRepository(repository string) *DefaultProvider {
	dp.metadata.Repository = repository
	return dp
}

func (dp *DefaultProvider) WithPublisher(publisher string) *DefaultProvider {
	dp.metadata.Publisher = publisher
	return dp
}

func (dp *DefaultProvider) WithLogoURL(logoURL string) *DefaultProvider {
	dp.metadata.LogoURL = logoURL
	return dp
}

func (dp *DefaultProvider) WithLicense(license string) *DefaultProvider {
	dp.metadata.License = license
	return dp
}

func (dp *DefaultProvider) WithPluginDownloadURL(pluginDownloadURL string) *DefaultProvider {
	dp.metadata.PluginDownloadURL = pluginDownloadURL
	return dp
}

// WithName sets the provider name.
func (dp *DefaultProvider) WithName(name string) *DefaultProvider {
	dp.name = name
	return dp
}

// WithVersion sets the provider version.
func (dp *DefaultProvider) WithVersion(version string) *DefaultProvider {
	dp.version = version
	return dp
}

// Build constructs the provider options based on the current state of the default provider.
func (dp *DefaultProvider) Build() *Options {
	return &Options{
		Metadata:   dp.metadata,
		Resources:  dp.resources,
		Components: dp.components,
		Functions:  dp.functions,
		Config:     dp.config,
		ModuleMap:  dp.moduleMap,
	}
}

// validate checks if the default provider configuration is valid.
func (dp *DefaultProvider) validate() error {
	switch {
	case dp.name == "":
		return fmt.Errorf("provider name is required")
	case dp.version == "":
		return fmt.Errorf("provider version is required")
	case len(dp.components) == 0 && len(dp.resources) == 0 && len(dp.functions) == 0:
		return fmt.Errorf("at least one resource, component, or function is required")
	}

	return nil
}

// BuildAndRun builds the provider options, validates them, and runs the provider.
func (dp *DefaultProvider) BuildAndRun() error {
	if err := dp.validate(); err != nil {
		return err
	}

	opts := dp.Build()
	return provider.RunProvider(dp.name, dp.version, Provider(*opts))
}
