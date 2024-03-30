// Copyright 2024, Pulumi Corporation.
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

package evict

import (
	"sync"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// A data structure which provides amortized O(1) insertion, removal, and draining.
type Pool[T any] struct {
	entries []entry[T]

	// emptyEntries holds the indexes of empty cells in entries.
	emptyEntries []int

	m sync.Mutex

	closed bool

	OnEvict func(T)
}

type Handle[T any] struct {
	cache    *Pool[T]
	idx      int
	revision int
}

func (h Handle[T]) Evict() {
	h.cache.m.Lock()
	defer h.cache.m.Unlock()

	h.threadUnsafeEvict()
}

func (h Handle[T]) threadUnsafeEvict() {
	entry := &h.cache.entries[h.idx]
	if !entry.has(h.revision) {
		return
	}

	h.cache.OnEvict(entry.value)
	entry.markEmpty()
	h.cache.emptyEntries = append(h.cache.emptyEntries, h.idx)
}

type entry[T any] struct {
	revision int
	empty    bool
	value    T
}

func (e *entry[T]) markEmpty() {
	if e.empty {
		return
	}
	e.revision++
	e.empty = true
}

func (e *entry[T]) has(revision int) bool {
	return !e.empty && e.revision == revision
}

// Insert a new element into the Pool. The new element can be ejected by calling
// `evict`. If the element was already drained or if `evict` was already called, then
// `evict` will return true. Otherwise it returns false.
func (c *Pool[T]) Insert(t T) (ret Handle[T]) {
	c.m.Lock()
	defer c.m.Unlock()

	// If we are finished, immediately evict the returned handle.
	if c.closed {
		defer func() { ret.threadUnsafeEvict() }()
	}

	// Check if an existing cell is empty
	if l := len(c.emptyEntries); l > 0 {
		// Pop the last empty entry
		i := c.emptyEntries[l-1]
		c.emptyEntries = c.emptyEntries[:l-1]
		entry := &c.entries[i]

		contract.Assertf(entry.empty, "emptyEntries must contain only evicted slots")

		// Set the entry to hold t
		entry.value = t
		entry.empty = false

		// Return a handle to the new entry
		return Handle[T]{
			cache:    c,
			idx:      i,
			revision: entry.revision,
		}
	}

	// No existing cells are empty, so create a new cell
	i := len(c.entries)
	c.entries = append(c.entries, entry[T]{
		value: t,
	})
	return Handle[T]{cache: c, idx: i}

}

// Close the cache, evicting all elements and evicting future elements on insertion.
func (c *Pool[T]) Close() {
	c.m.Lock()
	defer c.m.Unlock()

	c.closed = true

	for i, e := range c.entries {
		// Construct a handle to the maybe-empty slot in entries.
		handle := Handle[T]{
			cache:    c,
			idx:      i,
			revision: e.revision,
		}

		handle.threadUnsafeEvict()
	}
}
