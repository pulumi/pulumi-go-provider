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

type Provider struct {
	p.Provider
	customs    map[string]t.CustomResource
	components map[string]t.ComponentResource
	invokes    map[string]t.Invoke
}

func Wrap(provider p.Provider) *Provider {
	if provider == nil {
		provider = &t.Scaffold{}
	}
	return &Provider{
		Provider:   provider,
		customs:    map[string]t.CustomResource{},
		components: map[string]t.ComponentResource{},
		invokes:    map[string]t.Invoke{},
	}
}

func normalize(tk tokens.Type) string {
	return tk.Module().Name().String() + tokens.TokenDelimiter + tk.Name().String()
}

func fixupError(tk string, err error) error {
	if status.Code(err) == codes.Unimplemented {
		err = status.Errorf(codes.NotFound, "Type '%s' not found", tk)
	}
	return err
}

func (d *Provider) WithCustomResources(resources map[tokens.Type]t.CustomResource) *Provider {
	for k, v := range resources {
		d.customs[normalize(k)] = v
	}
	return d
}

func (d *Provider) WithComponentResources(components map[tokens.Type]t.ComponentResource) *Provider {
	for k, v := range components {
		d.components[normalize(k)] = v
	}
	return d
}

func (d *Provider) WithInvokes(invokes map[tokens.Type]t.Invoke) *Provider {
	for k, v := range invokes {
		d.invokes[normalize(k)] = v
	}
	return d
}

func (d *Provider) Invoke(ctx p.Context, req p.InvokeRequest) (p.InvokeResponse, error) {
	tk := normalize(req.Token)
	inv, ok := d.invokes[tk]
	if ok {
		return inv.Invoke(ctx, req)
	}
	r, err := d.Provider.Invoke(ctx, req)
	return r, fixupError(tk, err)
}

func (d *Provider) Check(ctx p.Context, req p.CheckRequest) (p.CheckResponse, error) {
	tk := normalize(req.Urn.Type())
	r, ok := d.customs[tk]
	if ok {
		return r.Check(ctx, req)
	}
	c, err := d.Provider.Check(ctx, req)
	return c, fixupError(tk, err)
}

func (d *Provider) Diff(ctx p.Context, req p.DiffRequest) (p.DiffResponse, error) {
	tk := normalize(req.Urn.Type())
	r, ok := d.customs[tk]
	if ok {
		return r.Diff(ctx, req)
	}
	diff, err := d.Provider.Diff(ctx, req)
	return diff, fixupError(tk, err)

}

func (d *Provider) Create(ctx p.Context, req p.CreateRequest) (p.CreateResponse, error) {
	tk := normalize(req.Urn.Type())
	r, ok := d.customs[tk]
	if ok {
		return r.Create(ctx, req)
	}
	c, err := d.Provider.Create(ctx, req)
	return c, fixupError(tk, err)
}

func (d *Provider) Read(ctx p.Context, req p.ReadRequest) (p.ReadResponse, error) {
	tk := normalize(req.Urn.Type())
	r, ok := d.customs[tk]
	if ok {
		return r.Read(ctx, req)
	}
	read, err := d.Provider.Read(ctx, req)
	return read, fixupError(tk, err)
}

func (d *Provider) Update(ctx p.Context, req p.UpdateRequest) (p.UpdateResponse, error) {
	tk := normalize(req.Urn.Type())
	r, ok := d.customs[tk]
	if ok {
		return r.Update(ctx, req)
	}
	up, err := d.Provider.Update(ctx, req)
	return up, fixupError(tk, err)
}

func (d *Provider) Delete(ctx p.Context, req p.DeleteRequest) error {
	tk := normalize(req.Urn.Type())
	r, ok := d.customs[tk]
	if ok {
		return r.Delete(ctx, req)
	}
	return fixupError(tk, d.Provider.Delete(ctx, req))
}

func (d *Provider) Construct(pctx p.Context, typ string, name string,
	ctx *pulumi.Context, inputs pprovider.ConstructInputs, opts pulumi.ResourceOption) (pulumi.ComponentResource, error) {
	tk := normalize(tokens.Type(typ))
	r, ok := d.components[tk]
	if ok {
		return r.Construct(pctx, typ, name, ctx, inputs, opts)
	}
	con, err := d.Provider.Construct(pctx, typ, name, ctx, inputs, opts)
	return con, fixupError(typ, err)
}
