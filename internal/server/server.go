package server

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/blang/semver"
	pprovider "github.com/pulumi/pulumi/pkg/v3/resource/provider"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/mapper"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/provider"
	rpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/pulumi/pulumi-go-provider/internal/introspect"
	r "github.com/pulumi/pulumi-go-provider/resource"
)

func getToken(pkg tokens.Package, t interface{}) (tokens.Type, error) {
	typ := reflect.TypeOf(t)
	if typ == nil {
		return "", fmt.Errorf("Cannot get token of nil type")
	}

	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
		if typ == nil {
			return "", fmt.Errorf("Cannot get token of nil type")
		}
	}

	if typ.Kind() != reflect.Struct {
		return "", fmt.Errorf("Can only get tokens with underlying structs")
	}

	name := typ.Name()
	if name == "" {
		return "", fmt.Errorf("Type %T has no name", t)
	}
	mod := typ.PkgPath()
	if mod == "" {
		return "", fmt.Errorf("Type %T has no module path", t)
	}
	// Take off the pkg name, since that is supplied by `pkg`.
	mod = mod[strings.IndexRune(mod, '/')+1:]
	m := tokens.NewModuleToken(pkg, tokens.ModuleName(mod))
	return tokens.NewTypeToken(m, tokens.TypeName(name)), nil
}

func newOfType[T any](t T) T {
	typ := reflect.TypeOf(t)
	v := reflect.New(typ)
	return v.Interface().(T)
}

type Server struct {
	Name    string
	Version semver.Version
	Host    *pprovider.HostClient

	Components ComponentResources
	Customs    CustomResources
}

// GetSchema fetches the schema for this resource provider.
func (s *Server) GetSchema(context.Context, *rpc.GetSchemaRequest) (*rpc.GetSchemaResponse, error) {
	return nil, status.Error(codes.Unimplemented, "GetSchema is not yet implemented")
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
func (s *Server) Check(ctx context.Context, req *rpc.CheckRequest) (*rpc.CheckResponse, error) {
	custom, err := s.Customs.GetCustom(resource.URN(req.Urn).Type())
	if err != nil {
		return nil, err
	}
	if r, ok := custom.(r.ResourceCheck); ok {
		mapper := mapper.New(&mapper.Opts{
			IgnoreMissing: true,
		})
		err := mapper.Decode(req.GetOlds().AsMap(), &r)
		if err != nil {
			return nil, err
		}
		new := newOfType(custom)
		err = mapper.Decode(req.GetNews().AsMap(), &new)
		if err != nil {
			return nil, err
		}
		failures, nErr := r.Check(ctx, new, int(req.SequenceNumber))
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

		inputs, err := mapper.Encode(r)
		if err != nil {
			return nil, err
		}
		s, nErr := structpb.NewStruct(inputs)
		if nErr != nil {
			return nil, nErr
		}

		return &rpc.CheckResponse{
			Inputs:   s,
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
	custom, err := s.Customs.GetCustom(resource.URN(req.Urn).Type())
	if err != nil {
		return nil, err
	}
	if r, ok := custom.(r.ResourceDiff); ok {
		mapper := mapper.New(&mapper.Opts{
			IgnoreMissing: true,
		})
		err := mapper.Decode(req.GetOlds().AsMap(), &r)
		if err != nil {
			return nil, err
		}
		new := newOfType(custom)
		err = mapper.Decode(req.GetNews().AsMap(), &new)
		if err != nil {
			return nil, err
		}
		return r.Diff(ctx, req.GetId(), new, req.GetIgnoreChanges())
	}
	return nil, status.Error(codes.Unimplemented, "Diff is not yet implemented")
}

// Create allocates a new instance of the provided resource and returns its unique ID afterwards.  (The input ID
// must be blank.)  If this call fails, the resource must not have been created (i.e., it is "transactional").
func (s *Server) Create(ctx context.Context, req *rpc.CreateRequest) (*rpc.CreateResponse, error) {
	custom, err := s.Customs.GetCustom(resource.URN(req.Urn).Type())
	if err != nil {
		return nil, err
	}
	mapper := mapper.New(&mapper.Opts{
		IgnoreMissing: true,
	})
	err = mapper.Decode(req.GetProperties().AsMap(), &custom)
	if err != nil {
		return nil, err
	}
	id, err := custom.Create(ctx, req.Preview)
	if err != nil {
		return nil, err
	}
	props, err := introspect.ResourceToProperties(custom)
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
	custom, err := s.Customs.GetCustom(resource.URN(req.Urn).Type())
	if err != nil {
		return nil, err
	}
	if r, ok := custom.(r.ResourceRead); ok {
		mapper := mapper.New(&mapper.Opts{
			IgnoreMissing: true,
		})
		// NOTE: this only works if we can prioritize one set over the other
		// AND
		// mapper.Decode won't overwrite blanks.
		// TODO: make sure this works
		mapErr := mapper.Decode(req.GetInputs().AsMap(), &r)
		if mapErr != nil {
			return nil, mapErr
		}
		new := newOfType(custom)
		mapErr = mapper.Decode(req.GetProperties().AsMap(), &new)
		if mapErr != nil {
			return nil, mapErr
		}
		err = r.Read(ctx)
		if err != nil {
			return nil, err
		}
	}

	return nil, status.Error(codes.Unimplemented, "Read is not yet implemented")
}

// Update updates an existing resource with new values.
func (s *Server) Update(ctx context.Context, req *rpc.UpdateRequest) (*rpc.UpdateResponse, error) {
	custom, err := s.Customs.GetCustom(resource.URN(req.Urn).Type())
	if err != nil {
		return nil, err
	}
	if r, ok := custom.(r.ResourceUpdate); ok {
		mapper := mapper.New(&mapper.Opts{
			IgnoreMissing: true,
		})
		err = mapper.Decode(req.GetOlds().AsMap(), &r)
		if err != nil {
			return nil, err
		}
		new := newOfType(custom)
		err = mapper.Decode(req.GetNews().AsMap(), &new)
		if err != nil {
			return nil, err
		}
		err = r.Update(ctx, req.Id, new, req.GetIgnoreChanges(), req.GetPreview())
		if err != nil {
			return nil, err
		}

		props, err := introspect.ResourceToProperties(custom)
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
	custom, err := s.Customs.GetCustom(resource.URN(req.Urn).Type())
	if err != nil {
		return nil, err
	}
	mapper := mapper.New(&mapper.Opts{
		IgnoreMissing: true,
	})
	err = mapper.Decode(req.GetProperties().AsMap(), &custom)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, time.Duration(req.GetTimeout())*time.Second)
	defer cancel()
	err = custom.Delete(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// Construct creates a new instance of the provided component resource and returns its state.
func (s *Server) Construct(ctx context.Context, request *rpc.ConstructRequest) (*rpc.ConstructResponse, error) {
	c, err := s.Components.GetComponent(tokens.Type(request.Type))
	if err != nil {
		return nil, err
	}
	return provider.Construct(ctx, request, s.Host.EngineConn(), componentFn(s.Name, c))
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
func (s *Server) Attach(context.Context, *rpc.PluginAttach) (*emptypb.Empty, error) {
	return nil, status.Error(codes.Unimplemented, "Attach is not yet implemented")
}
