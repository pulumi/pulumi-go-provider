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
)

type globalRegistry struct {
	InferredComponents []infer.InferredComponent
}

var registry = globalRegistry{
	InferredComponents: []infer.InferredComponent{},
}

// RegisterType registers a type with the global registry.
func RegisterType(ic infer.InferredComponent) {
	registry.InferredComponents = append(registry.InferredComponents, ic)
}

func ProviderHost() {
	p.RunProvider("go-components", "0.1.0", provider())
}

func provider() p.Provider {
	opt := infer.Options{
		// Components: []infer.InferredComponent{
		// 	infer.Component[*RandomComponent, RandomComponentArgs, *RandomComponentState](),
		// },
		Metadata: schema.Metadata{
			LanguageMap: map[string]any{
				"nodejs": map[string]any{
					"dependencies": map[string]any{
						"@pulumi/random": "^4.16.8",
					},
					"respectSchemaVersion": true,
				},
				"go": map[string]any{
					"generateResourceContainerTypes": true,
					"respectSchemaVersion":           true,
				},
				"python": map[string]any{
					"requires": map[string]any{
						"pulumi":        ">=3.0.0,<4.0.0",
						"pulumi_random": ">=4.0.0,<5.0.0",
					},
					"respectSchemaVersion": true,
				},
				"csharp": map[string]any{
					"packageReferences": map[string]any{
						"Pulumi":        "3.*",
						"Pulumi.Random": "4.*",
					},
					"respectSchemaVersion": true,
				},
			},
		},
	}

	opt.Components = append(opt.Components, registry.InferredComponents...)

	return infer.Provider(opt)
}
