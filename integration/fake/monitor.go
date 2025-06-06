// Copyright 2025, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package fake

import (
	"context"
	"fmt"
	"sync"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"golang.org/x/exp/maps"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

type SimpleMonitor struct {
	CallF        func(args pulumi.MockCallArgs) (resource.PropertyMap, error)
	NewResourceF func(args pulumi.MockResourceArgs) (string, resource.PropertyMap, error)
}

func (m *SimpleMonitor) Call(args pulumi.MockCallArgs) (resource.PropertyMap, error) {
	if m.CallF == nil {
		return resource.PropertyMap{}, nil
	}
	return m.CallF(args)
}

func (m *SimpleMonitor) NewResource(args pulumi.MockResourceArgs) (string, resource.PropertyMap, error) {
	if m.NewResourceF == nil {
		return args.Name, args.Inputs, nil
	}
	return m.NewResourceF(args)
}

func StartMonitorServer(ctx context.Context, monitor pulumirpc.ResourceMonitorServer) (addr string, done <-chan error,
	err error) {
	cancel := make(chan bool)
	go func() {
		<-ctx.Done()
		close(cancel)
	}()

	handle, err := rpcutil.ServeWithOptions(rpcutil.ServeOptions{
		Cancel: cancel,
		Init: func(srv *grpc.Server) error {
			pulumirpc.RegisterResourceMonitorServer(srv, monitor)
			return nil
		},
		Options: rpcutil.OpenTracingServerInterceptorOptions(nil),
	})
	if err != nil {
		return "", nil, err
	}

	return fmt.Sprintf("127.0.0.1:%v", handle.Port), handle.Done, nil
}

func NewResourceMonitorServer(monitor pulumi.MockResourceMonitor) *ResourceMonitorServer {
	return &ResourceMonitorServer{
		project:       "project",
		stack:         "stack",
		mocks:         monitor,
		registrations: map[string]Registration{},
	}
}

type Registration struct {
	Urn     string
	ID      string
	State   resource.PropertyMap
	Request pulumirpc.RegisterResourceRequest
}

type ResourceMonitorServer struct {
	pulumirpc.UnimplementedResourceMonitorServer
	project string
	stack   string
	mocks   pulumi.MockResourceMonitor

	mu            sync.Mutex
	registrations map[string]Registration
}

// Registrations returns the resource registrations.
func (m *ResourceMonitorServer) Registrations() map[string]Registration {
	m.mu.Lock()
	defer m.mu.Unlock()
	return maps.Clone(m.registrations)
}

var _ pulumirpc.ResourceMonitorServer = &ResourceMonitorServer{}

func (m *ResourceMonitorServer) newURN(parent, typ, name string) string {
	parentType := tokens.Type("")
	if parentURN := resource.URN(parent); parentURN != "" && parentURN.QualifiedType() != resource.RootStackType {
		parentType = parentURN.QualifiedType()
	}

	return string(resource.NewURN(tokens.QName(m.stack), tokens.PackageName(m.project), parentType, tokens.Type(typ),
		name))
}

func (m *ResourceMonitorServer) SupportsFeature(context.Context, *pulumirpc.SupportsFeatureRequest,
) (*pulumirpc.SupportsFeatureResponse, error) {
	return &pulumirpc.SupportsFeatureResponse{
		HasSupport: true,
	}, nil
}

func (m *ResourceMonitorServer) RegisterResourceOutputs(context.Context, *pulumirpc.RegisterResourceOutputsRequest,
) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (m *ResourceMonitorServer) Invoke(ctx context.Context, req *pulumirpc.ResourceInvokeRequest,
) (*pulumirpc.InvokeResponse, error) {
	args, err := plugin.UnmarshalProperties(req.GetArgs(), plugin.MarshalOptions{
		SkipNulls:        false,
		KeepUnknowns:     true,
		KeepSecrets:      true,
		KeepOutputValues: true,
		KeepResources:    true,
	})
	if err != nil {
		return nil, err
	}

	result, err := m.mocks.Call(pulumi.MockCallArgs{
		Token:    req.GetTok(),
		Args:     args,
		Provider: req.Provider,
	})
	if err != nil {
		return nil, err
	}

	resultOut, err := plugin.MarshalProperties(result, plugin.MarshalOptions{
		KeepSecrets:   true,
		KeepResources: true,
	})
	if err != nil {
		return nil, err
	}

	return &pulumirpc.InvokeResponse{
		Return: resultOut,
	}, nil
}

func (m *ResourceMonitorServer) RegisterResource(ctx context.Context, in *pulumirpc.RegisterResourceRequest,
) (*pulumirpc.RegisterResourceResponse, error) {
	if in.GetType() == string(resource.RootStackType) && in.GetParent() == "" {
		return &pulumirpc.RegisterResourceResponse{
			Urn: m.newURN(in.GetParent(), in.GetType(), in.GetName()),
		}, nil
	}

	inputs, err := plugin.UnmarshalProperties(in.GetObject(), plugin.MarshalOptions{
		SkipNulls:        false,
		KeepUnknowns:     true,
		KeepSecrets:      true,
		KeepOutputValues: true,
		KeepResources:    true,
	})
	if err != nil {
		return nil, err
	}

	id, state, err := m.mocks.NewResource(pulumi.MockResourceArgs{
		TypeToken:   in.GetType(),
		Name:        in.GetName(),
		Inputs:      inputs,
		Provider:    in.GetProvider(),
		ID:          in.GetImportId(),
		Custom:      in.GetCustom(),
		RegisterRPC: in,
	})
	if err != nil {
		return nil, err
	}

	urn := m.newURN(in.GetParent(), in.GetType(), in.GetName())

	func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		m.registrations[urn] = Registration{
			Urn:     urn,
			ID:      id,
			State:   state,
			Request: *in, //nolint:govet // copylocks
		}
	}()

	stateOut, err := plugin.MarshalProperties(state, plugin.MarshalOptions{
		KeepSecrets:   true,
		KeepResources: true,
	})
	if err != nil {
		return nil, err
	}

	return &pulumirpc.RegisterResourceResponse{
		Urn:    urn,
		Id:     id,
		Object: stateOut,
	}, nil
}
