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
	"sync"

	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	pprovider "github.com/pulumi/pulumi/sdk/v3/go/pulumi/provider"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	p "github.com/pulumi/pulumi-go-provider"
	t "github.com/pulumi/pulumi-go-provider/middleware"
)

// A provider that dispatches URN based methods to their appropriate go instance.
type Provider struct {
	// The underlying provider if any
	p.Provider

	// The actual items given to the provider to dispatch.
	customs    map[tokens.Type]t.CustomResource
	components map[tokens.Type]t.ComponentResource
	invokes    map[tokens.Type]t.Invoke

	// Maps of the above items noramalized to remove the package name and to account for
	// the module map. These can be nil and are lazily regenerated on demand.
	normalizedCustoms    rwMap[string, t.CustomResource]
	normalizedComponents rwMap[string, t.ComponentResource]
	normalizedInvokes    rwMap[string, t.Invoke]

	// A map of token name replacements. Given map{k: v}, pkg:k:Name will be replaced with
	// pkg:v:Name.
	moduleMap map[tokens.ModuleName]tokens.ModuleName
}

type rwMap[K comparable, V any] struct {
	m     *sync.RWMutex
	store map[K]V
}

func newRWMap[K comparable, V any]() rwMap[K, V] {
	return rwMap[K, V]{
		m: new(sync.RWMutex),
	}
}

func (c *rwMap[K, V]) Reset() {
	c.m.Lock()
	defer c.m.Unlock()
	c.store = nil
}

func (c *rwMap[K, V]) Initialize(f func(map[K]V)) {
	c.m.Lock()
	defer c.m.Unlock()
	if c.store == nil {
		c.store = map[K]V{}
		f(c.store)
	}
}
func (c *rwMap[K, V]) Load(k K) (V, bool) {
	c.m.RLock()
	defer c.m.RUnlock()
	if c.store == nil {
		var v V
		return v, false
	}
	v, ok := c.store[k]
	return v, ok
}

// Create a new Dispatch provider around another provider. If `provider` is nil then an
// empty provider will be used.
func Wrap(provider p.Provider) *Provider {
	if provider == nil {
		provider = &t.Scaffold{}
	}
	return &Provider{
		Provider:   provider,
		customs:    map[tokens.Type]t.CustomResource{},
		components: map[tokens.Type]t.ComponentResource{},
		invokes:    map[tokens.Type]t.Invoke{},
		moduleMap:  map[tokens.ModuleName]tokens.ModuleName{},

		normalizedCustoms:    newRWMap[string, t.CustomResource](),
		normalizedComponents: newRWMap[string, t.ComponentResource](),
		normalizedInvokes:    newRWMap[string, t.Invoke](),
	}
}

// Normalize tokens via the module map.
func (d *Provider) normalize(tk tokens.Type) string {
	fix := func(tk tokens.Type) string {
		m := tk.Module().Name()
		if mod, ok := d.moduleMap[m]; ok {
			m = mod
		}
		return m.String() + tokens.TokenDelimiter + tk.Name().String()
	}
	d.normalizedComponents.Initialize(func(normalized map[string]t.ComponentResource) {
		for k, v := range d.components {
			normalized[fix(k)] = v
		}
	})
	d.normalizedCustoms.Initialize(func(normalized map[string]t.CustomResource) {
		for k, v := range d.customs {
			normalized[fix(k)] = v
		}
	})
	d.normalizedInvokes.Initialize(func(normalized map[string]t.Invoke) {
		for k, v := range d.invokes {
			normalized[fix(k)] = v
		}
	})
	return fix(tk)
}

// Add custom resources to be dispatched to.
func (d *Provider) WithCustomResources(resources map[tokens.Type]t.CustomResource) *Provider {
	d.normalizedCustoms.Reset()
	for k, v := range resources {
		d.customs[k] = v
	}
	return d
}

// Add component resources to be dispatched to.
func (d *Provider) WithComponentResources(components map[tokens.Type]t.ComponentResource) *Provider {
	d.normalizedComponents.Reset()
	for k, v := range components {
		d.components[k] = v
	}
	return d
}

// Add invokes to be dispatched to.
func (d *Provider) WithInvokes(invokes map[tokens.Type]t.Invoke) *Provider {
	d.normalizedInvokes.Reset()
	for k, v := range invokes {
		d.invokes[k] = v
	}
	return d
}

func (d *Provider) WithModuleMap(m map[tokens.ModuleName]tokens.ModuleName) *Provider {
	d.normalizedComponents.Reset()
	d.normalizedCustoms.Reset()
	d.normalizedInvokes.Reset()
	for k, v := range m {
		d.moduleMap[k] = v
	}
	return d
}

func (d *Provider) Invoke(ctx p.Context, req p.InvokeRequest) (p.InvokeResponse, error) {
	tk := d.normalize(req.Token)
	inv, ok := d.normalizedInvokes.Load(tk)
	if ok {
		return inv.Invoke(ctx, req)
	}
	r, err := d.Provider.Invoke(ctx, req)
	return r, d.fixupError(tk, err)
}

func (d *Provider) Check(ctx p.Context, req p.CheckRequest) (p.CheckResponse, error) {
	tk := d.normalize(req.Urn.Type())
	r, ok := d.normalizedCustoms.Load(tk)
	if ok {
		return r.Check(ctx, req)
	}
	c, err := d.Provider.Check(ctx, req)
	return c, d.fixupError(tk, err)
}

func (d *Provider) Diff(ctx p.Context, req p.DiffRequest) (p.DiffResponse, error) {
	tk := d.normalize(req.Urn.Type())
	r, ok := d.normalizedCustoms.Load(tk)
	if ok {
		return r.Diff(ctx, req)
	}
	diff, err := d.Provider.Diff(ctx, req)
	return diff, d.fixupError(tk, err)

}

func (d *Provider) Create(ctx p.Context, req p.CreateRequest) (p.CreateResponse, error) {
	tk := d.normalize(req.Urn.Type())
	r, ok := d.normalizedCustoms.Load(tk)
	if ok {
		return r.Create(ctx, req)
	}
	c, err := d.Provider.Create(ctx, req)
	return c, d.fixupError(tk, err)
}

func (d *Provider) Read(ctx p.Context, req p.ReadRequest) (p.ReadResponse, error) {
	tk := d.normalize(req.Urn.Type())
	r, ok := d.normalizedCustoms.Load(tk)
	if ok {
		return r.Read(ctx, req)
	}
	read, err := d.Provider.Read(ctx, req)
	return read, d.fixupError(tk, err)
}

func (d *Provider) Update(ctx p.Context, req p.UpdateRequest) (p.UpdateResponse, error) {
	tk := d.normalize(req.Urn.Type())
	r, ok := d.normalizedCustoms.Load(tk)
	if ok {
		return r.Update(ctx, req)
	}
	up, err := d.Provider.Update(ctx, req)
	return up, d.fixupError(tk, err)
}

func (d *Provider) Delete(ctx p.Context, req p.DeleteRequest) error {
	tk := d.normalize(req.Urn.Type())
	r, ok := d.normalizedCustoms.Load(tk)
	if ok {
		return r.Delete(ctx, req)
	}
	return d.fixupError(tk, d.Provider.Delete(ctx, req))
}

func (d *Provider) Construct(pctx p.Context, typ string, name string,
	ctx *pulumi.Context, inputs pprovider.ConstructInputs, opts pulumi.ResourceOption) (pulumi.ComponentResource, error) {
	tk := d.normalize(tokens.Type(typ))
	r, ok := d.normalizedComponents.Load(tk)
	if ok {
		return r.Construct(pctx, typ, name, ctx, inputs, opts)
	}
	con, err := d.Provider.Construct(pctx, typ, name, ctx, inputs, opts)
	return con, d.fixupError(typ, err)
}

func (d *Provider) fixupError(tk string, err error) error {
	if status.Code(err) == codes.Unimplemented {
		err = status.Errorf(codes.NotFound, "Type '%s' not found", tk)
	}
	return err
}
