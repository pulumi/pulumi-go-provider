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
	p "github.com/pulumi/pulumi-go-provider"
)

// The function applied to each provider.Context that passes through this Provider.
type Wrapper = func(p.Context) p.Context

// Create a Provider that calls `wrapper` on each context passed into `provider`.
func Wrap(provider p.Provider, wrapper Wrapper) p.Provider {
	return p.Provider{
		GetSchema: func(ctx p.Context, req p.GetSchemaRequest) (p.GetSchemaResponse, error) {
			return provider.GetSchema(wrapper(ctx), req)
		},
		Cancel: func(ctx p.Context) error {
			return provider.Cancel(wrapper(ctx))
		},
		CheckConfig: func(ctx p.Context, req p.CheckRequest) (p.CheckResponse, error) {
			return provider.CheckConfig(wrapper(ctx), req)
		},
		DiffConfig: func(ctx p.Context, req p.DiffRequest) (p.DiffResponse, error) {
			return provider.DiffConfig(wrapper(ctx), req)
		},
		Configure: func(ctx p.Context, req p.ConfigureRequest) error {
			return provider.Configure(wrapper(ctx), req)
		},
		Invoke: func(ctx p.Context, req p.InvokeRequest) (p.InvokeResponse, error) {
			return provider.Invoke(wrapper(ctx), req)
		},
		Check: func(ctx p.Context, req p.CheckRequest) (p.CheckResponse, error) {
			return provider.Check(wrapper(ctx), req)
		},
		Diff: func(ctx p.Context, req p.DiffRequest) (p.DiffResponse, error) {
			return provider.Diff(wrapper(ctx), req)
		},
		Create: func(ctx p.Context, req p.CreateRequest) (p.CreateResponse, error) {
			return provider.Create(wrapper(ctx), req)
		},
		Read: func(ctx p.Context, req p.ReadRequest) (p.ReadResponse, error) {
			return provider.Read(wrapper(ctx), req)
		},
		Update: func(ctx p.Context, req p.UpdateRequest) (p.UpdateResponse, error) {
			return provider.Update(wrapper(ctx), req)
		},
		Delete: func(ctx p.Context, req p.DeleteRequest) error {
			return provider.Delete(wrapper(ctx), req)
		},
		Construct: func(ctx p.Context, req p.ConstructRequest) (p.ConstructResponse, error) {
			return provider.Construct(wrapper(ctx), req)
		},
	}
}
