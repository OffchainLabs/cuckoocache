package onChainIndex

import (
	"encoding/binary"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
	"offchainlabs.com/cuckoo-cache/onChainStorage"
	"testing"
)

func TestCuckooOnChain(t *testing.T) {
	capacity := uint64(32)
	storage := onChainStorage.NewMockOnChainStorage()
	cache := OpenOnChainCuckooTable(storage, capacity)
	cache.Initialize(capacity)
	header := cache.ReadHeader()
	assert.Equal(t, cache.IsInCache(&header, keyFromUint64(0)), false) // verify that uninitialized table items aren't false positives
	assert.Equal(t, cache.IsInCache(&header, keyFromUint64(31)), false)
	verifyAccurateGenerationCounts(t, cache)

	// make cache almost full and verify items are in cache
	for i := uint64(0); i < capacity-2; i++ {
		cache.AccessItem(keyFromUint64(i))
		verifyAccurateGenerationCounts(t, cache)
		assert.Equal(t, countCachedItems(cache), i+1)
	}
	cache = OpenOnChainCuckooTable(storage, capacity)
	for i := uint64(0); i < capacity-2; i++ {
		header := cache.ReadHeader()
		assert.Equal(t, cache.IsInCache(&header, keyFromUint64(i)), true)
		verifyAccurateGenerationCounts(t, cache)
	}
	cache = OpenOnChainCuckooTable(storage, capacity)
	assert.Equal(t, cache.ReadHeader().InCacheCount, capacity-2)
	verifyAccurateGenerationCounts(t, cache)

	// add items beyond capacity and verify that something was evicted
	for i := capacity - 2; i < capacity+1; i++ {
		cache = OpenOnChainCuckooTable(storage, capacity)
		cache.AccessItem(keyFromUint64(i))
		verifyAccurateGenerationCounts(t, cache)
	}
	foundThemAll := true
	for i := uint64(0); i < capacity+1; i++ {
		cache = OpenOnChainCuckooTable(storage, capacity)
		header := cache.ReadHeader()
		if !cache.IsInCache(&header, keyFromUint64(i)) {
			foundThemAll = false
		}
	}
	assert.Equal(t, foundThemAll, false)
	verifyAccurateGenerationCounts(t, cache)

	cache = OpenOnChainCuckooTable(storage, capacity)
	sprayOnChainCache(cache, 98113084)
	cache = OpenOnChainCuckooTable(storage, capacity)
	verifyAccurateGenerationCounts(t, cache)
	cache.AccessItem(keyFromUint64(58712))
	cache = OpenOnChainCuckooTable(storage, capacity)
	header = cache.ReadHeader()
	assert.Equal(t, cache.IsInCache(&header, keyFromUint64(58712)), true)
}

func keyFromUint64(key uint64) CacheItemKey {
	h := crypto.Keccak256(binary.LittleEndian.AppendUint64([]byte{}, key))
	ret := [24]byte{}
	copy(ret[:], h[0:24])
	return ret
}

func sprayOnChainCache(cache *OnChainCuckooTable, seed uint64) {
	capacity := cache.ReadHeader().Capacity
	modulus := 11 * capacity / 7
	for i := uint64(seed); i < seed+capacity; i++ {
		item := seed + (i % modulus)
		cache.AccessItem(keyFromUint64(item))
	}
}

func countCachedItems(cache *OnChainCuckooTable) uint64 {
	return ForAllOnChainCachedItems(
		cache,
		func(_ CacheItemKey, _ bool, numSoFar uint64) uint64 {
			return numSoFar + 1
		},
		0,
	)
}

func verifyAccurateGenerationCounts(t *testing.T, cache *OnChainCuckooTable) {
	t.Helper()
	header := cache.ReadHeader()
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
	assert.Equal(t, manualLastGenCount, header.CurrentGenCount)
	manualBothGensCount := ForAllOnChainCachedItems[uint64](
		cache,
		func(key CacheItemKey, inLatestGeneration bool, soFar uint64) uint64 {
			return soFar + 1
		},
		0,
	)
	assert.Equal(t, manualBothGensCount, header.InCacheCount)
}
