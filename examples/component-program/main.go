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

// package main shows how a simple comoponent provider can be created using existing
// Pulumi programs that contain components.
package main

import (
	"github.com/pulumi/pulumi-go-provider/component"
	"github.com/pulumi/pulumi-go-provider/examples/component-program/nested"
)

func main() {
	err := component.ProviderHost(
		component.WithName("go-components"),
		component.WithVersion("v0.0.1"),
		component.WithResources(
			component.ProgramComponent(
				component.ConstructorFn[RandomComponentArgs, *RandomComponent](NewMyComponent)),
			component.ProgramComponent(
				component.ConstructorFn[nested.NestedRandomComponentArgs, *nested.NestedRandomComponent](nested.CreateNestedRandomComponent))),
	)

	if err != nil {
		panic(err)
	}
}
