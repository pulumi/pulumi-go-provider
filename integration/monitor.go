// Copyright 2022, Pulumi Corporation.
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
	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/internal/putil"

	presource "github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// MockCallArgs is used to construct a call Mock
type MockCallArgs struct {
	// Token indicates which function is being called. This token is of the form "package:module:function".
	Token tokens.ModuleMember
	// Args are the arguments provided to the function call.
	Args property.Map
	// Provider is the identifier of the provider instance being used to make the call.
	Provider p.ProviderReference
}

// MockResourceArgs is a used to construct a newResource Mock
type MockResourceArgs struct {
	// TypeToken is the token that indicates which resource type is being constructed. This token
	// is of the form "package:module:type".
	TypeToken tokens.Type
	// Name is the logical name of the resource instance.
	Name string
	// Inputs are the inputs for the resource.
	Inputs property.Map
	// Provider is the identifier of the provider instance being used to manage this resource.
	Provider p.ProviderReference
	// ID is the physical identifier of an existing resource to read or import.
	ID string
	// Custom specifies whether or not the resource is Custom (i.e. managed by a resource provider).
	Custom bool
	// Full register RPC call, if available.
	RegisterRPC *pulumirpc.RegisterResourceRequest
	// Full read RPC call, if available
	ReadRPC *pulumirpc.ReadResourceRequest
}

// MockResourceMonitor mocks resource registration and function calls.
type MockResourceMonitor struct {
	CallF        func(args MockCallArgs) (property.Map, error)
	NewResourceF func(args MockResourceArgs) (string, property.Map, error)
}

var _ pulumi.MockResourceMonitor = (*MockResourceMonitor)(nil)

func (m *MockResourceMonitor) Call(args pulumi.MockCallArgs) (presource.PropertyMap, error) {
	if m.CallF == nil {
		return presource.PropertyMap{}, nil
	}

	_args := MockCallArgs{
		Token: tokens.ModuleMember(args.Token),
		Args:  presource.FromResourcePropertyMap(args.Args),
		Provider: func() p.ProviderReference {
			urn, id, _ := putil.ParseProviderReference(args.Provider)
			return p.ProviderReference{
				Urn: urn,
				ID:  id,
			}
		}(),
	}

	result, err := m.CallF(_args)
	if err != nil {
		return presource.PropertyMap{}, err
	}
	return presource.ToResourcePropertyMap(result), nil
}

func (m *MockResourceMonitor) NewResource(args pulumi.MockResourceArgs) (string, presource.PropertyMap, error) {
	if m.NewResourceF == nil {
		return args.Name, args.Inputs, nil
	}

	_args := MockResourceArgs{
		TypeToken: tokens.Type(args.TypeToken),
		Name:      args.Name,
		Inputs:    presource.FromResourcePropertyMap(args.Inputs),
		Provider: func() p.ProviderReference {
			urn, id, _ := putil.ParseProviderReference(args.Provider)
			return p.ProviderReference{
				Urn: urn,
				ID:  id,
			}
		}(),
		ID:          args.ID,
		Custom:      args.Custom,
		RegisterRPC: args.RegisterRPC,
		ReadRPC:     args.ReadRPC,
	}

	id, state, err := m.NewResourceF(_args)
	return id, presource.ToResourcePropertyMap(state), err
}
