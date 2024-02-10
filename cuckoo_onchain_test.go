package generational_cache

import (
	"encoding/binary"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestCuckooOnChain(t *testing.T) {
	capacity := uint64(32)
	cache := NewOnChainCuckoo(capacity)
	assert.Zero(t, cache.Len())
	assert.Equal(t, cache.IsInCache(keyFromUint64(0)), false) // verify that uninitialized table items aren't false positives
	assert.Equal(t, cache.IsInCache(keyFromUint64(31)), false)
	assert.Zero(t, cache.Len())
	verifyAccurateGenerationCounts(t, cache)

	// make cache almost full and verify items are in cache
	for i := uint64(0); i < capacity-2; i++ {
		cache.AccessItem(keyFromUint64(i))
		verifyAccurateGenerationCounts(t, cache)
	}
	for i := uint64(0); i < capacity-2; i++ {
		assert.Equal(t, cache.IsInCache(keyFromUint64(i)), true)
		verifyAccurateGenerationCounts(t, cache)
	}
	assert.Equal(t, cache.Len(), capacity-2)
	verifyAccurateGenerationCounts(t, cache)

	// add items beyond capacity and verify that something was evicted
	for i := capacity - 2; i < capacity+1; i++ {
		cache.AccessItem(keyFromUint64(i))
		verifyAccurateGenerationCounts(t, cache)
	}
	foundThemAll := true
	for i := uint64(0); i < capacity+1; i++ {
		if !cache.IsInCache(keyFromUint64(i)) {
			foundThemAll = false
		}
	}
	assert.Equal(t, foundThemAll, false)
	verifyAccurateGenerationCounts(t, cache)

	sprayOnChainCache(cache, 98113084)
	verifyAccurateGenerationCounts(t, cache)
	cache.AccessItem(keyFromUint64(58712))
	assert.Equal(t, cache.IsInCache(keyFromUint64(58712)), true)
}

func keyFromUint64(key uint64) CacheItemKey {
	h := crypto.Keccak256(binary.LittleEndian.AppendUint64([]byte{}, key))
	return common.BytesToAddress(h[0:20])
}

func sprayOnChainCache(cache *OnChainCuckoo, seed uint64) {
	modulus := 11 * cache.header.capacity / 7
	for i := uint64(seed); i < seed+cache.header.capacity; i++ {
		item := seed + (i % modulus)
		cache.AccessItem(keyFromUint64(item))
	}
}

func verifyAccurateGenerationCounts(t *testing.T, cache *OnChainCuckoo) {
	t.Helper()
	manualLastGenCount := ForAllOnChainCachedItems[uint64](
		cache,
		func(key CacheItemKey, inLatestGeneration bool, soFar uint64) uint64 {
			if inLatestGeneration {
				return soFar + 1
			} else {
				return soFar
			}
		},
		0,
	)
	assert.Equal(t, manualLastGenCount, cache.header.currentGenCount)
	manualBothGensCount := ForAllOnChainCachedItems[uint64](
		cache,
		func(key CacheItemKey, inLatestGeneration bool, soFar uint64) uint64 {
			return soFar + 1
		},
		0,
	)
	assert.Equal(t, manualBothGensCount, cache.header.inCacheCount)
}
