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

// Cancel ensures that contexts are canceled when their associated tasks are completed.
// There are two parts of this middleware:
// 1. Tying Provider.Cancel to all associated contexts.
// 2. Applying timeout information when available.
package cancel

import (
	"context"
	"sync"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	pprovider "github.com/pulumi/pulumi/sdk/v3/go/pulumi/provider"

	p "github.com/pulumi/pulumi-go-provider"
)

func Wrap(provider p.Provider) p.Provider {
	return &cancelProvider{
		inner:       provider,
		cancelFuncs: inOutCache[context.CancelFunc]{},
	}
}

type cancelProvider struct {
	inner       p.Provider
	cancelFuncs inOutCache[context.CancelFunc]
	canceled    bool // If Cancel has been called on this provider
}

func (c *cancelProvider) Cancel(ctx p.Context) error {
	for _, f := range c.cancelFuncs.drain() {
		f()
	}
	return c.inner.Cancel(ctx)
}

const noTimeout float64 = 0

func (c *cancelProvider) cancel(ctx p.Context, timeout float64) (p.Context, func()) {
	var cancel context.CancelFunc
	if timeout == noTimeout {
		ctx, cancel = p.CtxWithCancel(ctx)
	} else {
		ctx, cancel = p.CtxWithTimeout(ctx, time.Second*time.Duration(timeout))
	}
	if c.canceled {
		cancel()
		return ctx, func() {}
	}
	evict := c.cancelFuncs.insert(cancel)
	return ctx, func() {
		if !evict() {
			cancel()
		}
	}
}

func (c *cancelProvider) GetSchema(ctx p.Context, req p.GetSchemaRequest) (p.GetSchemaResponse, error) {
	ctx, end := c.cancel(ctx, noTimeout)
	defer end()
	return c.inner.GetSchema(ctx, req)
}

func (c *cancelProvider) CheckConfig(ctx p.Context, req p.CheckRequest) (p.CheckResponse, error) {
	ctx, end := c.cancel(ctx, noTimeout)
	defer end()
	return c.inner.CheckConfig(ctx, req)
}

func (c *cancelProvider) DiffConfig(ctx p.Context, req p.DiffRequest) (p.DiffResponse, error) {
	ctx, end := c.cancel(ctx, noTimeout)
	defer end()
	return c.inner.DiffConfig(ctx, req)
}

func (c *cancelProvider) Configure(ctx p.Context, req p.ConfigureRequest) error {
	ctx, end := c.cancel(ctx, noTimeout)
	defer end()
	return c.inner.Configure(ctx, req)
}

func (c *cancelProvider) Invoke(ctx p.Context, req p.InvokeRequest) (p.InvokeResponse, error) {
	ctx, end := c.cancel(ctx, noTimeout)
	defer end()
	return c.inner.Invoke(ctx, req)
}

func (c *cancelProvider) Check(ctx p.Context, req p.CheckRequest) (p.CheckResponse, error) {
	ctx, end := c.cancel(ctx, noTimeout)
	defer end()
	return c.inner.Check(ctx, req)
}

func (c *cancelProvider) Diff(ctx p.Context, req p.DiffRequest) (p.DiffResponse, error) {
	ctx, end := c.cancel(ctx, noTimeout)
	defer end()
	return c.inner.Diff(ctx, req)
}

func (c *cancelProvider) Create(ctx p.Context, req p.CreateRequest) (p.CreateResponse, error) {
	ctx, end := c.cancel(ctx, req.Timeout)
	defer end()
	return c.inner.Create(ctx, req)
}

func (c *cancelProvider) Read(ctx p.Context, req p.ReadRequest) (p.ReadResponse, error) {
	ctx, end := c.cancel(ctx, noTimeout)
	defer end()
	return c.inner.Read(ctx, req)
}

func (c *cancelProvider) Update(ctx p.Context, req p.UpdateRequest) (p.UpdateResponse, error) {
	ctx, end := c.cancel(ctx, req.Timeout)
	defer end()
	return c.inner.Update(ctx, req)
}

func (c *cancelProvider) Delete(ctx p.Context, req p.DeleteRequest) error {
	ctx, end := c.cancel(ctx, req.Timeout)
	defer end()
	return c.inner.Delete(ctx, req)
}

func (c *cancelProvider) Construct(pctx p.Context, typ string, name string,
	ctx *pulumi.Context, inputs pprovider.ConstructInputs, opts pulumi.ResourceOption) (pulumi.ComponentResource, error) {
	pctx, end := c.cancel(pctx, noTimeout)
	defer end()
	return c.inner.Construct(pctx, typ, name, ctx, inputs, opts)
}

// A data structure which provides amortized O(1) insertion, removal, and draining.
type inOutCache[T any] struct {
	values     []*T  // An unorderd list of stored values
	tombstones []int // An unordered list of empty slots in values
	m          sync.Mutex
}

// Insert a new element into the inOutCahce. The new element can be ejected by calling
// `evict`. If the element was already drained or if `evict` was already called, then
// `evict` will return true. Otherwise it returns false.
func (h *inOutCache[T]) insert(t T) (evict func() (missing bool)) {
	h.m.Lock()
	defer h.m.Unlock()
	var i int
	if len(h.tombstones) == 0 {
		h.values = append(h.values, &t)
		i = len(h.values) - 1
	} else {
		i = h.tombstones[len(h.tombstones)-1]
		h.tombstones = h.tombstones[:len(h.tombstones)-1]
		h.values[i] = &t
	}
	return func() bool {
		h.m.Lock()
		defer h.m.Unlock()
		gone := h.values[i] == nil
		h.values[i] = nil
		h.tombstones = append(h.tombstones, i)
		return gone
	}
}

// Remove all values from the inOutCache.
func (h *inOutCache[T]) drain() []T {
	h.m.Lock()
	defer h.m.Unlock()
	values := []T{}
	// We reset tombstones.
	h.tombstones = h.tombstones[:0]
	for i, v := range h.values {
		if v != nil {
			values = append(values, *v)
			h.values[i] = nil
		}
		h.tombstones = append(h.tombstones, i)
	}
	return values
}
