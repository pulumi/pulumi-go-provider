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

// Package integration is a test library for validating in-memory providers behave
// correctly.
//
// It sits just above the gRPC level. For full unit testing, see
// [github.com/pulumi/pulumi/pkg/v3/testing/integration].
package integration

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/blang/semver"
	pprovider "github.com/pulumi/pulumi/pkg/v3/resource/provider"
	presource "github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	comProvider "github.com/pulumi/pulumi/sdk/v3/go/pulumi/provider"
	"github.com/stretchr/testify/assert"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/integration/fake"
	"github.com/pulumi/pulumi-go-provider/internal/key"
	internalrpc "github.com/pulumi/pulumi-go-provider/internal/rpc"
)

type Server interface {
	GetSchema(p.GetSchemaRequest) (p.GetSchemaResponse, error)
	Cancel() error
	CheckConfig(p.CheckRequest) (p.CheckResponse, error)
	DiffConfig(p.DiffRequest) (p.DiffResponse, error)
	Configure(p.ConfigureRequest) error
	Invoke(p.InvokeRequest) (p.InvokeResponse, error)
	Check(p.CheckRequest) (p.CheckResponse, error)
	Diff(p.DiffRequest) (p.DiffResponse, error)
	Create(p.CreateRequest) (p.CreateResponse, error)
	Read(p.ReadRequest) (p.ReadResponse, error)
	Update(p.UpdateRequest) (p.UpdateResponse, error)
	Delete(p.DeleteRequest) error
	Construct(p.ConstructRequest) (p.ConstructResponse, error)
	Call(p.CallRequest) (p.CallResponse, error)
}

type ServerOption interface {
	applyServerOption(*serverOptions)
}

// WithMocks allows injecting mock resources, helpful for testing a component
// which cretes child resources.
func WithMocks(mocks pulumi.MockResourceMonitor) ServerOption {
	return mocksOption{mocks: mocks}
}

// WithProvider backs the server with the given concrete provider.
func WithProvider(p p.Provider) ServerOption {
	return providerOption{provider: p}
}

// WithProviderF backs the server with a lazily initialized provider.
func WithProviderF(p func(*pprovider.HostClient) p.Provider) ServerOption {
	return providerFOption{providerF: p}
}

// serverOptions is the internal representation of the effect of
// [ServerOption]s.
type serverOptions struct {
	mocks     pulumi.MockResourceMonitor
	provider  *p.Provider
	providerF func(*pprovider.HostClient) p.Provider
}

type mocksOption struct {
	mocks pulumi.MockResourceMonitor
}

func (mo mocksOption) applyServerOption(opts *serverOptions) {
	opts.mocks = mo.mocks
}

type providerOption struct {
	provider p.Provider
}

func (po providerOption) applyServerOption(opts *serverOptions) {
	opts.provider = &po.provider
}

type providerFOption struct {
	providerF func(*pprovider.HostClient) p.Provider
}

func (po providerFOption) applyServerOption(opts *serverOptions) {
	opts.providerF = po.providerF
}

// NewServer constructs a gRPC server for testing the given provider.
//
// Must be called with WithProvider or WithProviderF.
func NewServer(ctx context.Context, pkg string, version semver.Version,
	opts ...ServerOption,
) (Server, error) {
	o := &serverOptions{}
	for _, opt := range opts {
		opt.applyServerOption(o)
	}
	if o.mocks == nil {
		o.mocks = &MockMonitor{} // MockResourceMonitor requires a non-nil monitor.
	}
	if o.provider == nil && o.providerF == nil {
		return nil, fmt.Errorf("WithProvider or WithProviderF is required")
	}

	host := newHost(ctx, o.mocks)

	var prov p.Provider
	if o.provider != nil {
		prov = *o.provider
	}
	if o.providerF != nil {
		host.lazyInit()
		prov = o.providerF(host.client)
	}

	s := &server{p.RunInfo{
		PackageName: pkg,
		Version:     version.String(),
	}, host, prov.WithDefaults(), ctx}

	return s, nil
}

// Server hosts a [Provider] for integration test purposes.
type server struct {
	runInfo p.RunInfo
	host    *host
	p       p.Provider
	context context.Context
}

type host struct {
	lazyInit func()

	engine      *fake.EngineServer
	engineAddr  string
	client      *pprovider.HostClient
	monitor     *fake.ResourceMonitorServer
	monitorAddr string
}

func newHost(ctx context.Context, m pulumi.MockResourceMonitor) *host {
	h := &host{
		engine:  fake.NewEngineServer(),
		monitor: fake.NewResourceMonitorServer(m),
	}
	h.lazyInit = sync.OnceFunc(func() {
		// Start the fake Pulumi engine server and connect to it.
		// Note that the servers and connections are automatically closed when the context is cancelled.
		engineCtx, engineCancel := context.WithCancel(ctx)
		engineAddr, engineDone, err := fake.StartEngineServer(engineCtx, h.engine)
		if err != nil {
			panic(fmt.Errorf("could not start engine server: %w", err))
		}
		h.engineAddr = engineAddr

		hc, err := pprovider.NewHostClient(h.engineAddr)
		if err != nil {
			panic(err)
		}
		h.client = hc

		monitorCtx, monitorCancel := context.WithCancel(ctx)
		monitorAddr, monitorDone, err := fake.StartMonitorServer(monitorCtx, h.monitor)
		if err != nil {
			panic(fmt.Errorf("could not start monitor server: %w", err))
		}
		h.monitorAddr = monitorAddr

		go func() {
			<-ctx.Done()
			monitorCancel()
			<-monitorDone
			_ = h.client.EngineConn().Close()
			engineCancel()
			<-engineDone
		}()
	})
	return h
}

func (s *server) ctx(urn presource.URN) context.Context {
	ctx := s.context
	if urn != "" {
		ctx = context.WithValue(ctx, key.URN, urn)
	}
	ctx = context.WithValue(ctx, key.RuntimeInfo, s.runInfo)
	ctx = context.WithValue(ctx, key.ProviderHost, s.host)
	return ctx
}

func (s *server) GetSchema(req p.GetSchemaRequest) (p.GetSchemaResponse, error) {
	return s.p.GetSchema(s.ctx(""), req)
}

func (s *server) Cancel() error {
	return s.p.Cancel(s.ctx(""))
}

func (s *server) CheckConfig(req p.CheckRequest) (p.CheckResponse, error) {
	return s.p.CheckConfig(s.ctx(""), req)
}

func (s *server) DiffConfig(req p.DiffRequest) (p.DiffResponse, error) {
	return s.p.DiffConfig(s.ctx(""), req)
}

func (s *server) Configure(req p.ConfigureRequest) error {
	return s.p.Configure(s.ctx(""), req)
}

func (s *server) Invoke(req p.InvokeRequest) (p.InvokeResponse, error) {
	return s.p.Invoke(s.ctx(presource.URN(req.Token)), req)
}

func (s *server) Check(req p.CheckRequest) (p.CheckResponse, error) {
	return s.p.Check(s.ctx(req.Urn), req)
}

func (s *server) Diff(req p.DiffRequest) (p.DiffResponse, error) {
	return s.p.Diff(s.ctx(req.Urn), req)
}

func (s *server) Create(req p.CreateRequest) (p.CreateResponse, error) {
	return s.p.Create(s.ctx(req.Urn), req)
}

func (s *server) Read(req p.ReadRequest) (p.ReadResponse, error) {
	return s.p.Read(s.ctx(req.Urn), req)
}

func (s *server) Update(req p.UpdateRequest) (p.UpdateResponse, error) {
	return s.p.Update(s.ctx(req.Urn), req)
}

func (s *server) Delete(req p.DeleteRequest) error {
	return s.p.Delete(s.ctx(req.Urn), req)
}

func (s *server) Construct(req p.ConstructRequest) (p.ConstructResponse, error) {
	// apply some defaults for convenenience
	if req.Parallel < 1 {
		req.Parallel = 1
	}

	return s.p.Construct(s.ctx(req.Urn), req)
}

var _ = (p.Host)(&host{})

// Construct implements the host interface to allow the provider to construct resources.
func (h *host) Construct(ctx context.Context, req p.ConstructRequest, construct comProvider.ConstructFunc,
) (p.ConstructResponse, error) {
	// Use the fake engine to create a pulumi context,
	// and then call the user's construct function with the context.
	// the function is expected to register resources, which will be
	// handled by the mock monitor.

	h.lazyInit()
	req.MonitorEndpoint = h.monitorAddr

	comReq := linkedConstructRequestToRPC(&req, internalrpc.MarshalProperties)
	comResp, err := comProvider.Construct(ctx, comReq, h.client.EngineConn(), construct)
	if err != nil {
		return p.ConstructResponse{}, err
	}

	return linkedConstructResponseFromRPC(comResp)
}

func (s *server) Call(req p.CallRequest) (p.CallResponse, error) {
	// apply some defaults for convenience
	if req.Parallel < 1 {
		req.Parallel = 1
	}
	return s.p.Call(s.ctx(""), req)
}

func (h *host) Call(ctx context.Context, req p.CallRequest, call comProvider.CallFunc,
) (p.CallResponse, error) {
	// Use the fake engine to create a pulumi context,
	// and then call the user's call function with the context.
	// the function is expected to register resources, which will be
	// handled by the mock monitor.

	h.lazyInit()
	req.MonitorEndpoint = h.monitorAddr

	comReq := linkedCallRequestToRPC(&req, internalrpc.MarshalProperties)
	comResp, err := comProvider.Call(ctx, comReq, h.client.EngineConn(), call)
	if err != nil {
		return p.CallResponse{}, err
	}

	return linkedCallResponseFromRPC(comResp)
}

type MockMonitor struct {
	CallF        func(args pulumi.MockCallArgs) (presource.PropertyMap, error)
	NewResourceF func(args pulumi.MockResourceArgs) (string, presource.PropertyMap, error)
}

func (m *MockMonitor) Call(args pulumi.MockCallArgs) (presource.PropertyMap, error) {
	if m.CallF == nil {
		return presource.PropertyMap{}, nil
	}
	return m.CallF(args)
}

func (m *MockMonitor) NewResource(args pulumi.MockResourceArgs) (string, presource.PropertyMap, error) {
	if m.NewResourceF == nil {
		return args.Name, args.Inputs, nil
	}
	return m.NewResourceF(args)
}

// Operation describes a step in a [LifeCycleTest].
//
// TODO: Add support for diff verification.
type Operation struct {
	// The inputs for the operation
	Inputs property.Map
	// The expected output for the operation. If ExpectedOutput is nil, no check will be made.
	ExpectedOutput *property.Map
	// A function called on the output of this operation.
	Hook func(inputs, output property.Map)
	// If the test should expect the operation to signal an error.
	ExpectFailure bool
	// If CheckFailures is non-nil, expect the check step to fail with the provided output.
	CheckFailures []p.CheckFailure
}

// LifeCycleTest describing the lifecycle of a resource test.
type LifeCycleTest struct {
	Resource tokens.Type
	Create   Operation
	Updates  []Operation
}

// Run a resource through it's lifecycle asserting that its output is as expected.
// The resource is
//
// 1. Previewed.
// 2. Created.
// 2. Previewed and Updated for each update in the Updates list.
// 3. Deleted.
func (l LifeCycleTest) Run(t *testing.T, server Server) {
	urn := presource.NewURN("test", "provider", "", l.Resource, "test")

	runCreate := func(op Operation) (p.CreateResponse, bool) {
		// Here we do the create and the initial setup
		checkResponse, err := server.Check(p.CheckRequest{
			Urn:    urn,
			State:  property.Map{},
			Inputs: op.Inputs,
		})
		assert.NoError(t, err, "resource check errored")
		if len(op.CheckFailures) > 0 || len(checkResponse.Failures) > 0 {
			assert.ElementsMatch(t, op.CheckFailures, checkResponse.Failures,
				"check failures mismatch on create")
			return p.CreateResponse{}, false
		}

		_, err = server.Create(p.CreateRequest{
			Urn:        urn,
			Properties: checkResponse.Inputs,
			DryRun:     true,
		})
		// We allow the failure from ExpectFailure to hit at either the preview or the Create.
		if op.ExpectFailure && err != nil {
			return p.CreateResponse{}, false
		}
		createResponse, err := server.Create(p.CreateRequest{
			Urn:        urn,
			Properties: checkResponse.Inputs,
		})
		if op.ExpectFailure {
			assert.Error(t, err, "expected an error on create")
			return p.CreateResponse{}, false
		}
		assert.NoError(t, err, "failed to run the create")
		if err != nil {
			return p.CreateResponse{}, false
		}
		if op.Hook != nil {
			op.Hook(checkResponse.Inputs, createResponse.Properties)
		}
		if op.ExpectedOutput != nil {
			assert.EqualValues(t, op.ExpectedOutput, createResponse.Properties, "create outputs")
		}
		return createResponse, true
	}

	createResponse, keepGoing := runCreate(l.Create)
	if !keepGoing {
		return
	}

	id := createResponse.ID
	olds := createResponse.Properties
	for i, update := range l.Updates {
		// Perform the check
		check, err := server.Check(p.CheckRequest{
			Urn:    urn,
			State:  olds,
			Inputs: update.Inputs,
		})

		assert.NoErrorf(t, err, "check returned an error on update %d", i)
		if err != nil {
			return
		}
		if len(update.CheckFailures) > 0 || len(check.Failures) > 0 {
			assert.ElementsMatchf(t, update.CheckFailures, check.Failures,
				"check failures mismatch on update %d", i)
			continue
		}

		diff, err := server.Diff(p.DiffRequest{
			ID:     id,
			Urn:    urn,
			State:  olds,
			Inputs: check.Inputs,
		})
		assert.NoErrorf(t, err, "diff failed on update %d", i)
		if err != nil {
			return
		}
		if !diff.HasChanges {
			// We don't have any changes, so we can just do nothing
			continue
		}
		isDelete := false
		for _, v := range diff.DetailedDiff {
			switch v.Kind {
			case p.AddReplace:
				fallthrough
			case p.DeleteReplace:
				fallthrough
			case p.UpdateReplace:
				isDelete = true
			}
		}
		if isDelete {
			runDelete := func() {
				err = server.Delete(p.DeleteRequest{
					ID:         id,
					Urn:        urn,
					Properties: olds,
				})
				assert.NoError(t, err, "failed to delete the resource")
			}
			if diff.DeleteBeforeReplace {
				runDelete()
				result, keepGoing := runCreate(update)
				if !keepGoing {
					continue
				}
				id = result.ID
				olds = result.Properties
			} else {
				result, keepGoing := runCreate(update)
				if !keepGoing {
					continue
				}

				runDelete()
				// Set the new block
				id = result.ID
				olds = result.Properties
			}
		} else {

			// Now perform the preview
			_, err = server.Update(p.UpdateRequest{
				ID:     id,
				Urn:    urn,
				State:  olds,
				Inputs: check.Inputs,
				DryRun: true,
			})

			if update.ExpectFailure && err != nil {
				continue
			}

			result, err := server.Update(p.UpdateRequest{
				ID:     id,
				Urn:    urn,
				State:  olds,
				Inputs: check.Inputs,
			})
			if !update.ExpectFailure && err != nil {
				assert.NoError(t, err, "failed to update the resource")
				continue
			}
			if update.ExpectFailure {
				assert.Errorf(t, err, "expected failure on update %d", i)
				continue
			}
			if update.Hook != nil {
				update.Hook(check.Inputs, result.Properties)
			}
			if update.ExpectedOutput != nil {
				assert.EqualValues(t, update.ExpectedOutput, result.Properties, "expected output on update %d", i)
			}
			olds = result.Properties
		}
	}
	err := server.Delete(p.DeleteRequest{
		ID:         id,
		Urn:        urn,
		Properties: olds,
	})
	assert.NoError(t, err, "failed to delete the resource")
}
