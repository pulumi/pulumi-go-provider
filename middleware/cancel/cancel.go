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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	p "github.com/pulumi/pulumi-go-provider"
)

func Wrap(provider p.Provider) p.Provider {
	var canceled bool
	var cancelFuncs inOutCache[context.CancelFunc]
	cancel := func(ctx p.Context, timeout float64) (p.Context, func()) {
		var cancel context.CancelFunc
		if timeout == noTimeout {
			ctx, cancel = p.CtxWithCancel(ctx)
		} else {
			ctx, cancel = p.CtxWithTimeout(ctx, time.Second*time.Duration(timeout))
		}
		if canceled {
			cancel()
			return ctx, func() {}
		}
		evict := cancelFuncs.insert(cancel)
		return ctx, func() {
			if !evict() {
				cancel()
			}
		}
	}
	new := provider
	new.Cancel = func(ctx p.Context) error {
		canceled = true
		for _, f := range cancelFuncs.drain() {
			f()
		}

		// We consider this a valid implementation of the Cancel RPC request. We still pass on
		// the request so downstream provides *may* rely on the Cancel call, but we catch an
		// Unimplemented error, making implementing the Cancel call optional for downstream
		// providers.
		var err error
		if provider.Cancel != nil {
			err = provider.Cancel(ctx)
			if status.Code(err) == codes.Unimplemented {
				return nil
			}
		}
		return err
	}
	if provider.GetSchema != nil {
		new.GetSchema = func(ctx p.Context, req p.GetSchemaRequest) (p.GetSchemaResponse, error) {
			ctx, end := cancel(ctx, noTimeout)
			defer end()
			return provider.GetSchema(ctx, req)
		}
	}
	if provider.CheckConfig != nil {
		new.CheckConfig = func(ctx p.Context, req p.CheckRequest) (p.CheckResponse, error) {
			ctx, end := cancel(ctx, noTimeout)
			defer end()
			return provider.CheckConfig(ctx, req)
		}
	}
	if provider.DiffConfig != nil {
		new.DiffConfig = func(ctx p.Context, req p.DiffRequest) (p.DiffResponse, error) {
			ctx, end := cancel(ctx, noTimeout)
			defer end()
			return provider.DiffConfig(ctx, req)
		}
	}
	if provider.Configure != nil {
		new.Configure = func(ctx p.Context, req p.ConfigureRequest) error {
			ctx, end := cancel(ctx, noTimeout)
			defer end()
			return provider.Configure(ctx, req)
		}
	}
	if provider.Invoke != nil {
		new.Invoke = func(ctx p.Context, req p.InvokeRequest) (p.InvokeResponse, error) {
			ctx, end := cancel(ctx, noTimeout)
			defer end()
			return provider.Invoke(ctx, req)
		}
	}
	if provider.Check != nil {
		new.Check = func(ctx p.Context, req p.CheckRequest) (p.CheckResponse, error) {
			ctx, end := cancel(ctx, noTimeout)
			defer end()
			return provider.Check(ctx, req)
		}
	}
	if provider.Diff != nil {
		new.Diff = func(ctx p.Context, req p.DiffRequest) (p.DiffResponse, error) {
			ctx, end := cancel(ctx, noTimeout)
			defer end()
			return provider.Diff(ctx, req)
		}
	}
	if provider.Create != nil {
		new.Create = func(ctx p.Context, req p.CreateRequest) (p.CreateResponse, error) {
			ctx, end := cancel(ctx, req.Timeout)
			defer end()
			return provider.Create(ctx, req)
		}
	}
	if provider.Read != nil {
		new.Read = func(ctx p.Context, req p.ReadRequest) (p.ReadResponse, error) {
			ctx, end := cancel(ctx, noTimeout)
			defer end()
			return provider.Read(ctx, req)
		}
	}
	if provider.Update != nil {
		new.Update = func(ctx p.Context, req p.UpdateRequest) (p.UpdateResponse, error) {
			ctx, end := cancel(ctx, req.Timeout)
			defer end()
			return provider.Update(ctx, req)
		}
	}
	if provider.Delete != nil {
		new.Delete = func(ctx p.Context, req p.DeleteRequest) error {
			ctx, end := cancel(ctx, req.Timeout)
			defer end()
			return provider.Delete(ctx, req)
		}
	}
	if provider.Construct != nil {
		new.Construct = func(pctx p.Context, typ string, name string,
			ctx *pulumi.Context, inputs pprovider.ConstructInputs, opts pulumi.ResourceOption,
		) (pulumi.ComponentResource, error) {
			pctx, end := cancel(pctx, noTimeout)
			defer end()
			return provider.Construct(pctx, typ, name, ctx, inputs, opts)
		}
	}
	return new
}

const noTimeout float64 = 0

// A data structure which provides amortized O(1) insertion, removal, and draining.
type inOutCache[T any] struct {
	values     []*entry[T] // An unorderd list of stored values or tombstone (nil) entries.
	tombstones []int       // An unordered list of empty slots in values
	m          sync.Mutex
	inDrain    bool // Wheither the cache is currently being drained. inDrain=true implies m can be ignored.
}

type entry[T any] struct {
	evict func() bool
	value T
}

// Insert a new element into the inOutCahce. The new element can be ejected by calling
// `evict`. If the element was already drained or if `evict` was already called, then
// `evict` will return true. Otherwise it returns false.
func (h *inOutCache[T]) insert(t T) (evict func() (missing bool)) {
	h.m.Lock()
	defer h.m.Unlock()
	var i int // The index in values of the new entry.
	if len(h.tombstones) == 0 {
		i = len(h.values) // We extend values.
	} else {
		// There is an empty slot in values, so use that.
		i = h.tombstones[len(h.tombstones)-1]
		h.tombstones = h.tombstones[:len(h.tombstones)-1]
	}

	el := &entry[T]{
		value: t,
	}
	el.evict = func() bool {
		if !h.inDrain {
			h.m.Lock()
			defer h.m.Unlock()
		}
		gone := el.evict == nil
		if gone {
			return true
		}
		el.evict = nil
		h.values[i] = nil
		h.tombstones = append(h.tombstones, i)
		return gone

	}

	// Push the value
	if len(h.tombstones) == 0 {
		h.values = append(h.values, el)
	} else {
		h.values[i] = el
	}
	return el.evict
}

// Remove all values from the inOutCache, and return them.
func (h *inOutCache[T]) drain() []T {
	h.m.Lock()
	defer h.m.Unlock()
	// Setting inDrain indicates a trusted actor holds the mutex, indicating that evict
	// functions don't need to grab the mutex before executing.
	h.inDrain = true
	defer func() { h.inDrain = false }()
	values := []T{} // Values currently in the cache.
	for _, v := range h.values {
		if v != nil {
			v.evict()
			values = append(values, v.value)
		}
	}
	return values
}
