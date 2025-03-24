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

// package main shows how a simple component provider can be created using existing
// Pulumi programs that contain components.
package main

import (
	"fmt"
	"os"

	"github.com/pulumi/pulumi-go-provider/examples/component-program/nested"
	"github.com/pulumi/pulumi-go-provider/infer"
)

func main() {
	err := infer.Default().
		WithName("go-components").
		WithVersion("v0.0.1").
		WithComponent(
			infer.ProgramComponent(NewMyComponent),
			infer.ProgramComponent(nested.CreateNestedRandomComponent),
		).BuildAndRun()

	if err != nil {
		fmt.Fprintln(os.Stderr, "Error: "+err.Error())
		os.Exit(1)
	}
}
