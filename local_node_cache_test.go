// Copyright 2024, Offchain Labs, Inc.
// For license information, see https://github.com/OffchainLabs/nitro/blob/master/LICENSE

package cuckoocache

import (
	"bytes"
	"encoding/binary"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/offchainlabs/cuckoocache/cacheBackingStore"
	"github.com/offchainlabs/cuckoocache/cacheKeys"
	"github.com/offchainlabs/cuckoocache/onChainIndex"
	"github.com/offchainlabs/cuckoocache/onChainStorage"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestNodeCacheLRUProperties(t *testing.T) {
	capacity := uint64(32)
	onChain := onChainIndex.OpenOnChainCuckooTable(onChainStorage.NewMockOnChainStorage(), capacity)
	assert.Nil(t, onChain.Initialize(capacity))
	backing := cacheBackingStore.NewMockBackingStore[cacheKeys.Uint64LocalCacheKey]()
	cache, err := NewLocalNodeCache[cacheKeys.Uint64LocalCacheKey](capacity, onChain, backing)
	assert.Nil(t, err)

	for key := uint64(0); key < capacity; key++ {
		_, hit, err := ReadItemFromLocalCache(cache, cacheKeys.NewUint64LocalCacheKey(key))
		assert.Nil(t, err)
		assert.Equal(t, hit, false)
		verifyCacheInvariants(t, cache)
	}
	verifyItemsAreInCache(t, cache, 0, capacity-1)

	_, hit, err := ReadItemFromLocalCache(cache, cacheKeys.NewUint64LocalCacheKey(capacity))
	assert.Nil(t, err)
	assert.Equal(t, hit, false)
	_, hit, err = ReadItemFromLocalCache(cache, cacheKeys.NewUint64LocalCacheKey(capacity+1))
	assert.Nil(t, err)
	assert.Equal(t, hit, false)
	assert.Equal(t, IsInLocalNodeCache(cache, cacheKeys.NewUint64LocalCacheKey(0)), false)
	assert.Equal(t, IsInLocalNodeCache(cache, cacheKeys.NewUint64LocalCacheKey(1)), false)
	verifyItemsAreInCache(t, cache, 2, capacity+1)
	verifyCacheInvariants(t, cache)

	_, hit, err = ReadItemFromLocalCache(cache, cacheKeys.NewUint64LocalCacheKey(0))
	assert.Nil(t, err)
	assert.Equal(t, hit, false)
	assert.Equal(t, IsInLocalNodeCache(cache, cacheKeys.NewUint64LocalCacheKey(0)), true)
	assert.Equal(t, IsInLocalNodeCache(cache, cacheKeys.NewUint64LocalCacheKey(1)), false)
	assert.Equal(t, IsInLocalNodeCache(cache, cacheKeys.NewUint64LocalCacheKey(2)), false)
	verifyItemsAreInCache(t, cache, 3, capacity+1)
	verifyCacheInvariants(t, cache)

	sprayNodeCache(t, cache, 129581247)
	for i := uint64(0); i < capacity; i++ {
		_, _, err = ReadItemFromLocalCache(cache, cacheKeys.NewUint64LocalCacheKey(10000+i))
		assert.Nil(t, err)
		verifyItemsAreInCache(t, cache, 10000, 10000+i)
		verifyCacheInvariants(t, cache)
	}
}

func TestCacheSubsetProperty(t *testing.T) {
	onChainCapacity := uint64(32)
	nodeCapacity := 2*onChainCapacity + 17
	onChainSto := onChainStorage.NewMockOnChainStorage()
	onChain := onChainIndex.OpenOnChainCuckooTable(onChainSto, onChainCapacity)
	assert.Nil(t, onChain.Initialize(onChainCapacity))
	backing := cacheBackingStore.NewMockBackingStore[cacheKeys.Uint64LocalCacheKey]()

	// if both caches are cold, subset property should hold
	cache, err := NewLocalNodeCache[cacheKeys.Uint64LocalCacheKey](nodeCapacity, onChain, backing)
	assert.Nil(t, err)
	assert.Equal(t, subsetPropertyHolds(t, cache), true)
	verifyCacheInvariants(t, cache)

	// cold-start node cache with a warm on-chain cache, subset property shouldn't hold
	sprayOnChainCache(t, onChain, 0)
	cache, err = NewLocalNodeCache[cacheKeys.Uint64LocalCacheKey](nodeCapacity, onChain, backing)
	assert.Nil(t, err)
	assert.Equal(t, subsetPropertyHolds(t, cache), false)
	verifyCacheInvariants(t, cache)

	// if on-chain cache advances two generations, subset property should hold
	header, err := cache.onChain.ReadHeader()
	assert.Nil(t, err)
	startGen := header.CurrentGeneration
	for seed := onChainCapacity; readHeader(t, cache.onChain).CurrentGeneration < startGen+2; seed += nodeCapacity {
		sprayNodeCache(t, cache, seed)
		verifyCacheInvariants(t, cache)
	}
	assert.Equal(t, subsetPropertyHolds(t, cache), true)
	verifyCacheInvariants(t, cache)

	// if we exercise the cache, subset property should continue to hold
	for i := uint64(0); i < 2000; i++ {
		_, _, err = ReadItemFromLocalCache(cache, cacheKeys.NewUint64LocalCacheKey(1000000+i))
		assert.Nil(t, err)
		assert.Equal(t, subsetPropertyHolds(t, cache), true)
		verifyCacheInvariants(t, cache)
	}
}

func readHeader(t *testing.T, onChain *onChainIndex.OnChainCuckooTable) onChainIndex.OnChainCuckooHeader {
	t.Helper()
	header, err := onChain.ReadHeader()
	assert.Nil(t, err)
	return header
}

func TestCacheFlush(t *testing.T) {
	onChainCapacity := uint64(32)
	nodeCapacity := 2*onChainCapacity + 17
	storage := onChainStorage.NewMockOnChainStorage()
	onChain := onChainIndex.OpenOnChainCuckooTable(storage, onChainCapacity)
	assert.Nil(t, onChain.Initialize(onChainCapacity))
	backing := cacheBackingStore.NewMockBackingStore[cacheKeys.Uint64LocalCacheKey]()
	cache, err := NewLocalNodeCache[cacheKeys.Uint64LocalCacheKey](nodeCapacity, onChain, backing)
	assert.Nil(t, err)

	sprayNodeCache(t, cache, 0)
	assert.Greater(t, cache.numInCache, uint64(0))
	key42 := cacheKeys.NewUint64LocalCacheKey(42)
	_, _, err = ReadItemFromLocalCache(cache, key42)
	assert.Nil(t, err)
	assert.Equal(t, IsInLocalNodeCache(cache, key42), true)

	assert.Nil(t, FlushOneItemFromLocalNodeCache(cache, key42, false))
	assert.Equal(t, IsInLocalNodeCache(cache, key42), false)
	header, err := cache.onChain.ReadHeader()
	assert.Nil(t, err)
	in, err := cache.onChain.IsInCache(&header, key42.ToCacheKey())
	assert.Nil(t, err)
	assert.Equal(t, in, true)

	sprayNodeCache(t, cache, 0)
	_, _, err = ReadItemFromLocalCache(cache, key42)
	assert.Nil(t, err)
	assert.Nil(t, FlushOneItemFromLocalNodeCache(cache, key42, true))
	assert.Equal(t, IsInLocalNodeCache(cache, key42), false)
	header, err = cache.onChain.ReadHeader()
	assert.Nil(t, err)
	in, err = cache.onChain.IsInCache(&header, key42.ToCacheKey())
	assert.Nil(t, err)
	assert.Equal(t, in, false)

	sprayNodeCache(t, cache, 0)
	assert.Nil(t, FlushLocalNodeCache(cache, false))
	assert.Equal(t, cache.numInCache, uint64(0))
	header, err = cache.onChain.ReadHeader()
	assert.Nil(t, err)
	assert.Greater(t, header.InCacheCount, uint64(0))

	sprayNodeCache(t, cache, 0)
	assert.Nil(t, FlushLocalNodeCache(cache, true))
	assert.Equal(t, cache.numInCache, uint64(0))
	header, err = cache.onChain.ReadHeader()
	assert.Nil(t, err)
	assert.Equal(t, header.InCacheCount, uint64(0))
}

func subsetPropertyHolds(t *testing.T, cache *LocalNodeCache[cacheKeys.Uint64LocalCacheKey]) bool {
	t.Helper()
	keysInLocal := ForAllInLocalNodeCache(
		cache,
		func(key cacheKeys.Uint64LocalCacheKey, _ []byte, soFar map[onChainIndex.CacheItemKey]struct{}) map[onChainIndex.CacheItemKey]struct{} {
			soFar[key.ToCacheKey()] = struct{}{}
			return soFar
		},
		map[onChainIndex.CacheItemKey]struct{}{},
	)
	result, err := onChainIndex.ForAllOnChainCachedItems(
		cache.onChain,
		func(key onChainIndex.CacheItemKey, _ bool, soFar bool) (bool, error) {
			_, exists := keysInLocal[key]
			return soFar && exists, nil
		},
		true,
	)
	assert.Nil(t, err)
	return result
}

func numInCacheCorrect(cache *LocalNodeCache[cacheKeys.Uint64LocalCacheKey]) bool {
	return ForAllInLocalNodeCache[cacheKeys.Uint64LocalCacheKey, uint64](
		cache,
		func(_ cacheKeys.Uint64LocalCacheKey, _ []byte, numSoFar uint64) uint64 {
			return numSoFar + 1
		},
		0,
	) == cache.numInCache
}

func sprayNodeCache(t *testing.T, cache *LocalNodeCache[cacheKeys.Uint64LocalCacheKey], seed uint64) {
	t.Helper()
	modulus := 11 * cache.localCapacity / 7
	for i := uint64(seed); i < seed+cache.localCapacity; i++ {
		item := seed + (i % modulus)
		_, _, err := ReadItemFromLocalCache(cache, cacheKeys.NewUint64LocalCacheKey(item))
		assert.Nil(t, err)
	}
}

func verifyItemsAreInCache(t *testing.T, cache *LocalNodeCache[cacheKeys.Uint64LocalCacheKey], first uint64, last uint64) {
	t.Helper()
	for i := first; i <= last; i++ {
		assert.Equal(t, IsInLocalNodeCache(cache, cacheKeys.NewUint64LocalCacheKey(i)), true)
	}
}

func verifyAllCachedValuesCorrect[KeyType cacheKeys.LocalNodeCacheKey](cache *LocalNodeCache[KeyType]) bool {
	return ForAllInLocalNodeCache[KeyType, bool](
		cache,
		func(key KeyType, value []byte, okSoFar bool) bool {
			return okSoFar && bytes.Equal(value, cache.backingStore.Read(key))
		},
		true,
	)
}

func verifyCacheInvariants(t *testing.T, cache *LocalNodeCache[cacheKeys.Uint64LocalCacheKey]) {
	t.Helper()
	assert.Equal(t, verifyAllCachedValuesCorrect(cache), true)
	assert.Equal(t, numInCacheCorrect(cache), true)
}

func sprayOnChainCache(t *testing.T, cache *onChainIndex.OnChainCuckooTable, seed uint64) {
	t.Helper()
	header, err := cache.ReadHeader()
	assert.Nil(t, err)
	capacity := header.Capacity
	modulus := 11 * capacity / 7
	for i := uint64(seed); i < seed+capacity; i++ {
		item := seed + (i % modulus)
		_, _, err = cache.AccessItem(keyFromUint64(item))
		assert.Nil(t, err)
	}
}

func keyFromUint64(key uint64) onChainIndex.CacheItemKey {
	h := crypto.Keccak256(binary.LittleEndian.AppendUint64([]byte{}, key))
	ret := [24]byte{}
	copy(ret[:], h[0:24])
	return ret
}
