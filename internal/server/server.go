package server

import (
	"context"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/resource/provider"
	rpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

type URN = string

type Server struct {
	version semver.Version
	Host    *provider.HostClient
	Schema  []byte
}

// GetSchema fetches the schema for this resource provider.
func (s *Server) GetSchema(context.Context, *rpc.GetSchemaRequest) (*rpc.GetSchemaResponse, error) {
	response := &rpc.GetSchemaResponse{
		Schema: string(s.Schema),
	}

	return response, nil
}

// CheckConfig validates the configuration for this resource provider.
func (s *Server) CheckConfig(context.Context, *rpc.CheckRequest) (*rpc.CheckResponse, error) {
	return nil, status.Error(codes.Unimplemented, "CheckConfig is not yet implemented")
}

// DiffConfig checks the impact a hypothetical change to this provider's configuration will have on the provider.
func (s *Server) DiffConfig(context.Context, *rpc.DiffRequest) (*rpc.DiffResponse, error) {
	return nil, status.Error(codes.Unimplemented, "DiffConfig is not yet implemented")
}

// Configure configures the resource provider with "globals" that control its behavior.
func (s *Server) Configure(context.Context, *rpc.ConfigureRequest) (*rpc.ConfigureResponse, error) {
	return nil, status.Error(codes.Unimplemented, "Configure is not yet implemented")
}

// Invoke dynamically executes a built-in function in the provider.
func (s *Server) Invoke(context.Context, *rpc.InvokeRequest) (*rpc.InvokeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "Invoke is not yet implemented")
}

// StreamInvoke dynamically executes a built-in function in the provider, which returns a stream
// of responses.
func (s *Server) StreamInvoke(*rpc.InvokeRequest, rpc.ResourceProvider_StreamInvokeServer) error {
	return status.Error(codes.Unimplemented, "StreamInvoke is not yet implemented")
}

// Call dynamically executes a method in the provider associated with a component resource.
func (s *Server) Call(context.Context, *rpc.CallRequest) (*rpc.CallResponse, error) {
	return nil, status.Error(codes.Unimplemented, "Call is not yet implemented")
}

// Check validates that the given property bag is valid for a resource of the given type and returns the inputs
// that should be passed to successive calls to Diff, Create, or Update for this resource. As a rule, the provider
// inputs returned by a call to Check should preserve the original representation of the properties as present in
// the program inputs. Though this rule is not required for correctness, violations thereof can negatively impact
// the end-user experience, as the provider inputs are using for detecting and rendering diffs.
func (s *Server) Check(context.Context, *rpc.CheckRequest) (*rpc.CheckResponse, error) {
	return nil, status.Error(codes.Unimplemented, "Check is not yet implemented")
}

// Diff checks what impacts a hypothetical update will have on the resource's properties.
func (s *Server) Diff(context.Context, *rpc.DiffRequest) (*rpc.DiffResponse, error) {
	return nil, status.Error(codes.Unimplemented, "Diff is not yet implemented")
}

// Create allocates a new instance of the provided resource and returns its unique ID afterwards.  (The input ID
// must be blank.)  If this call fails, the resource must not have been created (i.e., it is "transactional").
func (s *Server) Create(context.Context, *rpc.CreateRequest) (*rpc.CreateResponse, error) {
	return nil, status.Error(codes.Unimplemented, "Create is not yet implemented")
}

// Read the current live state associated with a resource.  Enough state must be include in the inputs to uniquely
// identify the resource; this is typically just the resource ID, but may also include some properties.
func (s *Server) Read(context.Context, *rpc.ReadRequest) (*rpc.ReadResponse, error) {
	return nil, status.Error(codes.Unimplemented, "Read is not yet implemented")
}

// Update updates an existing resource with new values.
func (s *Server) Update(context.Context, *rpc.UpdateRequest) (*rpc.UpdateResponse, error) {
	return nil, status.Error(codes.Unimplemented, "Update is not yet implemented")
}

// Delete tears down an existing resource with the given ID.  If it fails, the resource is assumed to still exist.
func (s *Server) Delete(context.Context, *rpc.DeleteRequest) (*emptypb.Empty, error) {
	return nil, status.Error(codes.Unimplemented, "Delete is not yet implemented")
}

// Construct creates a new instance of the provided component resource and returns its state.
func (s *Server) Construct(context.Context, *rpc.ConstructRequest) (*rpc.ConstructResponse, error) {
	return nil, status.Error(codes.Unimplemented, "Construct is not yet implemented")
}

// Cancel signals the provider to gracefully shut down and abort any ongoing resource operations.
// Operations aborted in this way will return an error (e.g., `Update` and `Create` will either return a
// creation error or an initialization error). Since Cancel is advisory and non-blocking, it is up
// to the host to decide how long to wait after Cancel is called before (e.g.)
// hard-closing any gRPC connection.
func (s *Server) Cancel(context.Context, *emptypb.Empty) (*emptypb.Empty, error) {
	return nil, status.Error(codes.Unimplemented, "Cancel is not yet implemented")
}

// GetPluginInfo returns generic information about this plugin, like its version.
func (s *Server) GetPluginInfo(context.Context, *emptypb.Empty) (*rpc.PluginInfo, error) {
	return &rpc.PluginInfo{
		Version: s.version.String(),
	}, nil
}

// Attach sends the engine address to an already running plugin.
func (s *Server) Attach(context.Context, *rpc.PluginAttach) (*emptypb.Empty, error) {
	return nil, status.Error(codes.Unimplemented, "Attach is not yet implemented")
}
