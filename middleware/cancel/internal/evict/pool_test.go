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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPool(t *testing.T) {
	t.Parallel()

	evicted := map[int]struct{}{}

	cache := Pool[int]{
		OnEvict: func(i int) {
			_, alreadyEvicted := evicted[i]
			assert.False(t, alreadyEvicted)
			evicted[i] = struct{}{}
		},
	}

	h1 := cache.Insert(1)
	cache.Insert(2)

	h1.Evict()
	assert.Contains(t, evicted, 1)

	cache.Insert(3)

	cache.Close()

	assert.Equal(t, map[int]struct{}{
		1: {},
		2: {},
		3: {},
	}, evicted)
}

func TestPoolParallel(t *testing.T) {
	t.Parallel()

	evicted := new(sync.Map)

	cache := Pool[int]{
		OnEvict: func(i int) {
			_, alreadyEvicted := evicted.LoadOrStore(i, struct{}{})
			assert.False(t, alreadyEvicted)
		},
	}

	var wait sync.WaitGroup
	wait.Add(5)

	for i := 1; i <= 5; i++ {
		go func(i int) {
			min, max := i*100, (i+1)*100
			localEvict := make(map[Handle[int]]struct{}, 50)
			for i := min; i < max; i++ {
				h := cache.Insert(i)
				if i%2 == 0 {
					localEvict[h] = struct{}{}
				}
			}

			for h := range localEvict {
				h.Evict()
			}

			for i := min; i < max; i += 2 {
				_, has := evicted.Load(i)
				assert.True(t, has)
			}

			wait.Done()
		}(i)
	}

	wait.Wait()

	lenSyncMap := func(m *sync.Map) int {
		i := 0
		m.Range(func(any, any) bool {
			i++
			return true
		})
		return i
	}

	assert.Equal(t, 250, lenSyncMap(evicted),
		"Check that the 250 evens are present, and no other numbers are.")

	cache.Close()

	assert.Equal(t, 500, lenSyncMap(evicted),
		"Check that all 500 numbers are now present")
}
