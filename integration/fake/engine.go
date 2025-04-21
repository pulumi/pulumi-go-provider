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

// Package fake implements a fake [pulumirpc.EngineServer] and [pulumirpc.ResourceMonitorServer]
// for integration test purposes.
package fake

import (
	"context"
	"fmt"
	"sync"

	pbempty "github.com/golang/protobuf/ptypes/empty"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func NewEngineServer() *EngineServer {
	return &EngineServer{}
}

func StartEngineServer(ctx context.Context, engine pulumirpc.EngineServer) (addr string, done <-chan error, err error) {
	cancel := make(chan bool)
	go func() {
		<-ctx.Done()
		close(cancel)
	}()
	handle, err := rpcutil.ServeWithOptions(rpcutil.ServeOptions{
		Cancel: cancel,
		Init: func(srv *grpc.Server) error {
			pulumirpc.RegisterEngineServer(srv, engine)
			return nil
		},
		Options: rpcutil.OpenTracingServerInterceptorOptions(nil),
	})
	if err != nil {
		return "", nil, err
	}

	go func() {
		err := <-handle.Done
		if err != nil {
			panic(fmt.Errorf("engine server failed: %w", err))
		}
	}()

	return fmt.Sprintf("127.0.0.1:%v", handle.Port), handle.Done, nil
}

func NewEngineConn(addr string) *grpc.ClientConn {
	conn, err := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(rpcutil.OpenTracingClientInterceptor()),
		grpc.WithStreamInterceptor(rpcutil.OpenTracingStreamClientInterceptor()),
		rpcutil.GrpcChannelOptions(),
	)
	if err != nil {
		panic(fmt.Errorf("could not create engine client: %w", err))
	}
	return conn
}

type EngineServer struct {
	pulumirpc.UnimplementedEngineServer

	mu           sync.Mutex
	rootResource string
	logs         []*pulumirpc.LogRequest
}

func (m *EngineServer) Logs() []*pulumirpc.LogRequest {
	m.mu.Lock()
	defer m.mu.Unlock()
	var logs []*pulumirpc.LogRequest
	logs = append(logs, m.logs...)
	return logs
}

var _ pulumirpc.EngineServer = &EngineServer{}

// Log logs a global message in the engine, including errors and warnings.
func (m *EngineServer) Log(ctx context.Context, in *pulumirpc.LogRequest) (*pbempty.Empty, error) {
	m.mu.Lock()
	m.logs = append(m.logs, in)
	m.mu.Unlock()
	return &pbempty.Empty{}, nil
}

// GetRootResource gets the URN of the root resource, the resource that should be the root of all
// otherwise-unparented resources.
func (m *EngineServer) GetRootResource(ctx context.Context, in *pulumirpc.GetRootResourceRequest,
) (*pulumirpc.GetRootResourceResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return &pulumirpc.GetRootResourceResponse{
		Urn: m.rootResource,
	}, nil
}

// SetRootResource sets the URN of the root resource.
func (m *EngineServer) SetRootResource(ctx context.Context, in *pulumirpc.SetRootResourceRequest,
) (*pulumirpc.SetRootResourceResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rootResource = in.GetUrn()
	return &pulumirpc.SetRootResourceResponse{}, nil
}
