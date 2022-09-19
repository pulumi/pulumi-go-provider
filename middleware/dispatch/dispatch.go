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

// A provider that dispatches provider level calls such as `Create` to resource level
// invocations.
package dispatch

import (
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	pprovider "github.com/pulumi/pulumi/sdk/v3/go/pulumi/provider"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	p "github.com/pulumi/pulumi-go-provider"
	t "github.com/pulumi/pulumi-go-provider/middleware"
)

// Create a new Dispatch provider around another provider. If `provider` is nil then an
// empty provider will be used.
func Wrap(provider p.Provider, opts Options) p.Provider {
	fix := func(tk tokens.Type) string {
		m := tk.Module().Name()
		if opts.ModuleMap != nil {
			if mod, ok := opts.ModuleMap[m]; ok {
				m = mod
			}
		}
		return m.String() + tokens.TokenDelimiter + tk.Name().String()
	}

	new := provider
	if len(opts.Invokes) > 0 {
		invokes := map[string]t.Invoke{}
		for k, v := range opts.Invokes {
			invokes[fix(k)] = v
		}
		new.Invoke = func(ctx p.Context, req p.InvokeRequest) (p.InvokeResponse, error) {
			tk := fix(req.Token)
			inv, ok := invokes[tk]
			if ok {
				return inv.Invoke(ctx, req)
			} else if provider.Invoke != nil {
				return provider.Invoke(ctx, req)
			}
			return p.InvokeResponse{}, status.Errorf(codes.NotFound, "Invoke '%s' not found", tk)
		}
	}
	if len(opts.Customs) > 0 {
		customs := map[string]t.CustomResource{}
		for k, v := range opts.Customs {
			customs[fix(k)] = v
		}
		notFound := func(tk string) error {
			return status.Errorf(codes.NotFound, "Resource '%s' not found", tk)
		}
		new.Check = func(ctx p.Context, req p.CheckRequest) (p.CheckResponse, error) {
			tk := fix(req.Urn.Type())
			r, ok := customs[tk]
			if ok {
				return r.Check(ctx, req)
			} else if provider.Check != nil {
				return provider.Check(ctx, req)
			}
			return p.CheckResponse{}, notFound(tk)
		}
		new.Diff = func(ctx p.Context, req p.DiffRequest) (p.DiffResponse, error) {
			tk := fix(req.Urn.Type())
			r, ok := customs[tk]
			if ok {
				return r.Diff(ctx, req)
			} else if provider.Diff != nil {
				return provider.Diff(ctx, req)
			}
			return p.DiffResponse{}, notFound(tk)
		}
		new.Create = func(ctx p.Context, req p.CreateRequest) (p.CreateResponse, error) {
			tk := fix(req.Urn.Type())
			r, ok := customs[tk]
			if ok {
				return r.Create(ctx, req)
			} else if provider.Create != nil {
				return provider.Create(ctx, req)
			}
			return p.CreateResponse{}, notFound(tk)
		}
		new.Read = func(ctx p.Context, req p.ReadRequest) (p.ReadResponse, error) {
			tk := fix(req.Urn.Type())
			r, ok := customs[tk]
			if ok {
				return r.Read(ctx, req)
			} else if provider.Read != nil {
				return provider.Read(ctx, req)
			}
			return p.ReadResponse{}, notFound(tk)
		}
		new.Update = func(ctx p.Context, req p.UpdateRequest) (p.UpdateResponse, error) {
			tk := fix(req.Urn.Type())
			r, ok := customs[tk]
			if ok {
				return r.Update(ctx, req)
			} else if provider.Update != nil {
				return provider.Update(ctx, req)
			}
			return p.UpdateResponse{}, notFound(tk)
		}
		new.Delete = func(ctx p.Context, req p.DeleteRequest) error {
			tk := fix(req.Urn.Type())
			r, ok := customs[tk]
			if ok {
				return r.Delete(ctx, req)
			} else if provider.Delete != nil {
				return provider.Delete(ctx, req)
			}
			return notFound(tk)
		}
	}
	if len(opts.Components) > 0 {
		components := map[string]t.ComponentResource{}
		for k, v := range opts.Components {
			components[fix(k)] = v
		}

		new.Construct = func(pctx p.Context, typ string, name string,
			ctx *pulumi.Context, inputs pprovider.ConstructInputs, opts pulumi.ResourceOption,
		) (pulumi.ComponentResource, error) {
			tk := fix(tokens.Type(typ))
			r, ok := components[tk]
			if ok {
				return r.Construct(pctx, typ, name, ctx, inputs, opts)
			} else if provider.Construct != nil {
				return provider.Construct(pctx, typ, name, ctx, inputs, opts)
			}
			return nil, status.Errorf(codes.NotFound, "Component Resource '%s' not found", tk)
		}
	}

	return new
}

type Options struct {
	Customs    map[tokens.Type]t.CustomResource
	Components map[tokens.Type]t.ComponentResource
	Invokes    map[tokens.Type]t.Invoke
	ModuleMap  map[tokens.ModuleName]tokens.ModuleName
}
