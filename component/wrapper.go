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

package component

import (
	"fmt"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi-go-provider/middleware/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// Resource is a type alias for the inferred component resource type.
type Resource = infer.InferredComponent

// ConstructorFn is a type alias for infer.ConstructorFn which represents
// a function that creates a component resource.
type ConstructorFn[I any, O pulumi.ComponentResource] = infer.ComponentFn[I, O]

// ProgramComponent allows us to create a component resource that can be understood by the
// underlying provider. This is a wrapper around the infer.ProgramComponent function.
func ProgramComponent[I any, O pulumi.ComponentResource](fn ConstructorFn[I, O]) Resource {
	return infer.ProgramComponent[I, O](fn)
}

// ProviderHost starts a provider with customized options. At least one component resource must be provided
// to the provider.
func ProviderHost(opts ...providerOpt) error {
	providerOpts := providerOpts{
		components: make(map[Resource]struct{}),
	}

	// Apply all option functions
	for _, opt := range opts {
		opt(&providerOpts)
	}

	if len(providerOpts.components) == 0 {
		return fmt.Errorf("no resource components were registered with the provider")
	}

	return p.RunProvider(providerOpts.name, providerOpts.version, provider(providerOpts.components))
}

func provider(components map[Resource]struct{}) p.Provider {
	inferOpts := infer.Options{
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
	}

	// Register all the inferred components with the provider.
	for ic := range components {
		inferOpts.Components = append(inferOpts.Components, ic)
	}

	return infer.Provider(inferOpts)
}
