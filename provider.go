// Copyright 2022-2024, Pulumi Corporation.
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

// Package provider works as a shared high-level interface for [rpc.ResourceProviderServer].
//
// It is the lowest level that the rest of this repo should target, and servers as an
// interoperability layer between middle-wares.
package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/blang/semver"
	"github.com/hashicorp/go-multierror"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	pprovider "github.com/pulumi/pulumi/pkg/v3/resource/provider"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	presource "github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	pconfig "github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil/rpcerror"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	comProvider "github.com/pulumi/pulumi/sdk/v3/go/pulumi/provider"
	rpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/pulumi/pulumi-go-provider/internal"
	"github.com/pulumi/pulumi-go-provider/internal/key"
	"github.com/pulumi/pulumi-go-provider/resourcex"
)

type GetSchemaRequest struct {
	Version int
}

type GetSchemaResponse struct {
	Schema string
}

type CheckRequest struct {
	Urn        presource.URN
	Olds       presource.PropertyMap
	News       presource.PropertyMap
	RandomSeed []byte
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

type diffChanges bool

func (c diffChanges) rpc() rpc.DiffResponse_DiffChanges {
	if c {
		return rpc.DiffResponse_DIFF_SOME
	}
	return rpc.DiffResponse_DIFF_NONE
}

func (d DiffResponse) rpc() *rpc.DiffResponse {

	hasDetailedDiff := true
	if _, ok := d.DetailedDiff[key.ForceNoDetailedDiff]; ok {
		hasDetailedDiff = false
		delete(d.DetailedDiff, key.ForceNoDetailedDiff)
	}

	r := rpc.DiffResponse{
		DeleteBeforeReplace: d.DeleteBeforeReplace,
		Changes:             diffChanges(d.HasChanges).rpc(),
		DetailedDiff:        detailedDiff(d.DetailedDiff).rpc(),
		HasDetailedDiff:     hasDetailedDiff,
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

	// non-nil to indicate that the create failed and left the resource in a partial
	// state.
	//
	// If PartialState is non-nil, then an error will be returned, annotated with
	// [pulumirpc.ErrorResourceInitFailed].
	PartialState *InitializationFailed
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

	// non-nil to indicate that the read failed and left the resource in a partial
	// state.
	//
	// If PartialState is non-nil, then an error will be returned, annotated with
	// [pulumirpc.ErrorResourceInitFailed].
	PartialState *InitializationFailed
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
	// any properties that were computed during updating.
	Properties presource.PropertyMap
	// non-nil to indicate that the update failed and left the resource in a partial
	// state.
	//
	// If PartialState is non-nil, then an error will be returned, annotated with
	// [pulumirpc.ErrorResourceInitFailed].
	PartialState *InitializationFailed
}

type DeleteRequest struct {
	ID         string                // the ID of the resource to delete.
	Urn        presource.URN         // the Pulumi URN for this resource.
	Properties presource.PropertyMap // the current properties on the resource.
	Timeout    float64               // the delete request timeout represented in seconds.
}

// InitializationFailed indicates that a resource exists but failed to initialize, and is
// thus in a partial state.
type InitializationFailed struct {
	// Reasons why the resource did not fully initialize.
	Reasons []string
}

// ConfigMissingKeys creates a structured error for missing provider keys.
func ConfigMissingKeys(missing map[string]string) error {
	if len(missing) == 0 {
		return nil
	}
	rpcMissing := make([]*rpc.ConfigureErrorMissingKeys_MissingKey, 0, len(missing))
	for k, v := range missing {
		rpcMissing = append(rpcMissing, &rpc.ConfigureErrorMissingKeys_MissingKey{
			Name:        k,
			Description: v,
		})
	}
	return rpcerror.WithDetails(
		rpcerror.New(codes.InvalidArgument, "required configuration keys were missing"),
		&rpc.ConfigureErrorMissingKeys{
			MissingKeys: rpcMissing,
		},
	)
}

type Provider struct {
	// Utility

	// GetSchema fetches the schema for this resource provider.
	GetSchema func(context.Context, GetSchemaRequest) (GetSchemaResponse, error)

	// Parameterize sets up the provider as a replacement parameterized provider.
	//
	// If a SDK was generated with parameters, then Parameterize should be called once before
	// [Provider.CheckConfig], [Provider.DiffConfig] or [Provider.Configure].
	//
	// Parameterize can be called in 2 configurations: with [ParameterizeRequest.Args] specified or with
	// [ParameterizeRequest.Value] specified. Parameterize should leave the provider in the same state
	// regardless of which variant was used.
	//
	// For more through documentation on Parameterize, see
	// https://pulumi-developer-docs.readthedocs.io/latest/docs/architecture/providers.html#parameterized-providers.
	Parameterize func(context.Context, ParameterizeRequest) (ParameterizeResponse, error)

	// Cancel signals the provider to gracefully shut down and abort any ongoing resource operations.
	// Operations aborted in this way will return an error (e.g., `Update` and `Create` will either return a
	// creation error or an initialization error). Since Cancel is advisory and non-blocking, it is up
	// to the host to decide how long to wait after Cancel is called before (e.g.)
	// hard-closing any gRPC connection.
	Cancel func(context.Context) error

	// Provider Config
	CheckConfig func(context.Context, CheckRequest) (CheckResponse, error)
	DiffConfig  func(context.Context, DiffRequest) (DiffResponse, error)
	// NOTE: We opt into all options.
	Configure func(context.Context, ConfigureRequest) error

	// Invokes
	Invoke func(context.Context, InvokeRequest) (InvokeResponse, error)
	// TODO Stream invoke (are those used anywhere)

	// Custom Resources

	// Check validates that the given property bag is valid for a resource of the given type and returns the inputs
	// that should be passed to successive calls to Diff, Create, or Update for this resource. As a rule, the provider
	// inputs returned by a call to Check should preserve the original representation of the properties as present in
	// the program inputs. Though this rule is not required for correctness, violations thereof can negatively impact
	// the end-user experience, as the provider inputs are using for detecting and rendering diffs.
	Check  func(context.Context, CheckRequest) (CheckResponse, error)
	Diff   func(context.Context, DiffRequest) (DiffResponse, error)
	Create func(context.Context, CreateRequest) (CreateResponse, error)
	Read   func(context.Context, ReadRequest) (ReadResponse, error)
	Update func(context.Context, UpdateRequest) (UpdateResponse, error)
	Delete func(context.Context, DeleteRequest) error

	// Call allows methods to be attached to resources.
	//
	// Right now, Call is restricted to methods on component resources.[^1][^2]
	//
	// [^1]: On the provider resource: https://github.com/pulumi/pulumi/issues/17025
	// [^2]: On custom resources: https://github.com/pulumi/pulumi/issues/16257
	Call func(context.Context, CallRequest) (CallResponse, error)

	// Components Resources
	Construct func(context.Context, ConstructRequest) (ConstructResponse, error)
}

// WithDefaults returns a provider with sensible defaults. It does not mutate its
// receiver.
//
// Most default values return a NotYetImplemented error, which the engine knows to ignore.
// Other defaults are no-op functions.
//
// You should not need to call this function manually. It will be automatically invoked
// before a provider is run.
func (d Provider) WithDefaults() Provider {
	nyi := func(fn string) error {
		return status.Errorf(codes.Unimplemented, "%s is not implemented", fn)
	}
	if d.GetSchema == nil {
		d.GetSchema = func(context.Context, GetSchemaRequest) (GetSchemaResponse, error) {
			return GetSchemaResponse{}, nyi("GetSchema")
		}
	}
	if d.Cancel == nil {
		d.Cancel = func(context.Context) error {
			return nyi("Cancel")
		}
	}

	if d.Parameterize == nil {
		d.Parameterize = func(context.Context, ParameterizeRequest) (ParameterizeResponse, error) {
			return ParameterizeResponse{}, nyi("Parameterize")
		}
	}

	if d.CheckConfig == nil {
		d.CheckConfig = func(context.Context, CheckRequest) (CheckResponse, error) {
			return CheckResponse{}, nyi("CheckConfig")
		}
	}
	if d.DiffConfig == nil {
		d.DiffConfig = func(context.Context, DiffRequest) (DiffResponse, error) {
			return DiffResponse{}, nyi("DiffConfig")
		}
	}
	if d.Configure == nil {
		d.Configure = func(context.Context, ConfigureRequest) error {
			return nil
		}
	}
	if d.Invoke == nil {
		d.Invoke = func(context.Context, InvokeRequest) (InvokeResponse, error) {
			return InvokeResponse{}, nyi("Invoke")
		}
	}
	if d.Check == nil {
		d.Check = func(context.Context, CheckRequest) (CheckResponse, error) {
			return CheckResponse{}, nyi("Check")
		}
	}
	if d.Diff == nil {
		d.Diff = func(context.Context, DiffRequest) (DiffResponse, error) {
			return DiffResponse{}, nyi("Diff")
		}
	}
	if d.Create == nil {
		d.Create = func(context.Context, CreateRequest) (CreateResponse, error) {
			return CreateResponse{}, nyi("Create")
		}
	}
	if d.Read == nil {
		d.Read = func(context.Context, ReadRequest) (ReadResponse, error) {
			return ReadResponse{}, nyi("Read")
		}
	}
	if d.Update == nil {
		d.Update = func(context.Context, UpdateRequest) (UpdateResponse, error) {
			return UpdateResponse{}, nyi("Update")
		}
	}
	if d.Delete == nil {
		d.Delete = func(context.Context, DeleteRequest) error {
			return nyi("Delete")
		}
	}
	if d.Call == nil {
		d.Call = func(context.Context, CallRequest) (CallResponse, error) {
			return CallResponse{}, nyi("Call")
		}
	}
	if d.Construct == nil {
		d.Construct = func(context.Context, ConstructRequest) (ConstructResponse, error) {
			return ConstructResponse{}, nyi("Construct")
		}
	}
	return d
}

// RunProvider runs a provider with the given name and version.
func RunProvider(name, version string, provider Provider) error {
	return pprovider.Main(name, newProvider(name, version, provider.WithDefaults()))
}

// RawServer converts the Provider into a factory for gRPC servers.
//
// If you are trying to set up a standard main function, see [RunProvider].
func RawServer(
	name, version string,
	provider Provider,
) func(*pprovider.HostClient) (rpc.ResourceProviderServer, error) {
	return newProvider(name, version, provider.WithDefaults())
}

// A context which prints its diagnostics, collecting all errors.
type errCollectingContext struct {
	context.Context
	errs   multierror.Error
	info   RunInfo
	stderr io.Writer
}

func (e *errCollectingContext) Log(severity diag.Severity, msg string) {
	if severity == diag.Error {
		e.errs.Errors = append(e.errs.Errors, errors.New(msg))
	}
	_, err := fmt.Fprintf(e.stderr, "Log(%s): %s\n", severity, msg)
	contract.IgnoreError(err)
}

func (e *errCollectingContext) Logf(severity diag.Severity, msg string, args ...any) {
	e.Log(severity, fmt.Sprintf(msg, args...))
}

func (e *errCollectingContext) LogStatus(severity diag.Severity, msg string) {
	if severity == diag.Error {
		e.errs.Errors = append(e.errs.Errors, errors.New(msg))
	}
	_, err := fmt.Fprintf(e.stderr, "LogStatus(%s): %s\n", severity, msg)
	contract.IgnoreError(err)
}

func (e *errCollectingContext) LogStatusf(severity diag.Severity, msg string, args ...any) {
	e.LogStatus(severity, fmt.Sprintf(msg, args...))
}

func (e *errCollectingContext) RuntimeInformation() RunInfo {
	return e.info
}

// GetSchema retrieves the schema from the provider by invoking GetSchema on the provider.
//
// This is a helper method to retrieve the schema from a provider without running the
// provider in a separate process. It should not be necessary for most providers.
//
// To retrieve the schema from a provider binary, use
//
//	pulumi package get-schema ./pulumi-resource-MYPROVIDER
func GetSchema(ctx context.Context, name, version string, provider Provider) (schema.PackageSpec, error) {
	collectingDiag := errCollectingContext{Context: ctx, stderr: os.Stderr, info: RunInfo{
		PackageName: name,
		Version:     version,
	}}
	s, err := provider.GetSchema(&collectingDiag, GetSchemaRequest{Version: 0})
	var errs multierror.Error
	if err != nil {
		errs.Errors = append(errs.Errors, err)
	}
	errs.Errors = append(errs.Errors, collectingDiag.errs.Errors...)

	spec := schema.PackageSpec{}
	if err := errs.ErrorOrNil(); err != nil {
		return spec, err
	}
	err = json.Unmarshal([]byte(s.Schema), &spec)
	return spec, err
}

type ProviderHost interface {
	Construct(context.Context, ConstructRequest, comProvider.ConstructFunc) (ConstructResponse, error)
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
	rpc.UnimplementedResourceProviderServer

	name    string
	version string
	host    *pprovider.HostClient
	client  Provider
}

var _ rpc.ResourceProviderServer = (*provider)(nil)

type providerHost struct {
	p    *provider
	host *pprovider.HostClient
}

var _ ProviderHost = (*providerHost)(nil)

type RunInfo struct {
	PackageName string
	Version     string
}

func GetRunInfo(ctx context.Context) RunInfo { return ctx.Value(key.RuntimeInfo).(RunInfo) }

func (p *provider) ctx(ctx context.Context, urn presource.URN) context.Context {
	if p.host != nil {
		ctx = context.WithValue(ctx, key.Logger, &hostSink{
			host: p.host,
		})
		ctx = context.WithValue(ctx, key.ProviderHost, &providerHost{p, p.host})
	}
	ctx = context.WithValue(ctx, key.URN, urn)
	return context.WithValue(ctx, key.RuntimeInfo, RunInfo{
		PackageName: p.name,
		Version:     p.version,
	})
}

func (p *provider) getMap(s *structpb.Struct) (presource.PropertyMap, error) {
	return plugin.UnmarshalProperties(s, plugin.MarshalOptions{
		KeepUnknowns:     true,
		SkipNulls:        true,
		KeepResources:    true,
		KeepSecrets:      true,
		KeepOutputValues: true,
	})
}

func (p *provider) asStruct(m presource.PropertyMap) (*structpb.Struct, error) {
	return plugin.MarshalProperties(m, plugin.MarshalOptions{
		KeepUnknowns:     true,
		SkipNulls:        true,
		KeepSecrets:      true,
		KeepOutputValues: true,
		KeepResources:    true,
	})
}

func (p *provider) GetSchema(ctx context.Context, req *rpc.GetSchemaRequest) (*rpc.GetSchemaResponse, error) {
	ctx = p.ctx(ctx, "")
	r, err := p.client.GetSchema(ctx, GetSchemaRequest{
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
	ctx = p.ctx(ctx, presource.URN(req.GetUrn()))
	olds, err := p.getMap(req.Olds)
	if err != nil {
		return nil, err
	}

	news, err := p.getMap(req.News)
	if err != nil {
		return nil, err
	}
	r, err := p.client.CheckConfig(ctx, CheckRequest{
		Urn:        presource.URN(req.GetUrn()),
		Olds:       olds,
		News:       news,
		RandomSeed: req.RandomSeed,
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
	ctx = p.ctx(ctx, presource.URN(req.GetUrn()))
	olds, err := p.getMap(req.GetOlds())
	if err != nil {
		return nil, err
	}
	news, err := p.getMap(req.GetNews())
	if err != nil {
		return nil, err
	}
	r, err := p.client.DiffConfig(ctx, DiffRequest{
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
	ctx = p.ctx(ctx, "")
	argMap, err := p.getMap(req.GetArgs())
	if err != nil {
		return nil, err
	}
	err = p.client.Configure(ctx, ConfigureRequest{
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
	ctx = p.ctx(ctx, "")
	argMap, err := p.getMap(req.GetArgs())
	if err != nil {
		return nil, err
	}
	r, err := p.client.Invoke(ctx, InvokeRequest{
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

func (p *provider) Call(ctx context.Context, req *rpc.CallRequest) (*rpc.CallResponse, error) {

	configPropertyMap := make(presource.PropertyMap, len(req.GetConfig()))
	for k, v := range req.GetConfig() {
		configPropertyMap[presource.PropertyKey(k)] = presource.NewProperty(v)
	}
	pulumiContext, err := pulumi.NewContext(ctx, pulumi.RunInfo{
		Project:           req.GetProject(),
		Stack:             req.GetStack(),
		Config:            req.GetConfig(),
		ConfigSecretKeys:  req.GetConfigSecretKeys(),
		ConfigPropertyMap: configPropertyMap,
		Parallel:          req.GetParallel(),
		DryRun:            req.GetDryRun(),
		MonitorAddr:       req.GetMonitorEndpoint(),
		Organization:      req.GetOrganization(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to build pulumi.Context: %w", err)
	}

	args, err := p.getMap(req.GetArgs())
	if err != nil {
		return nil, fmt.Errorf("unable to convert args into a property map: %w", err)
	}

	resp, err := p.client.Call(ctx, CallRequest{
		Tok:     tokens.ModuleMember(req.GetTok()),
		Args:    args,
		Context: pulumiContext,
	})
	if err != nil {
		return nil, err
	}

	// [comProvider.Call] acts as a synchronization point, ensuring that all actions
	// within p.client.Call are witnessed before Call returns.
	//
	// Eventually, [comProvider.Call] results in a call to [pulumi.Context.wait],
	// which is what forces the synchronization.
	var engineConn *grpc.ClientConn
	if p.host != nil {
		engineConn = p.host.EngineConn()
	}
	_, err = comProvider.Call(ctx, req, engineConn,
		func(ctx *pulumi.Context, tok string, args comProvider.CallArgs) (*comProvider.CallResult, error) {
			return &comProvider.CallResult{}, nil
		})
	if err != nil {
		return nil, err
	}

	returnDependencies := map[string]*rpc.CallResponse_ReturnDependencies{}
	for name, v := range resp.Return {
		var urns []string
		resourcex.Walk(v, func(v presource.PropertyValue, state resourcex.WalkState) {
			if state.Entering || !v.IsOutput() {
				return
			}
			for _, dep := range v.OutputValue().Dependencies {
				urns = append(urns, string(dep))
			}
		})

		returnDependencies[string(name)] = &rpc.CallResponse_ReturnDependencies{Urns: urns}
	}

	_return, err := p.asStruct(resp.Return)
	if err != nil {
		return nil, err
	}

	return &rpc.CallResponse{
		Return:             _return,
		ReturnDependencies: returnDependencies,
		Failures:           checkFailureList(resp.Failures).rpc(),
	}, nil
}

// CallRequest represents a requested resource method invocation.
//
// It corresponds to [rpc.CallRequest] on the wire.
type CallRequest struct {
	Tok     tokens.ModuleMember
	Args    presource.PropertyMap
	Context *pulumi.Context
}

// CallResponse represents a completed resource method invocation.
//
// It corresponds to [rpc.CallResponse] on the wire.
type CallResponse struct {
	// The returned values, if the call was successful.
	Return presource.PropertyMap
	// The failures if any arguments didn't pass verification.
	Failures []CheckFailure
}

func (p *provider) Check(ctx context.Context, req *rpc.CheckRequest) (*rpc.CheckResponse, error) {
	ctx = p.ctx(ctx, presource.URN(req.GetUrn()))
	olds, err := p.getMap(req.GetOlds())
	if err != nil {
		return nil, err
	}
	news, err := p.getMap(req.GetNews())
	if err != nil {
		return nil, err
	}

	r, err := p.client.Check(ctx, CheckRequest{
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
	ctx = p.ctx(ctx, presource.URN(req.GetUrn()))
	olds, err := p.getMap(req.GetOlds())
	if err != nil {
		return nil, err
	}
	news, err := p.getMap(req.GetNews())
	if err != nil {
		return nil, err
	}
	r, err := p.client.Diff(ctx, DiffRequest{
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
	ctx = p.ctx(ctx, presource.URN(req.GetUrn()))
	props, err := p.getMap(req.GetProperties())
	if err != nil {
		return nil, err
	}
	r, err := p.client.Create(ctx, CreateRequest{
		Urn:        presource.URN(req.GetUrn()),
		Properties: props,
		Timeout:    req.GetTimeout(),
		Preview:    req.GetPreview(),
	})
	if initFailed := r.PartialState; initFailed != nil {
		prop, propErr := p.asStruct(r.Properties)
		err = errors.Join(rpcerror.WithDetails(
			rpcerror.New(codes.Unknown, err.Error()),
			&rpc.ErrorResourceInitFailed{
				Id:         r.ID,
				Properties: prop,
				Reasons:    initFailed.Reasons,
			}), propErr)
	}
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
	ctx = p.ctx(ctx, presource.URN(req.GetUrn()))
	propMap, err := p.getMap(req.GetProperties())
	if err != nil {
		return nil, err
	}
	inputMap, err := p.getMap(req.GetInputs())
	if err != nil {
		return nil, err
	}
	r, err := p.client.Read(ctx, ReadRequest{
		ID:         req.GetId(),
		Urn:        presource.URN(req.GetUrn()),
		Properties: propMap,
		Inputs:     inputMap,
	})
	if initFailed := r.PartialState; initFailed != nil {
		props, propErr := p.asStruct(r.Properties)
		inputs, inputsErr := p.asStruct(r.Inputs)
		err = errors.Join(rpcerror.WithDetails(
			rpcerror.New(codes.Unknown, err.Error()),
			&rpc.ErrorResourceInitFailed{
				Id:         r.ID,
				Inputs:     inputs,
				Properties: props,
				Reasons:    initFailed.Reasons,
			}), propErr, inputsErr)
	}
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
	ctx = p.ctx(ctx, presource.URN(req.GetUrn()))
	oldsMap, err := p.getMap(req.GetOlds())
	if err != nil {
		return nil, err
	}
	newsMap, err := p.getMap(req.GetNews())
	if err != nil {
		return nil, err
	}
	r, err := p.client.Update(ctx, UpdateRequest{
		ID:            req.GetId(),
		Urn:           presource.URN(req.GetUrn()),
		Olds:          oldsMap,
		News:          newsMap,
		Timeout:       req.GetTimeout(),
		IgnoreChanges: getIgnoreChanges(req.GetIgnoreChanges()),
		Preview:       req.GetPreview(),
	})
	if initFailed := r.PartialState; initFailed != nil {
		prop, propErr := p.asStruct(r.Properties)
		err = errors.Join(rpcerror.WithDetails(
			rpcerror.New(codes.Unknown, err.Error()),
			&rpc.ErrorResourceInitFailed{
				Id:         req.GetId(),
				Properties: prop,
				Reasons:    initFailed.Reasons,
			}), propErr)
	}
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
	ctx = p.ctx(ctx, presource.URN(req.GetUrn()))
	props, err := p.getMap(req.GetProperties())
	if err != nil {
		return nil, err
	}
	err = p.client.Delete(ctx, DeleteRequest{
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

// ConstructRequest captures enough data to be able to register nested components against the caller's resource
// monitor.
//
// https://github.com/pulumi/pulumi/blob/3e7844a245832c50d8de1f1fc97896f98976ff9b/sdk/go/common/resource/plugin/provider.go#L290
type ConstructRequest struct {
	Urn     presource.URN // the Pulumi URN for this resource.
	Info    ConstructInfo
	Parent  presource.URN
	Inputs  presource.PropertyMap
	Options ConstructOptions
}

// ConstructInfo contains all of the information required to register resources as part of a call to Construct.
type ConstructInfo struct {
	Config           map[pconfig.Key]string // the configuration variables to apply before running.
	ConfigSecretKeys []pconfig.Key          // the configuration keys that have secret values.
	DryRun           bool                   // true if we are performing a dry-run (preview).
	Parallel         int32                  // the degree of parallelism for resource operations (<=1 for serial).
	MonitorEndpoint  string                 // the RPC address to the host resource monitor.
}

// ConstructOptions captures options for a call to Construct.
type ConstructOptions struct {
	// Aliases is the set of aliases for the component.
	Aliases []presource.URN

	// Dependencies is the list of resources this component depends on.
	Dependencies []presource.URN

	// Protect is true if the component is protected.
	Protect bool

	// Providers is a map from package name to provider reference.
	Providers map[string]string

	// InputDependencies is a map from property name to a list of resources that property depends on.
	InputDependencies map[presource.PropertyKey][]presource.URN

	// AdditionalSecretOutputs lists extra output properties
	// that should be treated as secrets.
	AdditionalSecretOutputs []presource.PropertyKey

	// CustomTimeouts overrides default timeouts for resource operations.
	CustomTimeouts *presource.CustomTimeouts

	// DeletedWith specifies that if the given resource is deleted,
	// it will also delete this resource.
	DeletedWith presource.URN

	// DeleteBeforeReplace specifies that replacements of this resource
	// should delete the old resource before creating the new resource.
	DeleteBeforeReplace bool

	// IgnoreChanges lists properties that should be ignored
	// when determining whether the resource should has changed.
	IgnoreChanges []string

	// ReplaceOnChanges lists properties changing which should cause
	// the resource to be replaced.
	ReplaceOnChanges []string

	// RetainOnDelete is true if deletion of the resource should not
	// delete the resource in the provider.
	RetainOnDelete bool
}

type propertyToRPC func(m presource.PropertyMap) (*structpb.Struct, error)

func (c ConstructRequest) rpc(marshal propertyToRPC) *rpc.ConstructRequest {

	// https://github.com/pulumi/pulumi/blob/3e7844a245832c50d8de1f1fc97896f98976ff9b/sdk/go/common/resource/plugin/provider_plugin.go#L1748

	fromUrns := func(urns []presource.URN) []string {
		r := make([]string, len(urns))
		for i, urn := range urns {
			r[i] = string(urn)
		}
		return r
	}

	ptr := func(b bool) *bool {
		return &b
	}

	// Marshal the input properties.
	minputs, err := marshal(c.Inputs)
	if err != nil {
		return nil
	}

	req := &rpc.ConstructRequest{
		Project: string(c.Urn.Project()),
		Stack:   string(c.Urn.Stack()),
		Config: func() map[string]string {
			m := make(map[string]string, len(c.Info.Config))
			for k, v := range c.Info.Config {
				m[k.String()] = v
			}
			return m
		}(),
		ConfigSecretKeys: func() []string {
			keys := make([]string, len(c.Info.ConfigSecretKeys))
			for i, k := range c.Info.ConfigSecretKeys {
				keys[i] = k.String()
			}
			return keys
		}(),
		DryRun:          c.Info.DryRun,
		Parallel:        c.Info.Parallel,
		MonitorEndpoint: c.Info.MonitorEndpoint,
		Type:            string(c.Urn.Type()),
		Name:            string(c.Urn.Name()),
		Parent:          string(c.Parent),
		Inputs:          minputs,
		Protect:         ptr(c.Options.Protect),
		Providers:       c.Options.Providers,
		InputDependencies: func() map[string]*rpc.ConstructRequest_PropertyDependencies {
			m := make(map[string]*rpc.ConstructRequest_PropertyDependencies, len(c.Options.InputDependencies))
			for k, v := range c.Options.InputDependencies {
				m[string(k)] = &rpc.ConstructRequest_PropertyDependencies{
					Urns: fromUrns(v),
				}
			}
			return m
		}(),
		Aliases:      fromUrns(c.Options.Aliases),
		Dependencies: fromUrns(c.Options.Dependencies),
		AdditionalSecretOutputs: func() []string {
			r := make([]string, len(c.Options.AdditionalSecretOutputs))
			for i, k := range c.Options.AdditionalSecretOutputs {
				r[i] = string(k)
			}
			return r
		}(),
		DeletedWith:         string(c.Options.DeletedWith),
		DeleteBeforeReplace: ptr(c.Options.DeleteBeforeReplace),
		IgnoreChanges:       c.Options.IgnoreChanges,
		ReplaceOnChanges:    c.Options.ReplaceOnChanges,
		RetainOnDelete:      ptr(c.Options.RetainOnDelete),
		AcceptsOutputValues: true,
	}

	if ct := c.Options.CustomTimeouts; ct != nil {
		req.CustomTimeouts = &rpc.ConstructRequest_CustomTimeouts{
			Create: (time.Duration(ct.Create) * time.Second).String(),
			Update: (time.Duration(ct.Update) * time.Second).String(),
			Delete: (time.Duration(ct.Delete) * time.Second).String(),
		}
	}

	return req
}

type ConstructResponse struct {
	Urn               presource.URN // the Pulumi URN for this resource.
	State             presource.PropertyMap
	StateDependencies map[presource.PropertyKey][]presource.URN
}

type propertyFromRPC func(m *structpb.Struct) (presource.PropertyMap, error)

func newConstructResponse(req *rpc.ConstructResponse, unmarshal propertyFromRPC) (ConstructResponse, error) {
	toUrns := func(s []string) []presource.URN {
		l := make([]presource.URN, len(s))
		for i, a := range s {
			l[i] = presource.URN(a)
		}
		return l
	}

	// Umarshal the state properties.
	state, err := unmarshal(req.State)
	if err != nil {
		return ConstructResponse{}, err
	}

	r := ConstructResponse{
		Urn:   presource.URN(req.Urn),
		State: state,
		StateDependencies: func() map[presource.PropertyKey][]presource.URN {
			m := make(map[presource.PropertyKey][]presource.URN, len(req.StateDependencies))
			for k, v := range req.StateDependencies {
				m[presource.PropertyKey(k)] = toUrns(v.Urns)
			}
			return m
		}(),
	}
	return r, nil
}

func (c ConstructResponse) rpc(marshal propertyToRPC) *rpc.ConstructResponse {
	fromUrns := func(urns []presource.URN) []string {
		r := make([]string, len(urns))
		for i, urn := range urns {
			r[i] = string(urn)
		}
		return r
	}

	// Marshal the state properties.
	mstate, err := marshal(c.State)
	if err != nil {
		return nil
	}

	return &rpc.ConstructResponse{
		Urn:   string(c.Urn),
		State: mstate,
		StateDependencies: func() map[string]*rpc.ConstructResponse_PropertyDependencies {
			m := make(map[string]*rpc.ConstructResponse_PropertyDependencies, len(c.StateDependencies))
			for k, v := range c.StateDependencies {
				m[string(k)] = &rpc.ConstructResponse_PropertyDependencies{
					Urns: fromUrns(v),
				}
			}
			return m
		}(),
	}
}

func (p *provider) Construct(ctx context.Context, req *rpc.ConstructRequest) (*rpc.ConstructResponse, error) {

	toDurationSecs := func(s string) float64 {
		d, err := time.ParseDuration(s)
		if err != nil {
			return 0
		}
		return d.Seconds()
	}

	toUrns := func(s []string) []presource.URN {
		r := make([]presource.URN, len(s))
		for i, a := range s {
			r[i] = presource.URN(a)
		}
		return r
	}

	parent := presource.URN(req.GetParent())
	urn := presource.NewURN(
		tokens.QName(req.GetStack()),
		tokens.PackageName(req.GetProject()),
		func() tokens.Type {
			if parent == "" {
				return ""
			}
			return parent.Type()
		}(),
		tokens.Type(req.GetType()),
		req.GetName(),
	)

	// see: https://github.com/pulumi/pulumi/blob/3e7844a245832c50d8de1f1fc97896f98976ff9b/sdk/go/common/resource/plugin/provider_plugin.go#L1748

	r := ConstructRequest{
		Urn: urn,
		Info: ConstructInfo{
			Config: func() map[pconfig.Key]string {
				m := make(map[pconfig.Key]string, len(req.GetConfig()))
				for k, v := range req.GetConfig() {
					m[pconfig.MustParseKey(k)] = v
				}
				return m
			}(),
			ConfigSecretKeys: func() []pconfig.Key {
				keys := make([]pconfig.Key, len(req.GetConfigSecretKeys()))
				for i, k := range req.GetConfigSecretKeys() {
					keys[i] = pconfig.MustParseKey(k)
				}
				return keys
			}(),
			DryRun:          req.GetDryRun(),
			Parallel:        req.GetParallel(),
			MonitorEndpoint: req.GetMonitorEndpoint(),
		},
		Parent: parent,
		Inputs: presource.PropertyMap{},
		Options: ConstructOptions{
			Protect:   req.GetProtect(),
			Providers: req.GetProviders(),
			InputDependencies: func() map[presource.PropertyKey][]presource.URN {
				m := make(map[presource.PropertyKey][]presource.URN, len(req.GetInputDependencies()))
				for k, v := range req.GetInputDependencies() {
					m[presource.PropertyKey(k)] = toUrns(v.Urns)
				}
				return m
			}(),
			Aliases:      toUrns(req.GetAliases()),
			Dependencies: toUrns(req.GetDependencies()),
			AdditionalSecretOutputs: func() []presource.PropertyKey {
				r := make([]presource.PropertyKey, len(req.GetAdditionalSecretOutputs()))
				for i, k := range req.GetAdditionalSecretOutputs() {
					r[i] = presource.PropertyKey(k)
				}
				return r
			}(),
			DeletedWith:         presource.URN(req.GetDeletedWith()),
			DeleteBeforeReplace: req.GetDeleteBeforeReplace(),
			IgnoreChanges:       req.GetIgnoreChanges(),
			ReplaceOnChanges:    req.GetReplaceOnChanges(),
			RetainOnDelete:      req.GetRetainOnDelete(),
			CustomTimeouts: func() *presource.CustomTimeouts {
				t := req.GetCustomTimeouts()
				if t == nil {
					return nil
				}
				return &presource.CustomTimeouts{
					Create: toDurationSecs(t.GetCreate()),
					Update: toDurationSecs(t.GetUpdate()),
					Delete: toDurationSecs(t.GetDelete()),
				}
			}(),
		},
	}

	inputs, err := p.getMap(req.Inputs)
	if err != nil {
		return nil, err
	}
	r.Inputs = inputs

	ctx = p.ctx(ctx, urn)
	result, err := p.client.Construct(ctx, r)
	return result.rpc(p.asStruct), err
}

func (h *providerHost) Construct(ctx context.Context, req ConstructRequest, construct comProvider.ConstructFunc) (ConstructResponse, error) {
	r, err := comProvider.Construct(ctx, req.rpc(h.p.asStruct), h.host.EngineConn(), construct)
	if err != nil {
		return ConstructResponse{}, err
	}
	return newConstructResponse(r, h.p.getMap)
}

func (p *provider) Cancel(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	ctx = p.ctx(ctx, "")
	err := p.client.Cancel(ctx)
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil

}

type (
	// ParameterizeRequest configures the provider as parameterized.
	//
	// Parameterize can be called in 2 configurations: with Args non-nil or with Value non-nil. Exactly
	// one of Args or Value will be non-nil. Parameterize should leave the provider in the same state
	// regardless of which variant was used.
	ParameterizeRequest struct {
		// Args indicates that the provider has been configured from the CLI.
		Args *ParameterizeRequestArgs
		// Value re-parameterizes an existing provider.
		Value *ParameterizeRequestValue
	}

	ParameterizeRequestArgs struct {
		// Args is the un-processed CLI args for the parameterization.
		//
		// For example:
		//
		//	pulumi package add my-provider arg1 arg2
		//	                               ^^^^ ^^^^
		//
		// Then ParameterizeRequestArgs{Args:[]string{"arg1", "arg2"}} will be sent.
		Args []string
	}

	// ParameterizeRequestValue represents a re-parameterization from an already generated parameterized
	// SDK.
	//
	// Name and Version will match what was in the ParameterizeResponse that generated the SDK. Value will
	// match what was in the schema returned during SDK generation.
	ParameterizeRequestValue struct {
		Name    string
		Version semver.Version
		Value   []byte
	}

	ParameterizeResponse struct {
		Name    string
		Version semver.Version
	}
)

func (p *provider) Parameterize(ctx context.Context, req *rpc.ParameterizeRequest) (*rpc.ParameterizeResponse, error) {
	var parsedRequest ParameterizeRequest

	switch params := req.Parameters.(type) {
	case *rpc.ParameterizeRequest_Args:
		parsedRequest.Args = &ParameterizeRequestArgs{
			Args: params.Args.GetArgs(),
		}
	case *rpc.ParameterizeRequest_Value:
		version, err := semver.Parse(params.Value.Version)
		if err != nil {
			return nil, rpcerror.Wrapf(codes.InvalidArgument, err, "invalid version %q", params.Value.Version)
		}
		parsedRequest.Value = &ParameterizeRequestValue{
			Name:    params.Value.Name,
			Version: version,
			Value:   params.Value.Value,
		}
	}

	resp, err := p.client.Parameterize(p.ctx(ctx, ""), parsedRequest)
	if err != nil {
		return nil, err
	}

	return &rpc.ParameterizeResponse{
		Name:    resp.Name,
		Version: resp.Version.String(),
	}, nil
}

func (p *provider) GetPluginInfo(context.Context, *emptypb.Empty) (*rpc.PluginInfo, error) {
	return &rpc.PluginInfo{
		Version: p.version,
	}, nil
}

func (p *provider) Attach(_ context.Context, req *rpc.PluginAttach) (*emptypb.Empty, error) {
	host, err := pprovider.NewHostClient(req.GetAddress())
	if err != nil {
		return nil, err
	}
	p.host = host
	return &emptypb.Empty{}, nil
}

// InternalErrorf indicates that the provider encountered an internal error.
//
// This will tell the user that the provider had a bug, and the bug should be reported to
// the provider author.
//
// This is different then [internal.Error], which indicates that pulumi-go-provider has a
// bug.
func InternalErrorf(msg string, a ...any) error {
	return internalError{fmt.Errorf(msg, a...)}
}

type internalError struct{ err error }

func (e internalError) Error() string {
	// If the root cause is an internal error, then we don't need to indicate this is
	// a provider author error.
	if errors.Is(e, internal.Error{}) {
		return e.Unwrap().Error()
	}

	return fmt.Sprintf(`This is an error in the provider. Please report it to the provider author:

%s`, e.err.Error())
}

func (e internalError) Unwrap() error { return e.err }

// GetTypeToken returns the type associated with the current call.
//
// ctx can either be the [context.Context] associated with currently gRPC method being
// served or a [*pulumi.Context] within [github.com/pulumi/pulumi-go-provider/infer]'s
// component resources.
//
// If no type token is available, then the empty string will be returned.
func GetTypeToken[Ctx interface{ Value(any) any }](ctx Ctx) string {
	urn, _ := ctx.Value(key.URN).(presource.URN)
	if urn.IsValid() {
		return urn.Type().String()
	}
	return ""
}

func GetProviderHost(ctx context.Context) ProviderHost {
	if v := ctx.Value(key.ProviderHost); v != nil {
		return v.(ProviderHost)
	}
	return nil
}
