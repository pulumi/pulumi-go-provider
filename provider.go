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
	"io"
	"os"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	pprovider "github.com/pulumi/pulumi/pkg/v3/resource/provider"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	presource "github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil/rpcerror"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	comProvider "github.com/pulumi/pulumi/sdk/v3/go/pulumi/provider"
	rpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/structpb"
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

// Provide a structured error for missing provider keys.
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
	GetSchema func(Context, GetSchemaRequest) (GetSchemaResponse, error)
	// Cancel signals the provider to gracefully shut down and abort any ongoing resource operations.
	// Operations aborted in this way will return an error (e.g., `Update` and `Create` will either return a
	// creation error or an initialization error). Since Cancel is advisory and non-blocking, it is up
	// to the host to decide how long to wait after Cancel is called before (e.g.)
	// hard-closing any gRPC connection.
	Cancel func(Context) error

	// Provider Config
	CheckConfig func(Context, CheckRequest) (CheckResponse, error)
	DiffConfig  func(Context, DiffRequest) (DiffResponse, error)
	// NOTE: We opt into all options.
	Configure func(Context, ConfigureRequest) error

	// Invokes
	Invoke func(Context, InvokeRequest) (InvokeResponse, error)
	// TODO Stream invoke (are those used anywhere)

	// Custom Resources

	// Check validates that the given property bag is valid for a resource of the given type and returns the inputs
	// that should be passed to successive calls to Diff, Create, or Update for this resource. As a rule, the provider
	// inputs returned by a call to Check should preserve the original representation of the properties as present in
	// the program inputs. Though this rule is not required for correctness, violations thereof can negatively impact
	// the end-user experience, as the provider inputs are using for detecting and rendering diffs.
	Check  func(Context, CheckRequest) (CheckResponse, error)
	Diff   func(Context, DiffRequest) (DiffResponse, error)
	Create func(Context, CreateRequest) (CreateResponse, error)
	Read   func(Context, ReadRequest) (ReadResponse, error)
	Update func(Context, UpdateRequest) (UpdateResponse, error)
	Delete func(Context, DeleteRequest) error
	// TODO Call

	// Components Resources
	Construct func(pctx Context, typ string, name string,
		ctx *pulumi.Context, inputs comProvider.ConstructInputs, opts pulumi.ResourceOption) (pulumi.ComponentResource, error)
}

// Provide a default value for each function.
//
// Most default values return a NotYetImplemented error, which the engine knows to ignore.
// Others are no-op functions.
//
// You should not need to call this function manually. It will be automatically called
// before a provider is run.
func (d Provider) WithDefaults() Provider {
	nyi := func(fn string) error {
		return status.Errorf(codes.Unimplemented, "%s is not implemented", fn)
	}
	if d.GetSchema == nil {
		d.GetSchema = func(Context, GetSchemaRequest) (GetSchemaResponse, error) {
			return GetSchemaResponse{}, nyi("GetSchema")
		}
	}
	if d.Cancel == nil {
		d.Cancel = func(ctx Context) error {
			return nyi("Cancel")
		}
	}
	if d.CheckConfig == nil {
		d.CheckConfig = func(ctx Context, req CheckRequest) (CheckResponse, error) {
			return CheckResponse{}, nyi("CheckConfig")
		}
	}
	if d.DiffConfig == nil {
		d.DiffConfig = func(ctx Context, req DiffRequest) (DiffResponse, error) {
			return DiffResponse{}, nyi("DiffConfig")
		}
	}
	if d.Configure == nil {
		d.Configure = func(ctx Context, req ConfigureRequest) error {
			return nyi("Configure")
		}
	}
	if d.Invoke == nil {
		d.Invoke = func(ctx Context, req InvokeRequest) (InvokeResponse, error) {
			return InvokeResponse{}, nyi("Invoke")
		}
	}
	if d.Check == nil {
		d.Check = func(ctx Context, req CheckRequest) (CheckResponse, error) {
			return CheckResponse{}, nyi("Check")
		}
	}
	if d.Diff == nil {
		d.Diff = func(ctx Context, req DiffRequest) (DiffResponse, error) {
			return DiffResponse{}, nyi("Diff")
		}
	}
	if d.Create == nil {
		d.Create = func(ctx Context, req CreateRequest) (CreateResponse, error) {
			return CreateResponse{}, nyi("Create")
		}
	}
	if d.Read == nil {
		d.Read = func(ctx Context, req ReadRequest) (ReadResponse, error) {
			return ReadResponse{}, nyi("Read")
		}
	}
	if d.Update == nil {
		d.Update = func(ctx Context, req UpdateRequest) (UpdateResponse, error) {
			return UpdateResponse{}, nyi("Update")
		}
	}
	if d.Delete == nil {
		d.Delete = func(ctx Context, req DeleteRequest) error {
			return nyi("Delete")
		}
	}
	if d.Construct == nil {
		d.Construct = func(pctx Context, typ string, name string,
			ctx *pulumi.Context, inputs comProvider.ConstructInputs, opts pulumi.ResourceOption) (pulumi.ComponentResource, error) {
			return nil, nyi("Construct")
		}
	}
	return d
}

// Run a provider with the given name and version.
func RunProvider(name, version string, provider Provider) error {
	return pprovider.Main(name, newProvider(name, version, provider.WithDefaults()))
}

// A context which prints its diagnostics, collecting all errors
type errCollectingContext struct {
	context.Context
	errs   multierror.Error
	info   RunInfo
	stderr io.Writer
}

func (e *errCollectingContext) Log(severity diag.Severity, msg string) {
	if severity == diag.Error {
		e.errs.Errors = append(e.errs.Errors, fmt.Errorf(msg))
	}
	fmt.Fprintf(e.stderr, "Log(%s): %s\n", severity, msg)
}

func (e *errCollectingContext) Logf(severity diag.Severity, msg string, args ...any) {
	e.Log(severity, fmt.Sprintf(msg, args...))
}

func (e *errCollectingContext) LogStatus(severity diag.Severity, msg string) {
	if severity == diag.Error {
		e.errs.Errors = append(e.errs.Errors, fmt.Errorf(msg))
	}
	fmt.Fprintf(e.stderr, "LogStatus(%s): %s\n", severity, msg)
}

func (e *errCollectingContext) LogStatusf(severity diag.Severity, msg string, args ...any) {
	e.LogStatus(severity, fmt.Sprintf(msg, args...))
}

func (e *errCollectingContext) RuntimeInformation() RunInfo {
	return e.info
}

// Retrieve the schema from the provider by invoking GetSchema on the provider.
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
	for _, err := range collectingDiag.errs.Errors {
		errs.Errors = append(errs.Errors, err)
	}
	spec := schema.PackageSpec{}
	if err := errs.ErrorOrNil(); err != nil {
		return spec, err
	}
	err = json.Unmarshal([]byte(s.Schema), &spec)
	return spec, err
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

type wrapCtx struct {
	context.Context
	log                func(severity diag.Severity, msg string)
	logf               func(severity diag.Severity, msg string, args ...any)
	logStatus          func(severity diag.Severity, msg string)
	logStatusf         func(severity diag.Severity, msg string, args ...any)
	runtimeInformation func() RunInfo
}

// replaceContext replaces the embedded context.Context in a Context.
func replaceContext(ctx Context, new context.Context) Context { //nolint:revive
	switch ctx := ctx.(type) {
	case *wrapCtx:
		return &wrapCtx{
			Context:            new,
			log:                ctx.log,
			logf:               ctx.logf,
			logStatus:          ctx.logStatus,
			logStatusf:         ctx.logStatusf,
			runtimeInformation: ctx.runtimeInformation,
		}
	case *pkgContext:
		return &pkgContext{
			Context:  new,
			provider: ctx.provider,
			urn:      ctx.urn,
		}
	default:
		return &wrapCtx{
			Context:            new,
			log:                ctx.Log,
			logf:               ctx.Logf,
			logStatus:          ctx.LogStatus,
			logStatusf:         ctx.LogStatusf,
			runtimeInformation: ctx.RuntimeInformation,
		}
	}
}

func (c *wrapCtx) Log(severity diag.Severity, msg string) { c.log(severity, msg) }
func (c *wrapCtx) Logf(severity diag.Severity, msg string, args ...any) {
	c.logf(severity, msg, args...)
}
func (c *wrapCtx) LogStatus(severity diag.Severity, msg string) { c.logStatus(severity, msg) }
func (c *wrapCtx) LogStatusf(severity diag.Severity, msg string, args ...any) {
	c.logStatusf(severity, msg, args...)
}
func (c *wrapCtx) RuntimeInformation() RunInfo { return c.runtimeInformation() }

// Add a value to a Context. This is the moral equivalent to context.WithValue from the Go
// standard library.
func CtxWithValue(ctx Context, key, value any) Context {
	return replaceContext(ctx, context.WithValue(ctx, key, value))
}

func CtxWithCancel(ctx Context) (Context, context.CancelFunc) {
	c, cancel := context.WithCancel(ctx)
	return replaceContext(ctx, c), cancel
}

func CtxWithTimeout(ctx Context, timeout time.Duration) (Context, context.CancelFunc) {
	c, cancel := context.WithTimeout(ctx, timeout)
	return replaceContext(ctx, c), cancel
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
	return comProvider.Construct(pctx, req, p.host.EngineConn(),
		func(ctx *pulumi.Context, typ, name string,
			inputs comProvider.ConstructInputs, opts pulumi.ResourceOption,
		) (*comProvider.ConstructResult, error) {
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
