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
	"context"
	"fmt"
	"os"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/examples/component-provider/nested"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	provider, err := provider()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s", err.Error())
		os.Exit(1)
	}

	err = provider.Run(context.Background(), "go-components", "0.1.0")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s", err.Error())
		os.Exit(1)
	}
}

func provider() (p.Provider, error) {
	return infer.NewProviderBuilder().
		WithNamespace("examples").
		WithComponents(
			infer.ComponentF(NewMyComponent),
			infer.ComponentF(nested.NewNestedRandomComponent),
			infer.ComponentF(func(ctx *pulumi.Context, name string, args DemoArgs, opts ...pulumi.ResourceOption) (*Repro, error) {
				var comp Repro
				err := ctx.RegisterComponentResource(p.GetTypeToken(ctx.Context()), name, &comp, opts...)
				if err != nil {
					return nil, err
				}
				comp.NilOutputString = pulumi.StringPtrOutput{}
				return &comp, nil
			}),
		).
		Build()
}

type DemoArgs struct{}

type Repro struct {
	pulumi.ResourceState

	NilOutputString pulumi.StringPtrOutput `pulumi:"s,optional"`
}
