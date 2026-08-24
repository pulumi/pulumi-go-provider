// Copyright 2026, Pulumi Corporation.
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

// This provider reproduces https://github.com/pulumi/pulumi-go-provider/issues/565: a
// resource whose live "members" output is managed out-of-band and so never matches the
// program's (absent) input. With `ignoreChanges: ["members"]` the engine must see no
// diff, and Update must never observe the old output value as an input.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/pulumi/pulumi-go-provider/infer"
)

type Team struct{}

type TeamArgs struct {
	Slug    string   `pulumi:"slug"`
	Members []string `pulumi:"members,optional"`
}

type TeamState struct {
	Slug    string   `pulumi:"slug"`
	Members []string `pulumi:"members"`
}

func (*Team) Create(
	ctx context.Context, req infer.CreateRequest[TeamArgs],
) (infer.CreateResponse[TeamState], error) {
	return infer.CreateResponse[TeamState]{
		ID: req.Name,
		Output: TeamState{
			Slug: req.Inputs.Slug,
			// Simulate drift: membership is reconciled externally, so the live
			// list never matches the program's (absent) input.
			Members: []string{"external"},
		},
	}, nil
}

var _ = (infer.CustomUpdate[TeamArgs, TeamState])((*Team)(nil))

func (*Team) Update(
	ctx context.Context, req infer.UpdateRequest[TeamArgs, TeamState],
) (infer.UpdateResponse[TeamState], error) {
	// "members" is under ignoreChanges: the engine resets it to the old input
	// (absent). Seeing a value here means the provider re-injected the old
	// *output* from state, which is the bug pinned by this test.
	if req.Inputs.Members != nil {
		return infer.UpdateResponse[TeamState]{}, fmt.Errorf(
			"update saw members %v as an input; ignored properties must keep their old input (absent)",
			req.Inputs.Members)
	}
	return infer.UpdateResponse[TeamState]{
		Output: TeamState{
			Slug:    req.Inputs.Slug,
			Members: req.State.Members,
		},
	}, nil
}

func main() {
	provider, err := infer.NewProviderBuilder().
		WithNamespace("pulumi").
		WithResources(infer.Resource(&Team{})).
		Build()
	if err == nil {
		err = provider.Run(context.Background(), "test", "0.1.0")
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err.Error())
		os.Exit(1)
	}
}
