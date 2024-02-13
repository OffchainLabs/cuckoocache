package cuckoo_cache

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
	cache := NewLocalNodeCache[Uint64LocalCacheKey](capacity, onChain, backing)

	for key := uint64(0); key < capacity; key++ {
		_, hit := ReadItemFromLocalCache(cache, NewUint64LocalCacheKey(key))
		assert.Equal(t, hit, false)
		verifyCacheInvariants(t, cache)
	}
	verifyItemsAreInCache(t, cache, 0, capacity-1)

	_, hit := ReadItemFromLocalCache(cache, NewUint64LocalCacheKey(capacity))
	assert.Equal(t, hit, false)
	_, hit = ReadItemFromLocalCache(cache, NewUint64LocalCacheKey(capacity+1))
	assert.Equal(t, hit, false)
	assert.Equal(t, IsInLocalNodeCache(cache, NewUint64LocalCacheKey(0)), false)
	assert.Equal(t, IsInLocalNodeCache(cache, NewUint64LocalCacheKey(1)), false)
	verifyItemsAreInCache(t, cache, 2, capacity+1)
	verifyCacheInvariants(t, cache)

	_, hit = ReadItemFromLocalCache(cache, NewUint64LocalCacheKey(0))
	assert.Equal(t, hit, false)
	assert.Equal(t, IsInLocalNodeCache(cache, NewUint64LocalCacheKey(0)), true)
	assert.Equal(t, IsInLocalNodeCache(cache, NewUint64LocalCacheKey(1)), false)
	assert.Equal(t, IsInLocalNodeCache(cache, NewUint64LocalCacheKey(2)), false)
	verifyItemsAreInCache(t, cache, 3, capacity+1)
	verifyCacheInvariants(t, cache)

	sprayNodeCache(cache, 129581247)
	for i := uint64(0); i < capacity; i++ {
		_, _ = ReadItemFromLocalCache(cache, NewUint64LocalCacheKey(10000+i))
		verifyItemsAreInCache(t, cache, 10000, 10000+i)
		verifyCacheInvariants(t, cache)
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
	cache := NewLocalNodeCache[Uint64LocalCacheKey](nodeCapacity, onChain, backing)
	assert.Equal(t, subsetPropertyHolds(cache), true)
	verifyCacheInvariants(t, cache)

	// cold-start node cache with a warm on-chain cache, subset property shouldn't hold
	sprayOnChainCache(onChain, 0)
	cache = NewLocalNodeCache[Uint64LocalCacheKey](nodeCapacity, onChain, backing)
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
		_, _ = ReadItemFromLocalCache(cache, NewUint64LocalCacheKey(1000000+i))
		assert.Equal(t, subsetPropertyHolds(cache), true)
		verifyCacheInvariants(t, cache)
	}
}

func subsetPropertyHolds(cache *LocalNodeCache[Uint64LocalCacheKey]) bool {
	keysInLocal := ForAllInLocalNodeCache(
		cache,
		func(key Uint64LocalCacheKey, _ []byte, soFar map[CacheItemKey]struct{}) map[CacheItemKey]struct{} {
			soFar[key.ToCacheKey()] = struct{}{}
			return soFar
		},
		map[CacheItemKey]struct{}{},
	)
	return ForAllOnChainCachedItems(
		cache.onChain,
		func(key CacheItemKey, _ bool, soFar bool) bool {
			_, exists := keysInLocal[key]
			return soFar && exists
		},
		true,
	)
}

func numInCacheCorrect(cache *LocalNodeCache[Uint64LocalCacheKey]) bool {
	return ForAllInLocalNodeCache[Uint64LocalCacheKey, uint64](
		cache,
		func(_ Uint64LocalCacheKey, _ []byte, numSoFar uint64) uint64 {
			return numSoFar + 1
		},
		0,
	) == cache.numInCache
}

func sprayNodeCache(cache *LocalNodeCache[Uint64LocalCacheKey], seed uint64) {
	modulus := 11 * cache.localCapacity / 7
	for i := uint64(seed); i < seed+cache.localCapacity; i++ {
		item := seed + (i % modulus)
		_, _ = ReadItemFromLocalCache(cache, NewUint64LocalCacheKey(item))
	}
}

func verifyItemsAreInCache(t *testing.T, cache *LocalNodeCache[Uint64LocalCacheKey], first uint64, last uint64) {
	t.Helper()
	for i := first; i <= last; i++ {
		assert.Equal(t, IsInLocalNodeCache(cache, NewUint64LocalCacheKey(i)), true)
	}
}

func verifyAllCachedValuesCorrect[CacheKey LocalNodeCacheKey](cache *LocalNodeCache[CacheKey]) bool {
	return ForAllInLocalNodeCache[CacheKey, bool](
		cache,
		func(key CacheKey, value []byte, okSoFar bool) bool {
			return okSoFar && bytes.Equal(value, cache.backingStore.Read(key.ToCacheKey()))
		},
		true,
	)
}

func verifyCacheInvariants(t *testing.T, cache *LocalNodeCache[Uint64LocalCacheKey]) {
	t.Helper()
	assert.Equal(t, verifyAllCachedValuesCorrect(cache), true)
	assert.Equal(t, numInCacheCorrect(cache), true)
}
