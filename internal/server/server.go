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

package server

import (
	"context"
	"sort"
	"time"

	"github.com/blang/semver"
	pprovider "github.com/pulumi/pulumi/pkg/v3/resource/provider"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/provider"
	rpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/pulumi/pulumi-go-provider/internal/introspect"
	r "github.com/pulumi/pulumi-go-provider/resource"
)

type Server struct {
	Name    string
	Version semver.Version
	host    *pprovider.HostClient
	Schema  string

	components ComponentResources
	customs    CustomResources
	invokes    Invokes
}

func New(name string, version semver.Version, host *pprovider.HostClient,
	components ComponentResources, customs CustomResources, invokes Invokes, schema string) *Server {
	return &Server{
		Name:       name,
		Version:    version,
		host:       host,
		Schema:     schema,
		components: components,
		customs:    customs,
		invokes:    invokes,
	}
}

// GetSchema fetches the schema for this resource provider.
func (s *Server) GetSchema(context.Context, *rpc.GetSchemaRequest) (*rpc.GetSchemaResponse, error) {
	response := &rpc.GetSchemaResponse{
		Schema: s.Schema,
	}

	return response, nil
}

// CheckConfig validates the configuration for this resource provider.
func (s *Server) CheckConfig(_ context.Context, req *rpc.CheckRequest) (*rpc.CheckResponse, error) {
	return &rpc.CheckResponse{Inputs: req.GetNews()}, nil
}

// DiffConfig checks the impact a hypothetical change to this provider's configuration will have on the provider.
func (s *Server) DiffConfig(context.Context, *rpc.DiffRequest) (*rpc.DiffResponse, error) {
	return nil, status.Error(codes.Unimplemented, "DiffConfig is not yet implemented")
}

// Configure configures the resource provider with "globals" that control its behavior.
func (s *Server) Configure(context.Context, *rpc.ConfigureRequest) (*rpc.ConfigureResponse, error) {
	return &rpc.ConfigureResponse{
		AcceptSecrets:   true,
		SupportsPreview: true,
	}, nil
}

// Invoke dynamically executes a built-in function in the provider.
func (s *Server) Invoke(ctx context.Context, req *rpc.InvokeRequest) (*rpc.InvokeResponse, error) {
	input, err := s.invokes.getInvokeInput(tokens.Type(req.GetTok()))
	if err != nil {
		return nil, err
	}

	// Some methods don't take an input, so we don't map their fields
	if input != nil {
		err = introspect.PropertiesToResource(req.GetArgs(), input)
		if err != nil {
			return nil, err
		}
	}

	result, err := s.invokes.call(ctx, tokens.Type(req.GetTok()), input)
	if err != nil {
		return nil, err
	}

	var ret *structpb.Struct
	if result != nil {
		ret, err = introspect.ResourceToProperties(result, nil)
		if err != nil {
			return nil, err
		}
	}
	return &rpc.InvokeResponse{
		Return: ret,
	}, nil
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
func (s *Server) Check(ctx context.Context, req *rpc.CheckRequest) (*rpc.CheckResponse, error) {
	typ := resource.URN(req.Urn).Type()
	custom, err := s.customs.GetCustom(typ)
	if err != nil {
		return nil, err
	}
	if res, ok := custom.(r.Check); ok {
		err = introspect.PropertiesToResource(req.GetOlds(), res)
		if err != nil {
			return nil, err
		}
		new, err := s.customs.GetCustom(typ)
		contract.AssertNoErrorf(err, "We already know a type is registered for %s since we retrieved it before", typ)

		err = introspect.PropertiesToResource(req.GetNews(), new)
		if err != nil {
			return nil, err
		}

		checkContext := r.NewContext(ctx, s.host, resource.URN(req.Urn), introspect.NewFieldMatcher(custom))
		failures, nErr := res.Check(checkContext, new, int(0))
		if err != nil {
			return nil, nErr
		}
		f := make([]*rpc.CheckFailure, len(failures))
		for i, e := range failures {
			f[i] = &rpc.CheckFailure{
				Property: e.Property,
				Reason:   e.Reason,
			}
		}

		// TODO: Swap new and old so old is the argument and new is the default
		inputs, err := introspect.ResourceToProperties(res, nil)
		if err != nil {
			return nil, err
		}
		return &rpc.CheckResponse{
			Inputs:   inputs,
			Failures: f,
		}, nil
	}

	// No check method was provided, so we dafault to doing nothing
	return &rpc.CheckResponse{
		Inputs:   req.GetNews(),
		Failures: nil,
	}, nil
}

// Diff checks what impacts a hypothetical update will have on the resource's properties.
func (s *Server) Diff(ctx context.Context, req *rpc.DiffRequest) (*rpc.DiffResponse, error) {
	typ := resource.URN(req.Urn).Type()
	custom, err := s.customs.GetCustom(typ)
	if err != nil {
		return nil, err
	}
	if custom, ok := custom.(r.Diff); ok {
		err := introspect.PropertiesToResource(req.GetOlds(), custom)
		if err != nil {
			return nil, err
		}

		new, err := s.customs.GetCustom(typ)
		contract.AssertNoErrorf(err, "We already know a type is registered for %s since we retrieved it before", typ)

		err = introspect.PropertiesToResource(req.GetNews(), new)
		if err != nil {
			return nil, err
		}
		diffContext := r.NewContext(ctx, s.host, resource.URN(req.Urn), introspect.NewFieldMatcher(custom))
		return custom.Diff(diffContext, req.GetId(), new, req.GetIgnoreChanges())
	}

	// The user has not provided a diff, so use the default diff
	marshalOptions := plugin.MarshalOptions{
		KeepUnknowns: true,
		SkipNulls:    true,
	}
	olds, err := plugin.UnmarshalProperties(req.GetOlds(), marshalOptions)
	if err != nil {
		return nil, err
	}

	news, err := plugin.UnmarshalProperties(req.GetNews(), marshalOptions)
	if err != nil {
		return nil, err
	}
	changes := rpc.DiffResponse_DIFF_NONE
	var diffs, replaces []string

	outputKeys, err := introspect.FindOutputProperties(custom)
	if err != nil {
		return nil, err
	}

	if d := olds.Diff(news); d != nil {
		for _, propKey := range d.ChangedKeys() {
			// We don't want to signal when we have a changed outpuit
			key := string(propKey)
			if outputKeys[key] {
				continue
			}
			i := sort.SearchStrings(req.IgnoreChanges, key)
			if i < len(req.IgnoreChanges) && req.IgnoreChanges[i] == key {
				continue
			}

			if d.Changed(resource.PropertyKey(key)) {
				changes = rpc.DiffResponse_DIFF_SOME
				diffs = append(diffs, key)

				if _, hasUpdate := custom.(r.Update); !hasUpdate {
					replaces = append(replaces, key)
				}
			}
		}
	}

	return &rpc.DiffResponse{
		Replaces: replaces,
		Changes:  changes,
		Diffs:    diffs,
	}, nil
}

// Create allocates a new instance of the provided resource and returns its unique ID afterwards.  (The input ID
// must be blank.)  If this call fails, the resource must not have been created (i.e., it is "transactional").
func (s *Server) Create(ctx context.Context, req *rpc.CreateRequest) (*rpc.CreateResponse, error) {
	urn := resource.URN(req.Urn)
	custom, err := s.customs.GetCustom(urn.Type())
	if err != nil {
		return nil, err
	}
	// We need to be careful not to take an unnecessary reference to custom here.
	err = introspect.PropertiesToResource(req.GetProperties(), custom)
	if err != nil {
		return nil, err
	}

	// Apply timeout
	if t := req.GetTimeout(); t != 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(t)*time.Second)
		defer cancel()
	}

	createContext := r.NewContext(ctx, s.host, resource.URN(req.Urn), introspect.NewFieldMatcher(custom))
	id, err := custom.Create(createContext, urn.Name().String(), req.GetPreview())
	if err != nil {
		return nil, err
	}

	opts := introspect.ToPropertiesOptions{}
	if req.GetPreview() {
		opts.ComputedKeys = createContext.ComputedKeys()
	}
	props, err := introspect.ResourceToProperties(custom, &opts)
	if err != nil {
		return nil, err
	}
	return &rpc.CreateResponse{
		Id:         id,
		Properties: props,
	}, nil
}

// Read the current live state associated with a resource.  Enough state must be include in the inputs to uniquely
// identify the resource; this is typically just the resource ID, but may also include some properties.
func (s *Server) Read(ctx context.Context, req *rpc.ReadRequest) (*rpc.ReadResponse, error) {
	custom, err := s.customs.GetCustom(resource.URN(req.Urn).Type())
	if err != nil {
		return nil, err
	}
	if custom, ok := custom.(r.Read); ok {
		err = introspect.PropertiesToResource(req.GetProperties(), custom)
		if err != nil {
			return nil, err
		}

		err = introspect.PropertiesToResource(req.GetInputs(), custom)
		if err != nil {
			return nil, err
		}

		readContext := r.NewContext(ctx, s.host, resource.URN(req.Urn), introspect.NewFieldMatcher(custom))
		err := custom.Read(readContext, req.GetId())
		if err != nil {
			return nil, err
		}

		props, err := introspect.ResourceToProperties(custom, nil)
		if err != nil {
			return nil, err
		}

		return &rpc.ReadResponse{
			Id:         req.GetId(),
			Properties: props,
		}, nil
	}

	return nil, status.Error(codes.Unimplemented, "Read is not yet implemented")
}

// Update updates an existing resource with new values.
func (s *Server) Update(ctx context.Context, req *rpc.UpdateRequest) (*rpc.UpdateResponse, error) {
	typ := resource.URN(req.Urn).Type()
	custom, err := s.customs.GetCustom(typ)
	if err != nil {
		return nil, err
	}
	if custom, ok := custom.(r.Update); ok {
		err = introspect.PropertiesToResource(req.GetOlds(), custom)
		if err != nil {
			return nil, err
		}
		new, err := s.customs.GetCustom(typ)
		contract.AssertNoErrorf(err, "We already know a type is registered for %s since we retrieved it before", typ)
		err = introspect.PropertiesToResource(req.GetNews(), new)
		if err != nil {
			return nil, err
		}

		updateContext := r.NewContext(ctx, s.host, resource.URN(req.Urn), introspect.NewFieldMatcher(custom))
		err = custom.Update(updateContext, req.Id, new, req.GetIgnoreChanges(), req.GetPreview())
		if err != nil {
			return nil, err
		}

		opts := introspect.ToPropertiesOptions{}
		if req.GetPreview() {
			opts.ComputedKeys = updateContext.ComputedKeys()
		}

		props, err := introspect.ResourceToProperties(custom, &opts)
		if err != nil {
			return nil, err
		}

		return &rpc.UpdateResponse{
			Properties: props,
		}, nil
	}

	return nil, status.Error(codes.Unimplemented, "Update is not yet implemented")
}

// Delete tears down an existing resource with the given ID.  If it fails, the resource is assumed to still exist.
func (s *Server) Delete(ctx context.Context, req *rpc.DeleteRequest) (*emptypb.Empty, error) {
	custom, err := s.customs.GetCustom(resource.URN(req.Urn).Type())
	if err != nil {
		return nil, err
	}

	err = introspect.PropertiesToResource(req.GetProperties(), custom)
	if err != nil {
		return nil, err
	}

	// Apply timeout
	if t := req.GetTimeout(); t != 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(t)*time.Second)
		defer cancel()
	}

	deleteContext := r.NewContext(ctx, s.host, resource.URN(req.Urn), introspect.NewFieldMatcher(custom))
	err = custom.Delete(deleteContext, req.Id)
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// Construct creates a new instance of the provided component resource and returns its state.
func (s *Server) Construct(ctx context.Context, request *rpc.ConstructRequest) (*rpc.ConstructResponse, error) {
	c, err := s.components.GetComponent(tokens.Type(request.Type))
	if err != nil {
		return nil, err
	}
	cR, err := provider.Construct(ctx, request, s.host.EngineConn(), componentFn(s.Name, c))
	return cR, err
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
		Version: s.Version.String(),
	}, nil
}

// Attach sends the engine address to an already running plugin.
func (s *Server) Attach(_ context.Context, req *rpc.PluginAttach) (*emptypb.Empty, error) {
	host, err := pprovider.NewHostClient(req.GetAddress())
	if err != nil {
		return nil, err
	}
	s.host = host
	return &emptypb.Empty{}, nil
}
