// Copyright 2025, Pulumi Corporation.
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

package provider

import (
	"context"
	"errors"

	comProvider "github.com/pulumi/pulumi/sdk/v3/go/pulumi/provider"
)

var ErrNoHost = errors.New("no provider host found in context")

// ProgramConstruct is a convenience function for a Provider to implement [Provider.Construct] using the Pulumi Go SDK.
//
// Call this method from within your [Provider.Construct] method to transition into a Go SDK program context.
func ProgramConstruct(ctx context.Context, req ConstructRequest, constructF comProvider.ConstructFunc,
) (ConstructResponse, error) {
	host := GetHost(ctx)
	if host == nil {
		return ConstructResponse{}, ErrNoHost
	}
	return host.Construct(ctx, req, constructF)
}
