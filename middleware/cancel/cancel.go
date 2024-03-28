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
	wrapper := provider
	wrapper.Cancel = func(ctx p.Context) error {
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

	// Wrap each gRPC method to transform a cancel call into a cancel on
	// context.Cancel.
	wrapper.GetSchema = setCancel2(cancel, provider.GetSchema, nil)
	wrapper.CheckConfig = setCancel2(cancel, provider.CheckConfig, nil)
	wrapper.DiffConfig = setCancel2(cancel, provider.DiffConfig, nil)
	wrapper.Configure = setCancel1(cancel, provider.Configure, nil)
	wrapper.Invoke = setCancel2(cancel, provider.Invoke, nil)
	wrapper.Check = setCancel2(cancel, provider.Check, nil)
	wrapper.Diff = setCancel2(cancel, provider.Diff, nil)
	wrapper.Create = setCancel2(cancel, provider.Create, func(r p.CreateRequest) float64 {
		return r.Timeout
	})
	wrapper.Read = setCancel2(cancel, provider.Read, nil)
	wrapper.Update = setCancel2(cancel, provider.Update, func(r p.UpdateRequest) float64 {
		return r.Timeout
	})
	wrapper.Delete = setCancel1(cancel, provider.Delete, func(r p.DeleteRequest) float64 {
		return r.Timeout
	})
	wrapper.Construct = setCancel2(cancel, provider.Construct, nil)
	return wrapper
}

func setCancel1[
	Req any,
	F func(p.Context, Req) error,
	Cancel func(ctx p.Context, timeout float64) (p.Context, func()),
	GetTimeout func(Req) float64,
](cancel Cancel, f F, getTimeout GetTimeout) F {
	if f == nil {
		return nil
	}
	return func(ctx p.Context, req Req) error {
		var timeout float64
		if getTimeout != nil {
			timeout = getTimeout(req)
		}
		ctx, end := cancel(ctx, timeout)
		defer end()
		return f(ctx, req)
	}
}

func setCancel2[
	Req any, Resp any,
	F func(p.Context, Req) (Resp, error),
	Cancel func(ctx p.Context, timeout float64) (p.Context, func()),
	GetTimeout func(Req) float64,
](cancel Cancel, f F, getTimeout GetTimeout) F {
	if f == nil {
		return nil
	}
	return func(ctx p.Context, req Req) (Resp, error) {
		var timeout float64
		if getTimeout != nil {
			timeout = getTimeout(req)
		}

		ctx, end := cancel(ctx, timeout)
		defer end()
		return f(ctx, req)
	}
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
