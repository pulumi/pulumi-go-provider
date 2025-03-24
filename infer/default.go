// Copyright 2022, Pulumi Corporation.
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
	"slices"

	provider "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/middleware/schema"
)

// Default creates an inferred provider which applies as many defaults as possible.
func Default() DefaultProvider {
	return DefaultProvider{}
}

type DefaultProvider struct {
	name, version string
	components    []InferredComponent
}

func (p DefaultProvider) WithComponent(components ...InferredComponent) DefaultProvider {
	p.components = append(slices.Clone(p.components), components...)
	return p
}

func (p DefaultProvider) WithName(name string) DefaultProvider {
	p.name = name
	return p
}

func (p DefaultProvider) WithVersion(version string) DefaultProvider {
	p.version = version
	return p
}

func (p DefaultProvider) BuildAndRun() error {
	if p.name == "" {
		// We can relax this when Pulumi supports injecting the name into the
		// provider, but we shouldn't do so until then, since the rest of Pulumi
		// can't handle a schema with an empty name.
		return fmt.Errorf("Default provider names are not yet supported, please call WithName to add a name")
	}
	if len(p.components) == 0 {
		// This error message will need to be adjusted if DefaultProvider supports
		// more then just components.
		return fmt.Errorf("Default providers must be given at least one component")
	}
	inferOpts := Options{
		Metadata: schema.Metadata{
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
		},
		Components: p.components,
	}

	return provider.RunProvider(p.name, p.version, Provider(inferOpts))
}
