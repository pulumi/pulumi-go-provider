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

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	pprovider "github.com/pulumi/pulumi/pkg/v3/resource/provider"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	presource "github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	comProvider "github.com/pulumi/pulumi/sdk/v3/go/pulumi/provider"
	rpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/structpb"

	dotnetgen "github.com/pulumi/pulumi/pkg/v3/codegen/dotnet"
	gogen "github.com/pulumi/pulumi/pkg/v3/codegen/go"
	nodejsgen "github.com/pulumi/pulumi/pkg/v3/codegen/nodejs"
	pygen "github.com/pulumi/pulumi/pkg/v3/codegen/python"

	"github.com/pulumi/pulumi-go-provider/function"
	"github.com/pulumi/pulumi-go-provider/internal/server"
	"github.com/pulumi/pulumi-go-provider/resource"
	"github.com/pulumi/pulumi-go-provider/types"
)

type GetSchemaRequest struct {
	Version int
}

type GetSchemaResponse struct {
	Schema string
}

type CheckRequest struct {
	Urn  presource.URN
	Olds presource.PropertyMap
	News presource.PropertyMap
}

type CheckFailure struct {
	Property string
	Reason   string
}

type CheckResponse struct {
	Inputs   presource.PropertyMap
	Failures []CheckFailure
}

type DiffRequest struct {
	ID            string
	Urn           presource.URN
	Olds          presource.PropertyMap
	News          presource.PropertyMap
	IgnoreChanges []presource.PropertyKey
}

type PropertyDiff struct {
	Kind      DiffKind // The kind of diff asdsociated with this property.
	InputDiff bool     // The difference is between old and new inputs, not old and new state.
}

type DiffKind string

const (
	Add           DiffKind = "add"            // this property was added
	AddReplace    DiffKind = "add&replace"    // this property was added, and this change requires a replace
	Delete        DiffKind = "delete"         // this property was removed
	DeleteReplace DiffKind = "delete&replace" // this property was removed, and this change requires a replace
	Update        DiffKind = "update"         // this property's value was changed
	UpdateReplace DiffKind = "update&replace" // this property's value was changed, and this change requires a replace
	Stable        DiffKind = "stable"         // this property's value will not change
)

func (k DiffKind) rpc() rpc.PropertyDiff_Kind {
	switch k {
	case Add:
		return rpc.PropertyDiff_ADD
	case AddReplace:
		return rpc.PropertyDiff_ADD_REPLACE
	case Delete:
		return rpc.PropertyDiff_DELETE
	case DeleteReplace:
		return rpc.PropertyDiff_DELETE_REPLACE
	case Update:
		return rpc.PropertyDiff_UPDATE
	case UpdateReplace:
		return rpc.PropertyDiff_UPDATE_REPLACE
	default:
		panic("Unexpected diff kind: " + k)
	}
}

type DiffResponse struct {
	DeleteBeforeReplace bool // if true, this resource must be deleted before replacing it.
	HasChanges          bool // if true, this diff represents an actual difference and thus requires an update.
	// detailedDiff is an optional field that contains map from each changed property to the type of the change.
	//
	// The keys of this map are property paths. These paths are essentially Javascript property access expressions
	// in which all elements are literals, and obey the following EBNF-ish grammar:
	//
	//   propertyName := [a-zA-Z_$] { [a-zA-Z0-9_$] }
	//   quotedPropertyName := '"' ( '\' '"' | [^"] ) { ( '\' '"' | [^"] ) } '"'
	//   arrayIndex := { [0-9] }
	//
	//   propertyIndex := '[' ( quotedPropertyName | arrayIndex ) ']'
	//   rootProperty := ( propertyName | propertyIndex )
	//   propertyAccessor := ( ( '.' propertyName ) |  propertyIndex )
	//   path := rootProperty { propertyAccessor }
	//
	// Examples of valid keys:
	// - root
	// - root.nested
	// - root["nested"]
	// - root.double.nest
	// - root["double"].nest
	// - root["double"]["nest"]
	// - root.array[0]
	// - root.array[100]
	// - root.array[0].nested
	// - root.array[0][1].nested
	// - root.nested.array[0].double[1]
	// - root["key with \"escaped\" quotes"]
	// - root["key with a ."]
	// - ["root key with \"escaped\" quotes"].nested
	// - ["root key with a ."][100]
	DetailedDiff map[string]PropertyDiff
}

func (d DiffResponse) rpc() *rpc.DiffResponse {
	r := rpc.DiffResponse{
		DeleteBeforeReplace: d.DeleteBeforeReplace,
		Changes:             rpc.DiffResponse_DIFF_NONE,
		DetailedDiff:        detailedDiff(d.DetailedDiff).rpc(),
		HasDetailedDiff:     true,
	}
	if d.HasChanges {
		r.Changes = rpc.DiffResponse_DIFF_SOME
	}
	for k, v := range d.DetailedDiff {
		switch v.Kind {
		case Add:
			r.Diffs = append(r.Diffs, k)
		case AddReplace:
			r.Replaces = append(r.Replaces, k)
			r.Diffs = append(r.Diffs, k)
		case Delete:
			r.Diffs = append(r.Diffs, k)
		case DeleteReplace:
			r.Replaces = append(r.Replaces, k)
			r.Diffs = append(r.Diffs, k)
		case Update:
			r.Diffs = append(r.Diffs, k)
		case UpdateReplace:
			r.Replaces = append(r.Replaces, k)
			r.Diffs = append(r.Diffs, k)
		case Stable:
			r.Stables = append(r.Stables, k)
		}
	}
	return &r
}

type ConfigureRequest struct {
	Variables map[string]string
	Args      presource.PropertyMap
	// TODO: these options should be handled by the library, not by the provider
	// AcceptSecrets   bool
	// AcceptResources bool
}

type InvokeRequest struct {
	Token tokens.Type           // the function token to invoke.
	Args  presource.PropertyMap // the arguments for the function invocation.
}

type InvokeResponse struct {
	Return   presource.PropertyMap // the returned values, if invoke was successful.
	Failures []CheckFailure        // the failures if any arguments didn't pass verification.
}

type CreateRequest struct {
	Urn        presource.URN         // the Pulumi URN for this resource.
	Properties presource.PropertyMap // the provider inputs to set during creation.
	Timeout    float64               // the create request timeout represented in seconds.
	Preview    bool                  // true if this is a preview and the provider should not actually create the resource.
}

type CreateResponse struct {
	ID         string                // the ID of the created resource.
	Properties presource.PropertyMap // any properties that were computed during creation.
}

type ReadRequest struct {
	ID         string                // the ID of the resource to read.
	Urn        presource.URN         // the Pulumi URN for this resource.
	Properties presource.PropertyMap // the current state (sufficiently complete to identify the resource).
	Inputs     presource.PropertyMap // the current inputs, if any (only populated during refresh).
}

type ReadResponse struct {
	ID         string                // the ID of the resource read back (or empty if missing).
	Properties presource.PropertyMap // the state of the resource read from the live environment.
	Inputs     presource.PropertyMap // the inputs for this resource that would be returned from Check.
}

type UpdateRequest struct {
	ID            string                  // the ID of the resource to update.
	Urn           presource.URN           // the Pulumi URN for this resource.
	Olds          presource.PropertyMap   // the old values of provider inputs for the resource to update.
	News          presource.PropertyMap   // the new values of provider inputs for the resource to update.
	Timeout       float64                 // the update request timeout represented in seconds.
	IgnoreChanges []presource.PropertyKey // a set of property paths that should be treated as unchanged.
	Preview       bool                    // true if the provider should not actually create the resource.
}

type UpdateResponse struct {
	Properties presource.PropertyMap // any properties that were computed during updating.
}

type DeleteRequest struct {
	ID         string                // the ID of the resource to delete.
	Urn        presource.URN         // the Pulumi URN for this resource.
	Properties presource.PropertyMap // the current properties on the resource.
	Timeout    float64               // the delete request timeout represented in seconds.
}

type Provider interface {
	// Utility

	// GetSchema fetches the schema for this resource provider.
	GetSchema(Context, GetSchemaRequest) (GetSchemaResponse, error)
	// Cancel signals the provider to gracefully shut down and abort any ongoing resource operations.
	// Operations aborted in this way will return an error (e.g., `Update` and `Create` will either return a
	// creation error or an initialization error). Since Cancel is advisory and non-blocking, it is up
	// to the host to decide how long to wait after Cancel is called before (e.g.)
	// hard-closing any gRPC connection.
	Cancel(Context) error

	// Provider Config
	CheckConfig(Context, CheckRequest) (CheckResponse, error)
	DiffConfig(Context, DiffRequest) (DiffResponse, error)
	// NOTE: We opt into all options.
	Configure(Context, ConfigureRequest) error

	// Invokes
	Invoke(Context, InvokeRequest) (InvokeResponse, error)
	// TODO Stream invoke (are those used anywhere)

	// Custom Resources

	// Check validates that the given property bag is valid for a resource of the given type and returns the inputs
	// that should be passed to successive calls to Diff, Create, or Update for this resource. As a rule, the provider
	// inputs returned by a call to Check should preserve the original representation of the properties as present in
	// the program inputs. Though this rule is not required for correctness, violations thereof can negatively impact
	// the end-user experience, as the provider inputs are using for detecting and rendering diffs.
	Check(Context, CheckRequest) (CheckResponse, error)
	Diff(Context, DiffRequest) (DiffResponse, error)
	Create(Context, CreateRequest) (CreateResponse, error)
	Read(Context, ReadRequest) (ReadResponse, error)
	Update(Context, UpdateRequest) (UpdateResponse, error)
	Delete(Context, DeleteRequest) error
	// TODO Call

	// Components Resources
	Construct(pctx Context, typ string, name string,
		ctx *pulumi.Context, inputs comProvider.ConstructInputs, opts pulumi.ResourceOption) (pulumi.ComponentResource, error)
}

func RunProvider(name string, version semver.Version, provider Provider) error {
	return pprovider.Main(name, newProvider(name, version.String(), provider))
}

func newProvider(name, version string, p Provider) func(*pprovider.HostClient) (rpc.ResourceProviderServer, error) {
	return func(host *pprovider.HostClient) (rpc.ResourceProviderServer, error) {
		return &provider{
			name:    name,
			version: version,
			host:    host,
			client:  p,
		}, nil
	}
}

type provider struct {
	name    string
	version string
	host    *pprovider.HostClient
	client  Provider
}

type Context interface {
	context.Context
	// Log logs a global message, including errors and warnings.
	Log(severity diag.Severity, msg string)
	// Logf logs a global message, including errors and warnings.
	Logf(severity diag.Severity, msg string, args ...any)
	// LogStatus logs a global status message, including errors and warnings. Status messages will
	// appear in the `Info` column of the progress display, but not in the final output.
	LogStatus(severity diag.Severity, msg string)
	// LogStatusf logs a global status message, including errors and warnings. Status messages will
	// appear in the `Info` column of the progress display, but not in the final output.
	LogStatusf(severity diag.Severity, msg string, args ...any)
	RuntimeInformation() RunInfo
}

type RunInfo struct {
	PackageName string
	Version     string
}

type pkgContext struct {
	context.Context
	provider *provider
	urn      presource.URN
}

func (p *pkgContext) Log(severity diag.Severity, msg string) {
	err := p.provider.host.Log(p, severity, p.urn, msg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to log %s: %s", severity, msg)
	}
}

func (p *pkgContext) Logf(severity diag.Severity, msg string, args ...any) {
	p.Log(severity, fmt.Sprintf(msg, args...))
}
func (p *pkgContext) LogStatus(severity diag.Severity, msg string) {
	err := p.provider.host.LogStatus(p, severity, p.urn, msg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to log %s status: %s", severity, msg)
	}
}
func (p *pkgContext) LogStatusf(severity diag.Severity, msg string, args ...any) {
	p.LogStatus(severity, fmt.Sprintf(msg, args...))
}

func (p *pkgContext) RuntimeInformation() RunInfo {
	return RunInfo{
		PackageName: p.provider.name,
		Version:     p.provider.version,
	}
}

func (p *provider) ctx(ctx context.Context, urn presource.URN) Context {
	return &pkgContext{ctx, p, urn}
}

func (p *provider) getMap(s *structpb.Struct) (presource.PropertyMap, error) {
	return plugin.UnmarshalProperties(s, plugin.MarshalOptions{
		KeepUnknowns:  true,
		SkipNulls:     true,
		KeepResources: true,
		KeepSecrets:   true,
	})
}

func (p *provider) asStruct(m presource.PropertyMap) (*structpb.Struct, error) {
	return plugin.MarshalProperties(m, plugin.MarshalOptions{
		KeepUnknowns: true,
		SkipNulls:    true,
		KeepSecrets:  true,
	})
}

func (p *provider) GetSchema(ctx context.Context, req *rpc.GetSchemaRequest) (*rpc.GetSchemaResponse, error) {
	r, err := p.client.GetSchema(p.ctx(ctx, ""), GetSchemaRequest{
		Version: int(req.GetVersion()),
	})
	if err != nil {
		return nil, err
	}
	return &rpc.GetSchemaResponse{
		Schema: r.Schema,
	}, nil
}

type checkFailureList []CheckFailure

func (l checkFailureList) rpc() []*rpc.CheckFailure {
	failures := make([]*rpc.CheckFailure, len(l))
	for i, f := range l {
		failures[i] = &rpc.CheckFailure{
			Property: f.Property,
			Reason:   f.Reason,
		}
	}
	return failures
}

type detailedDiff map[string]PropertyDiff

func (d detailedDiff) rpc() map[string]*rpc.PropertyDiff {
	detailedDiff := map[string]*rpc.PropertyDiff{}
	for k, v := range d {
		if v.Kind == Stable {
			continue
		}
		detailedDiff[k] = &rpc.PropertyDiff{
			Kind:      v.Kind.rpc(),
			InputDiff: v.InputDiff,
		}
	}
	return detailedDiff
}

func (p *provider) CheckConfig(ctx context.Context, req *rpc.CheckRequest) (*rpc.CheckResponse, error) {
	olds, err := p.getMap(req.Olds)
	if err != nil {
		return nil, err
	}

	news, err := p.getMap(req.News)
	if err != nil {
		return nil, err
	}
	r, err := p.client.CheckConfig(p.ctx(ctx, presource.URN(req.GetUrn())), CheckRequest{
		Urn:  presource.URN(req.GetUrn()),
		Olds: olds,
		News: news,
	})

	if err != nil {
		return nil, err
	}

	inputs, err := p.asStruct(r.Inputs)
	if err != nil {
		return nil, err
	}

	return &rpc.CheckResponse{
		Inputs:   inputs,
		Failures: checkFailureList(r.Failures).rpc(),
	}, err
}

func getIgnoreChanges(l []string) []presource.PropertyKey {
	r := make([]presource.PropertyKey, len(l))
	for i, p := range l {
		r[i] = presource.PropertyKey(p)
	}
	return r
}

func (p *provider) DiffConfig(ctx context.Context, req *rpc.DiffRequest) (*rpc.DiffResponse, error) {
	olds, err := p.getMap(req.GetOlds())
	if err != nil {
		return nil, err
	}
	news, err := p.getMap(req.GetNews())
	if err != nil {
		return nil, err
	}
	r, err := p.client.DiffConfig(p.ctx(ctx, presource.URN(req.GetUrn())), DiffRequest{
		ID:            req.GetId(),
		Urn:           presource.URN(req.GetUrn()),
		Olds:          olds,
		News:          news,
		IgnoreChanges: getIgnoreChanges(req.GetIgnoreChanges()),
	})
	if err != nil {
		return nil, err
	}
	return r.rpc(), nil
}

func (p *provider) Configure(ctx context.Context, req *rpc.ConfigureRequest) (*rpc.ConfigureResponse, error) {
	argMap, err := p.getMap(req.GetArgs())
	if err != nil {
		return nil, err
	}
	err = p.client.Configure(p.ctx(ctx, ""), ConfigureRequest{
		Variables: req.GetVariables(),
		Args:      argMap,
	})
	if err != nil {
		return nil, err
	}
	return &rpc.ConfigureResponse{
		AcceptSecrets:   true,
		SupportsPreview: true,
		AcceptResources: true,
		AcceptOutputs:   true,
	}, nil
}

func (p *provider) Invoke(ctx context.Context, req *rpc.InvokeRequest) (*rpc.InvokeResponse, error) {
	argMap, err := p.getMap(req.GetArgs())
	if err != nil {
		return nil, err
	}
	r, err := p.client.Invoke(p.ctx(ctx, ""), InvokeRequest{
		Token: tokens.Type(req.GetTok()),
		Args:  argMap,
	})
	if err != nil {
		return nil, err
	}
	retStruct, err := p.asStruct(r.Return)
	if err != nil {
		return nil, err
	}
	return &rpc.InvokeResponse{
		Return:   retStruct,
		Failures: checkFailureList(r.Failures).rpc(),
	}, nil
}

func (p *provider) StreamInvoke(*rpc.InvokeRequest, rpc.ResourceProvider_StreamInvokeServer) error {
	return status.Error(codes.Unimplemented, "StreamInvoke is not yet implemented")
}

func (p *provider) Call(context.Context, *rpc.CallRequest) (*rpc.CallResponse, error) {
	return nil, status.Error(codes.Unimplemented, "Call is not yet implemented")
}

func (p *provider) Check(ctx context.Context, req *rpc.CheckRequest) (*rpc.CheckResponse, error) {
	olds, err := p.getMap(req.GetOlds())
	if err != nil {
		return nil, err
	}
	news, err := p.getMap(req.GetNews())
	if err != nil {
		return nil, err
	}

	r, err := p.client.Check(p.ctx(ctx, presource.URN(req.GetUrn())), CheckRequest{
		Urn:  presource.URN(req.GetUrn()),
		Olds: olds,
		News: news,
	})
	if err != nil {
		return nil, err
	}

	inputs, err := p.asStruct(r.Inputs)
	if err != nil {
		return nil, err
	}
	return &rpc.CheckResponse{
		Inputs:   inputs,
		Failures: checkFailureList(r.Failures).rpc(),
	}, nil

}

func (p *provider) Diff(ctx context.Context, req *rpc.DiffRequest) (*rpc.DiffResponse, error) {
	olds, err := p.getMap(req.GetOlds())
	if err != nil {
		return nil, err
	}
	news, err := p.getMap(req.GetNews())
	if err != nil {
		return nil, err
	}
	r, err := p.client.Diff(p.ctx(ctx, presource.URN(req.GetUrn())), DiffRequest{
		ID:            req.GetId(),
		Urn:           presource.URN(req.GetUrn()),
		Olds:          olds,
		News:          news,
		IgnoreChanges: getIgnoreChanges(req.GetIgnoreChanges()),
	})
	if err != nil {
		return nil, err
	}

	return r.rpc(), nil
}

func (p *provider) Create(ctx context.Context, req *rpc.CreateRequest) (*rpc.CreateResponse, error) {
	props, err := p.getMap(req.GetProperties())
	if err != nil {
		return nil, err
	}
	r, err := p.client.Create(p.ctx(ctx, presource.URN(req.GetUrn())), CreateRequest{
		Urn:        presource.URN(req.GetUrn()),
		Properties: props,
		Timeout:    req.GetTimeout(),
		Preview:    req.GetPreview(),
	})
	if err != nil {
		return nil, err
	}

	propStruct, err := p.asStruct(r.Properties)
	if err != nil {
		return nil, err
	}

	return &rpc.CreateResponse{
		Id:         r.ID,
		Properties: propStruct,
	}, nil
}

func (p *provider) Read(ctx context.Context, req *rpc.ReadRequest) (*rpc.ReadResponse, error) {
	propMap, err := p.getMap(req.GetProperties())
	if err != nil {
		return nil, err
	}
	inputMap, err := p.getMap(req.GetInputs())
	if err != nil {
		return nil, err
	}
	r, err := p.client.Read(p.ctx(ctx, presource.URN(req.GetUrn())), ReadRequest{
		ID:         req.GetId(),
		Urn:        presource.URN(req.GetUrn()),
		Properties: propMap,
		Inputs:     inputMap,
	})
	if err != nil {
		return nil, err
	}
	inputStruct, err := p.asStruct(r.Inputs)
	if err != nil {
		return nil, err
	}
	propStruct, err := p.asStruct(r.Properties)
	if err != nil {
		return nil, err
	}
	return &rpc.ReadResponse{
		Id:         r.ID,
		Properties: propStruct,
		Inputs:     inputStruct,
	}, nil
}

func (p *provider) Update(ctx context.Context, req *rpc.UpdateRequest) (*rpc.UpdateResponse, error) {
	oldsMap, err := p.getMap(req.GetOlds())
	if err != nil {
		return nil, err
	}
	newsMap, err := p.getMap(req.GetNews())
	if err != nil {
		return nil, err
	}
	r, err := p.client.Update(p.ctx(ctx, presource.URN(req.GetUrn())), UpdateRequest{
		ID:            req.GetId(),
		Urn:           presource.URN(req.GetUrn()),
		Olds:          oldsMap,
		News:          newsMap,
		Timeout:       req.GetTimeout(),
		IgnoreChanges: getIgnoreChanges(req.GetIgnoreChanges()),
		Preview:       req.GetPreview(),
	})
	if err != nil {
		return nil, err
	}
	props, err := p.asStruct(r.Properties)
	if err != nil {
		return nil, err
	}
	return &rpc.UpdateResponse{
		Properties: props,
	}, nil

}

func (p *provider) Delete(ctx context.Context, req *rpc.DeleteRequest) (*emptypb.Empty, error) {
	props, err := p.getMap(req.GetProperties())
	if err != nil {
		return nil, err
	}
	err = p.client.Delete(p.ctx(ctx, presource.URN(req.GetUrn())), DeleteRequest{
		ID:         req.GetId(),
		Urn:        presource.URN(req.GetUrn()),
		Properties: props,
		Timeout:    req.GetTimeout(),
	})
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil

}

func (p *provider) Construct(pctx context.Context, req *rpc.ConstructRequest) (*rpc.ConstructResponse, error) {
	return comProvider.Construct(pctx, req, p.host.EngineConn(), func(ctx *pulumi.Context, typ, name string,
		inputs comProvider.ConstructInputs, opts pulumi.ResourceOption) (*comProvider.ConstructResult, error) {
		r, err := p.client.Construct(p.ctx(pctx, ""), typ, name, ctx, inputs, opts)
		if err != nil {
			return nil, err
		}
		return comProvider.NewConstructResult(r)
	})
}

func (p *provider) Cancel(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	err := p.client.Cancel(p.ctx(ctx, ""))
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil

}

func (p *provider) GetPluginInfo(context.Context, *emptypb.Empty) (*rpc.PluginInfo, error) {
	return &rpc.PluginInfo{
		Version: p.version,
	}, nil
}

func (p *provider) Attach(ctx context.Context, req *rpc.PluginAttach) (*emptypb.Empty, error) {
	host, err := pprovider.NewHostClient(req.GetAddress())
	if err != nil {
		return nil, err
	}
	p.host = host
	return &emptypb.Empty{}, nil
}

// Run spawns a Pulumi Provider server, returning when the server shuts down. This
// function should be called directly from main and the program should return after Run
// returns.
func Run(name string, version semver.Version, providerOptions ...Options) error {
	opts := options{
		Name:     name,
		Version:  version,
		Language: map[string]schema.RawMessage{},
	}
	for _, o := range providerOptions {
		o(&opts)
	}
	makeProviderfunc, schemastr, err := prepareProvider(opts)
	if err != nil {
		return err
	}

	sdkGenIndex := -1
	emitSchemaIndex := -1
	for i, arg := range os.Args {
		if arg == "-sdkGen" {
			sdkGenIndex = i
		}
		if arg == "-emitSchema" {
			emitSchemaIndex = i
		}
	}
	runProvider := sdkGenIndex == -1 && emitSchemaIndex == -1

	getArgs := func(index int) []string {
		args := []string{}
		for _, arg := range os.Args[index+1:] {
			if strings.HasPrefix(arg, "-") {
				break
			}
			args = append(args, arg)
		}
		return args
	}

	if emitSchemaIndex != -1 {
		args := getArgs(emitSchemaIndex)
		if len(args) > 1 {
			return fmt.Errorf("-emitSchema only takes one optional argument, it received %d", len(args))
		}
		file := "schema.json"
		if len(args) > 0 {
			file = args[0]
		}

		if err := ioutil.WriteFile(file, []byte(schemastr), 0600); err != nil {
			return fmt.Errorf("failed to write schema: %w", err)
		}
	}

	if sdkGenIndex != -1 {
		args := getArgs(sdkGenIndex)
		rootDir := "."
		if len(args) != 0 {
			rootDir = args[0]
			args = args[1:]
		}
		sdkPath := filepath.Join(rootDir, "sdk")
		fmt.Printf("Generating sdk for %s in %s\n", args, sdkPath)
		var spec schema.PackageSpec
		err = json.Unmarshal([]byte(schemastr), &spec)
		if err != nil {
			return err
		}
		pkg, diags, err := schema.BindSpec(spec, nil)
		if err != nil {
			return err
		}
		if len(diags) > 0 {
			return diags
		}
		if err := generateSDKs(name, sdkPath, pkg, args...); err != nil {
			return fmt.Errorf("failed to generate schema: %w", err)
		}
	}

	if runProvider {
		return pprovider.Main(name, makeProviderfunc)
	}
	return nil
}

func generateSDKs(pkgName, outDir string, pkg *schema.Package, languages ...string) error {
	if outDir == "" {
		return fmt.Errorf("outDir not specified")
	}
	if err := os.RemoveAll(outDir); err != nil && !os.IsNotExist(err) {
		return err
	}
	if len(languages) == 0 {
		languages = []string{"go", "python", "nodejs", "dotnet"}
	}
	for _, lang := range languages {
		var files map[string][]byte
		var err error
		switch lang {
		case "go":
			files, err = gogen.GeneratePackage(pkgName, pkg)
		case "python":
			files, err = pygen.GeneratePackage(pkgName, pkg, files)
		case "nodejs":
			files, err = nodejsgen.GeneratePackage(pkgName, pkg, files)
		case "dotnet":
			files, err = dotnetgen.GeneratePackage(pkgName, pkg, files)
		default:
			fmt.Printf("Unknown language: '%s'", lang)
			continue
		}
		if err != nil {
			return err
		}
		root := filepath.Join(outDir, lang)
		for p, file := range files {
			// TODO: full conversion from path to filepath
			err = os.MkdirAll(filepath.Join(root, path.Dir(p)), 0700)
			if err != nil {
				return err
			}
			if err = os.WriteFile(filepath.Join(root, p), file, 0600); err != nil {
				return err
			}
		}
	}
	return nil
}

func prepareProvider(opts options) (func(*pprovider.HostClient) (rpc.ResourceProviderServer,
	error), string, error) {

	pkg := tokens.NewPackageToken(tokens.PackageName(opts.Name))
	components, err := server.NewComponentResources(pkg, opts.Components)
	if err != nil {
		return nil, "", err
	}
	customs, err := server.NewCustomResources(pkg, opts.Customs)
	if err != nil {
		return nil, "", err
	}
	invokes, err := server.NewInvokes(pkg, opts.Functions)
	if err != nil {
		return nil, "", err
	}
	schema, err := serialize(opts)
	if err != nil {
		return nil, "", err
	}

	return func(host *pprovider.HostClient) (rpc.ResourceProviderServer, error) {
		return server.New(pkg.String(), opts.Version, host, components,
			customs, invokes, schema), nil
	}, schema, nil
}

type options struct {
	Name        string
	Version     semver.Version
	Customs     []resource.Custom
	Types       []interface{}
	Components  []resource.Component
	PartialSpec schema.PackageSpec
	Functions   []function.Function

	Language map[string]schema.RawMessage
}

type Options func(*options)

// Resources adds resource.Custom for the provider to serve.
func Resources(resources ...resource.Custom) Options {
	return func(o *options) {
		o.Customs = append(o.Customs, resources...)
	}
}

// Types adds schema types for the provider to serve.
func Types(types ...interface{}) Options {
	return func(o *options) {
		o.Types = append(o.Types, types...)
	}
}

// Components adds resource.Components for the provider to serve.
func Components(components ...resource.Component) Options {
	return func(o *options) {
		o.Components = append(o.Components, components...)
	}
}

func Functions(functions ...function.Function) Options {
	return func(o *options) {
		o.Functions = append(o.Functions, functions...)
	}
}

func PartialSpec(spec schema.PackageSpec) Options {
	return func(o *options) {
		o.PartialSpec = spec
	}
}

func Enum[T any](values ...types.EnumValue) types.Enum {
	v := new(T)
	t := reflect.TypeOf(v).Elem()
	return types.Enum{
		Type:   t,
		Values: values,
	}
}

func EnumVal(name string, value any) types.EnumValue {
	return types.EnumValue{
		Name:  name,
		Value: value,
	}
}
func GoOptions(opts gogen.GoPackageInfo) Options {
	return func(o *options) {
		b, err := json.Marshal(opts)
		contract.AssertNoErrorf(err, "Failed to marshal go package info")
		o.Language["go"] = b
	}
}
