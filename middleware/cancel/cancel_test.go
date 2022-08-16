package cancel

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInOutCache(t *testing.T) {
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
