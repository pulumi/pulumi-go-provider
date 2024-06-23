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

package infer

import (
	"fmt"
)

// ResourceInitFailedError indicates that the resource was created but failed to initialize.
//
// This error is treated specially in Create, Update and Read. If the returner error for a
// Create, Update or Read returns true for Errors.Is, state is updated to correspond to
// the accompanying state, the resource will be considered created, and the next call will
// be Update with the new state.
//
//	func (*Team) Create(
//		ctx context.Context, name string, input TeamInput, preview bool,
//	) (id string, output TeamState, err error) {
//		team, err := GetConfig(ctx).Client.CreateTeam(ctx,
//			input.OrganizationName, input.Name,
//			input.TeamType, input.DisplayName,
//			input.Description, input.GithubTeamId)
//		if err != nil {
//			return "", TeamState{}, fmt.Errorf("error creating team '%s': %w", input.Name, err)
//		}
//
//		if membersAdded, err := addMembers(team, input.Members); err != nil {
//			return TeamState{
//				Input: input,
//				Members: membersAdded,
//			}, infer.ResourceInitFailedError{Reasons: []string{
//				fmt.Sprintf("Failed to add members: %s", err),
//			}}
//		}
//
//		return TeamState{input, input.Members}, nil
//	}
//
// If the the above example errors with [infer.ResourceInitFailedError], the next Update
// will be called with `state` equal to what was returned alongside
// [infer.ResourceInitFailedError].
type ResourceInitFailedError struct {
	Reasons []string
}

func (err ResourceInitFailedError) Error() string { return "resource failed to initialize" }

// ProviderError indicates a bug in the provider implementation.
//
// When displayed, ProviderError tells the user that the issue was internal and should be
// reported.
type ProviderError struct {
	Inner error
}

// ProviderErrorf create a new [ProviderError].
//
// Arguments are formatted with [fmt.Errorf].
func ProviderErrorf(msg string, a ...any) error {
	return ProviderError{fmt.Errorf(msg, a...)}
}

func (err ProviderError) Error() string {
	const (
		prefix = "provider error"
		suffix = "; please report this to the provider author"
	)
	if err.Inner == nil {
		return prefix + suffix
	}
	return prefix + ": " + err.Inner.Error() + suffix
}
