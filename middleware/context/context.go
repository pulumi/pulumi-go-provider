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

// Context allows systemic wrapping of provider.Context before invoking a subsidiary provider.
package context

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	pprovider "github.com/pulumi/pulumi/sdk/v3/go/pulumi/provider"

	p "github.com/pulumi/pulumi-go-provider"
)

// The function applied to each provider.Context that passes through this Provider.
type Wrapper = func(p.Context) p.Context

// Create a Provider that calls `wrapper` on each context passed into `provider`.
func Wrap(provider p.Provider, wrapper Wrapper) p.Provider {
	return &wrapProvider{provider, wrapper}
}

type wrapProvider struct {
	inner p.Provider
	w     Wrapper
}

func (s *wrapProvider) GetSchema(ctx p.Context, req p.GetSchemaRequest) (p.GetSchemaResponse, error) {
	return s.inner.GetSchema(s.w(ctx), req)
}

func (s *wrapProvider) Cancel(ctx p.Context) error {
	return s.inner.Cancel(s.w(ctx))
}

func (s *wrapProvider) CheckConfig(ctx p.Context, req p.CheckRequest) (p.CheckResponse, error) {
	return s.inner.CheckConfig(s.w(ctx), req)
}

func (s *wrapProvider) DiffConfig(ctx p.Context, req p.DiffRequest) (p.DiffResponse, error) {
	return s.inner.DiffConfig(s.w(ctx), req)
}

func (s *wrapProvider) Configure(ctx p.Context, req p.ConfigureRequest) error {
	return s.inner.Configure(s.w(ctx), req)
}

func (s *wrapProvider) Invoke(ctx p.Context, req p.InvokeRequest) (p.InvokeResponse, error) {
	return s.inner.Invoke(s.w(ctx), req)
}

func (s *wrapProvider) Check(ctx p.Context, req p.CheckRequest) (p.CheckResponse, error) {
	return s.inner.Check(s.w(ctx), req)
}

func (s *wrapProvider) Diff(ctx p.Context, req p.DiffRequest) (p.DiffResponse, error) {
	return s.inner.Diff(s.w(ctx), req)
}

func (s *wrapProvider) Create(ctx p.Context, req p.CreateRequest) (p.CreateResponse, error) {
	return s.inner.Create(s.w(ctx), req)
}

func (s *wrapProvider) Read(ctx p.Context, req p.ReadRequest) (p.ReadResponse, error) {
	return s.inner.Read(s.w(ctx), req)
}

func (s *wrapProvider) Update(ctx p.Context, req p.UpdateRequest) (p.UpdateResponse, error) {
	return s.inner.Update(s.w(ctx), req)
}

func (s *wrapProvider) Delete(ctx p.Context, req p.DeleteRequest) error {
	return s.inner.Delete(s.w(ctx), req)
}

func (s *wrapProvider) Construct(pctx p.Context, typ string, name string,
	ctx *pulumi.Context, inputs pprovider.ConstructInputs, opts pulumi.ResourceOption) (pulumi.ComponentResource, error) {
	return s.inner.Construct(s.w(pctx), typ, name, ctx, inputs, opts)
}
