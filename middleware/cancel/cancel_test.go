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

package cancel

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInOutCache(t *testing.T) {
	t.Parallel()
	cache := inOutCache[int]{}
	evicts := make([]func() bool, 1000)
	for i := 0; i < 1000; i++ {
		evicts[i] = cache.insert(i)
	}
	for i := 0; i < 1000; i += 2 {
		assert.False(t, evicts[i]())
	}

	for i := 1000; i < 1500; i++ {
		evicts = append(evicts, cache.insert(i))
	}

	assert.Len(t, cache.values, 1001)

	expected := make([]int, 0, 1000)
	for i := 1; i < 1000; i += 2 {
		expected = append(expected, i)
	}
	for i := 1000; i < 1500; i++ {
		expected = append(expected, i)
	}
	elements := cache.drain()
	assert.ElementsMatch(t, elements, expected)

	// Refill the cache
	for i := 0; i < 1000; i++ {
		cache.insert(i + 2000)
	}
	for _, f := range evicts {
		assert.True(t, f(), "cache element was evicted")
	}
}
