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

package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	rpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
)

// debugServer wraps a ResourceProviderServer and logs all gRPC calls and responses
// to a JSON file specified by the PULUMI_GO_PROVIDER_DEBUG_GRPC environment variable.
type debugServer struct {
	rpc.ResourceProviderServer
	file *os.File
	mu   sync.Mutex
}

// newDebugServer creates a new debug wrapper around the given server.
// The filePath specifies where to write the debug logs.
func newDebugServer(server rpc.ResourceProviderServer, filePath string) (*debugServer, error) {
	file, err := os.Create(filepath.Clean(filePath)) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("failed to open debug file %s: %w", filePath, err)
	}

	return &debugServer{
		ResourceProviderServer: server,
		file:                   file,
	}, nil
}

// logEntry represents a single gRPC call/response pair in the debug log.
type logEntry struct {
	Method   string          `json:"method"`
	Request  json.RawMessage `json:"request"`
	Response json.RawMessage `json:"response,omitempty"`
	Error    string          `json:"error,omitempty"`
}

// log writes a gRPC call entry to the debug file.
func (d *debugServer) log(method string, req, resp proto.Message, err error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	marshaler := protojson.MarshalOptions{
		EmitUnpopulated: false,
		UseProtoNames:   true,
	}

	entry := logEntry{
		Method: method,
	}

	if req != nil {
		reqJSON, marshalErr := marshaler.Marshal(req)
		if marshalErr != nil {
			fmt.Fprintf(os.Stderr, "failed to marshal request for %s: %v\n", method, marshalErr)
		} else {
			entry.Request = reqJSON
		}
	}

	if resp != nil {
		respJSON, marshalErr := marshaler.Marshal(resp)
		if marshalErr != nil {
			fmt.Fprintf(os.Stderr, "failed to marshal response for %s: %v\n", method, marshalErr)
		} else {
			entry.Response = respJSON
		}
	}

	if err != nil {
		entry.Error = err.Error()
	}

	// Write the entry as a JSON line
	encoder := json.NewEncoder(d.file)
	if encodeErr := encoder.Encode(entry); encodeErr != nil {
		fmt.Fprintf(os.Stderr, "failed to write debug log entry for %s: %v\n", method, encodeErr)
	}
}

// GetSchema wraps the GetSchema call with debug logging.
func (d *debugServer) GetSchema(ctx context.Context, req *rpc.GetSchemaRequest) (*rpc.GetSchemaResponse, error) {
	resp, err := d.ResourceProviderServer.GetSchema(ctx, req)
	d.log("/pulumirpc.ResourceProvider/GetSchema", req, resp, err)
	return resp, err
}

// CheckConfig wraps the CheckConfig call with debug logging.
func (d *debugServer) CheckConfig(ctx context.Context, req *rpc.CheckRequest) (*rpc.CheckResponse, error) {
	resp, err := d.ResourceProviderServer.CheckConfig(ctx, req)
	d.log("/pulumirpc.ResourceProvider/CheckConfig", req, resp, err)
	return resp, err
}

// DiffConfig wraps the DiffConfig call with debug logging.
func (d *debugServer) DiffConfig(ctx context.Context, req *rpc.DiffRequest) (*rpc.DiffResponse, error) {
	resp, err := d.ResourceProviderServer.DiffConfig(ctx, req)
	d.log("/pulumirpc.ResourceProvider/DiffConfig", req, resp, err)
	return resp, err
}

// Configure wraps the Configure call with debug logging.
func (d *debugServer) Configure(ctx context.Context, req *rpc.ConfigureRequest) (*rpc.ConfigureResponse, error) {
	resp, err := d.ResourceProviderServer.Configure(ctx, req)
	d.log("/pulumirpc.ResourceProvider/Configure", req, resp, err)
	return resp, err
}

// Check wraps the Check call with debug logging.
func (d *debugServer) Check(ctx context.Context, req *rpc.CheckRequest) (*rpc.CheckResponse, error) {
	resp, err := d.ResourceProviderServer.Check(ctx, req)
	d.log("/pulumirpc.ResourceProvider/Check", req, resp, err)
	return resp, err
}

// Diff wraps the Diff call with debug logging.
func (d *debugServer) Diff(ctx context.Context, req *rpc.DiffRequest) (*rpc.DiffResponse, error) {
	resp, err := d.ResourceProviderServer.Diff(ctx, req)
	d.log("/pulumirpc.ResourceProvider/Diff", req, resp, err)
	return resp, err
}

// Create wraps the Create call with debug logging.
func (d *debugServer) Create(ctx context.Context, req *rpc.CreateRequest) (*rpc.CreateResponse, error) {
	resp, err := d.ResourceProviderServer.Create(ctx, req)
	d.log("/pulumirpc.ResourceProvider/Create", req, resp, err)
	return resp, err
}

// Read wraps the Read call with debug logging.
func (d *debugServer) Read(ctx context.Context, req *rpc.ReadRequest) (*rpc.ReadResponse, error) {
	resp, err := d.ResourceProviderServer.Read(ctx, req)
	d.log("/pulumirpc.ResourceProvider/Read", req, resp, err)
	return resp, err
}

// Update wraps the Update call with debug logging.
func (d *debugServer) Update(ctx context.Context, req *rpc.UpdateRequest) (*rpc.UpdateResponse, error) {
	resp, err := d.ResourceProviderServer.Update(ctx, req)
	d.log("/pulumirpc.ResourceProvider/Update", req, resp, err)
	return resp, err
}

// Delete wraps the Delete call with debug logging.
func (d *debugServer) Delete(ctx context.Context, req *rpc.DeleteRequest) (*emptypb.Empty, error) {
	resp, err := d.ResourceProviderServer.Delete(ctx, req)
	d.log("/pulumirpc.ResourceProvider/Delete", req, resp, err)
	return resp, err
}

// Invoke wraps the Invoke call with debug logging.
func (d *debugServer) Invoke(ctx context.Context, req *rpc.InvokeRequest) (*rpc.InvokeResponse, error) {
	resp, err := d.ResourceProviderServer.Invoke(ctx, req)
	d.log("/pulumirpc.ResourceProvider/Invoke", req, resp, err)
	return resp, err
}

// Construct wraps the Construct call with debug logging.
func (d *debugServer) Construct(ctx context.Context, req *rpc.ConstructRequest) (*rpc.ConstructResponse, error) {
	resp, err := d.ResourceProviderServer.Construct(ctx, req)
	d.log("/pulumirpc.ResourceProvider/Construct", req, resp, err)
	return resp, err
}

// Call wraps the Call call with debug logging.
func (d *debugServer) Call(ctx context.Context, req *rpc.CallRequest) (*rpc.CallResponse, error) {
	resp, err := d.ResourceProviderServer.Call(ctx, req)
	d.log("/pulumirpc.ResourceProvider/Call", req, resp, err)
	return resp, err
}

// Cancel wraps the Cancel call with debug logging.
func (d *debugServer) Cancel(ctx context.Context, req *emptypb.Empty) (*emptypb.Empty, error) {
	resp, err := d.ResourceProviderServer.Cancel(ctx, req)
	d.log("/pulumirpc.ResourceProvider/Cancel", req, resp, err)
	return resp, err
}
