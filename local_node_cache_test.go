package generational_cache

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestNodeCacheLRUProperties(t *testing.T) {
	capacity := uint64(32)
	onChain := OpenOnChainCuckooTable(NewMockOnChainStorage(), capacity)
	onChain.Initialize(capacity)
	backing := NewMockBackingStore()
	cache := NewLocalNodeCache(capacity, onChain, backing, false)

	for key := uint64(0); key < capacity; key++ {
		_ = cache.ReadItem(keyFromUint64(key))
		verifyCacheInvariants(t, cache)
	}
	verifyItemsAreInCache(t, cache, 0, capacity-1)

	cache2 := NewLocalNodeCache(capacity, onChain, backing, true)
	verifyItemsAreInCache(t, cache2, 0, capacity-1)
	verifyCacheInvariants(t, cache2)

	_ = cache.ReadItem(keyFromUint64(capacity))
	_ = cache.ReadItem(keyFromUint64(capacity + 1))
	assert.Equal(t, cache.IsInCache(keyFromUint64(0)), false)
	assert.Equal(t, cache.IsInCache(keyFromUint64(1)), false)
	verifyItemsAreInCache(t, cache, 2, capacity+1)
	verifyCacheInvariants(t, cache)

	cache2.SyncFromOnChain()
	verifyCacheInvariants(t, cache2)

	_ = cache.ReadItem(keyFromUint64(0))
	assert.Equal(t, cache.IsInCache(keyFromUint64(0)), true)
	assert.Equal(t, cache.IsInCache(keyFromUint64(1)), false)
	assert.Equal(t, cache.IsInCache(keyFromUint64(2)), false)
	verifyItemsAreInCache(t, cache, 3, capacity+1)
	verifyCacheInvariants(t, cache)

	sprayNodeCache(cache, 129581247)
	for i := uint64(0); i < capacity; i++ {
		_ = cache.ReadItem(keyFromUint64(10000 + i))
		verifyItemsAreInCache(t, cache, 10000, 10000+i)
		verifyCacheInvariants(t, cache)
		cache2.SyncFromOnChain()
		verifyCacheInvariants(t, cache2)
	}
}

func TestCacheSubsetProperty(t *testing.T) {
	onChainCapacity := uint64(32)
	nodeCapacity := 2*onChainCapacity + 17
	onChainStorage := NewMockOnChainStorage()
	onChain := OpenOnChainCuckooTable(onChainStorage, onChainCapacity)
	onChain.Initialize(onChainCapacity)
	backing := NewMockBackingStore()

	// if both caches are cold, subset property should hold
	cache := NewLocalNodeCache(nodeCapacity, onChain, backing, false)
	assert.Equal(t, subsetPropertyHolds(cache), true)
	verifyCacheInvariants(t, cache)

	// cold-start node cache with a warm on-chain cache, subset property shouldn't hold
	sprayOnChainCache(onChain, 0)
	cache = NewLocalNodeCache(nodeCapacity, onChain, backing, false)
	assert.Equal(t, subsetPropertyHolds(cache), false)
	verifyCacheInvariants(t, cache)

	// if on-chain cache advances two generations, subset property should hold
	startGen := cache.onChain.readHeader().currentGeneration
	for seed := onChainCapacity; cache.onChain.readHeader().currentGeneration < startGen+2; seed += nodeCapacity {
		sprayNodeCache(cache, seed)
		verifyCacheInvariants(t, cache)
	}
	assert.Equal(t, subsetPropertyHolds(cache), true)
	verifyCacheInvariants(t, cache)

	// if we exercise the cache, subset property should continue to hold
	for i := uint64(0); i < 2000; i++ {
		_ = cache.ReadItem(keyFromUint64(1000000 + i))
		assert.Equal(t, subsetPropertyHolds(cache), true)
		verifyCacheInvariants(t, cache)
		/*
			cache = NewLocalNodeCache(nodeCapacity, onChain, backing, true)
			assert.Equal(t, subsetPropertyHolds(cache), true)
			verifyCacheInvariants(t, cache)
		*/
	}
}

func subsetPropertyHolds(cache *LocalNodeCache) bool {
	return ForAllOnChainCachedItems[bool](
		cache.onChain,
		func(key CacheItemKey, _ bool, okSoFar bool) bool {
			return okSoFar && cache.IsInCache(key)
		},
		true,
	)
}

func numInCacheCorrect(cache *LocalNodeCache) bool {
	return ForAllInLocalNodeCache[uint64](
		cache,
		func(key CacheItemKey, _ []byte, numSoFar uint64) uint64 {
			return numSoFar + 1
		},
		0,
	) == cache.numInCache
}

func sprayNodeCache(cache *LocalNodeCache, seed uint64) {
	modulus := 11 * cache.localCapacity / 7
	for i := uint64(seed); i < seed+cache.localCapacity; i++ {
		item := seed + (i % modulus)
		_ = cache.ReadItem(keyFromUint64(item))
	}
}

func verifyItemsAreInCache(t *testing.T, cache *LocalNodeCache, first uint64, last uint64) {
	t.Helper()
	for i := first; i <= last; i++ {
		assert.Equal(t, cache.IsInCache(keyFromUint64(i)), true)
	}
}

func verifyAllCachedValuesCorrect(cache *LocalNodeCache) bool {
	return ForAllInLocalNodeCache[bool](
		cache,
		func(key CacheItemKey, value []byte, okSoFar bool) bool {
			return okSoFar && bytes.Equal(value, cache.backingStore.Read(key))
		},
		true,
	)
}

func verifyCacheInvariants(t *testing.T, cache *LocalNodeCache) {
	t.Helper()
	assert.Equal(t, verifyAllCachedValuesCorrect(cache), true)
	assert.Equal(t, numInCacheCorrect(cache), true)
}
