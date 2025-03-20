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

// package main shows how a [component] based provider can be created.
package main

import (
	"example/nested"

	"github.com/pulumi/pulumi-go-provider/component"
)

func init() {
	component.RegisterType(component.ProgramComponent(
		component.ConstructorFn[nested.NestedRandomComponentArgs, *nested.NestedRandomComponent](nested.CreateNestedRandomComponent)))
	component.RegisterType(component.ProgramComponent(
		component.ConstructorFn[RandomComponentArgs, *RandomComponent](NewMyComponent)))
}

func main() {
	component.ProviderHost("go-components", "0.0.1")
}
