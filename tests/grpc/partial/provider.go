// Copyright 2024, Pulumi Corporation.
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

package partial

import (
	"context"

	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

func Provider() p.Provider {
	return infer.Provider(infer.Options{
		Resources: []infer.InferredResource{infer.Resource[*Partial, Args, State]()},
		ModuleMap: map[tokens.ModuleName]tokens.ModuleName{
			"partial": "index",
		},
	})
}

var (
	_ infer.CustomResource[Args, State] = (*Partial)(nil)
	_ infer.CustomUpdate[Args, State]   = (*Partial)(nil)
	_ infer.CustomRead[Args, State]     = (*Partial)(nil)
)

type Partial struct{}
type Args struct {
	S string `pulumi:"s"`
}
type State struct {
	Args

	Out string `pulumi:"out"`
}

func (*Partial) Create(ctx context.Context, name string, input Args, preview bool) (string, State, error) {
	if preview {
		return "", State{}, nil
	}
	contract.Assertf(input.S == "for-create", `expected input.S to be "for-create"`)
	return "id", State{
			Args: Args{S: "+for-create"},
			Out:  "partial-create",
		}, infer.ResourceInitFailedError{
			Reasons: []string{"create: failed to fully init"},
		}
}

func (*Partial) Update(ctx context.Context, id string, olds State, news Args, preview bool) (State, error) {
	if preview {
		return State{}, nil
	}
	contract.Assertf(news.S == "for-update", `expected news.S to be "for-update"`)
	contract.Assertf(olds.S == "+for-create", `expected olds.Out to be "partial-create"`)
	contract.Assertf(olds.Out == "partial-init", `expected olds.Out to be "partial-create"`)

	return State{
			Args: Args{
				S: "from-update",
			},
			Out: "partial-update",
		}, infer.ResourceInitFailedError{
			Reasons: []string{"update: failed to continue init"},
		}
}

func (*Partial) Read(ctx context.Context, id string, inputs Args, state State) (
	canonicalID string, normalizedInputs Args, normalizedState State, err error) {
	contract.Assertf(inputs.S == "for-read", `expected inputs.S to be "for-read"`)
	contract.Assertf(state.S == "from-update", `expected olds.Out to be "partial-create"`)
	contract.Assertf(state.Out == "state-for-read", `expected state.Out to be "state-for-read"`)

	return "from-read-id", Args{
			S: "from-read-input",
		}, State{
			Args: Args{"s-state-from-read"},
			Out:  "out-state-from-read",
		}, infer.ResourceInitFailedError{
			Reasons: []string{"read: failed to finish read"},
		}
}
