// Copyright 2016-2024, Pulumi Corporation.
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

package integration

import (
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

// HostClient is the interface that the provider uses to communicate with the Pulumi engine.
//
//nolint:golint // stutter
type HostClient interface {
	// Log logs a global message, including errors and warnings.
	Log(context context.Context, sev diag.Severity, urn resource.URN, msg string) error

	// LogStatus logs a global status message, including errors and warnings. Status messages will
	// appear in the `Info` column of the progress display, but not in the final output.
	LogStatus(context context.Context, sev diag.Severity, urn resource.URN, msg string) error

	// EngineConn provides the engine gRPC client connection.
	EngineConn() *grpc.ClientConn
}

type MockMonitor struct {
	CallF        func(args pulumi.MockCallArgs) (resource.PropertyMap, error)
	NewResourceF func(args pulumi.MockResourceArgs) (string, resource.PropertyMap, error)
}

func (m *MockMonitor) Call(args pulumi.MockCallArgs) (resource.PropertyMap, error) {
	if m.CallF == nil {
		return resource.PropertyMap{}, nil
	}
	return m.CallF(args)
}

func (m *MockMonitor) NewResource(args pulumi.MockResourceArgs) (string, resource.PropertyMap, error) {
	if m.NewResourceF == nil {
		return args.Name, args.Inputs, nil
	}
	return m.NewResourceF(args)
}
