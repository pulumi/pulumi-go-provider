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
	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi-go-provider/middleware/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type registry struct {
	// inferredComponents is a set of inferred components
	inferredComponents map[Resource]struct{}
}

// globalRegistry is a global registry for all inferred components. This is used to
// register the components with the provider during initialization.
var globalRegistry = registry{
	inferredComponents: make(map[Resource]struct{}),
}

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

// RegisterType registers a type within the global registry for this wrapper package. This
// allows the provider to access the custom types and functions required to infer its schema.
func RegisterType(ic Resource) {
	globalRegistry.inferredComponents[ic] = struct{}{}
}

// ProviderHost starts a provider that contains all inferred components.
func ProviderHost(name string, version string) error {
	return p.RunProvider(name, version, provider())
}

func provider() p.Provider {
	opt := infer.Options{
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
	for ic := range globalRegistry.inferredComponents {
		opt.Components = append(opt.Components, ic)
	}

	return infer.Provider(opt)
}
