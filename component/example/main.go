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
	"github.com/pulumi/pulumi-go-provider/component"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// Register all the types that this provider supports.
// TODO: This should be scafolded with code generation.
func init() {
	component.RegisterType(infer.Component[*RandomComponent, RandomComponentArgs, *RandomComponent]())
}

// Scafold a wrapped Construct method.
func (r *RandomComponent) Construct(ctx *pulumi.Context, name, typ string, args RandomComponentArgs, opts pulumi.ResourceOption) (*RandomComponent, error) {
	return NewMyComponent(ctx, name, args)
}

func main() {
	// Start the provider host using source code in the current directory.
	component.ProviderHost()
}
