// Copyright 2024, Offchain Labs, Inc.
// For license information, see https://github.com/OffchainLabs/nitro/blob/master/LICENSE

package onChainIndex

import (
	"encoding/binary"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/offchainlabs/cuckoocache/onChainStorage"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestCuckooOnChain(t *testing.T) {
	capacity := uint64(32)
	storage := onChainStorage.NewMockOnChainStorage()
	cache := OpenOnChainCuckooTable(storage, capacity)
	assert.Nil(t, cache.Initialize(capacity))
	header, err := cache.ReadHeader()
	assert.Nil(t, err)
	in, err := cache.IsInCache(&header, keyFromUint64(0))
	assert.Nil(t, err)
	assert.Equal(t, in, false) // verify that uninitialized table items aren't false positives
	in, err = cache.IsInCache(&header, keyFromUint64(31))
	assert.Nil(t, err)
	assert.Equal(t, in, false)
	verifyAccurateGenerationCounts(t, cache)

	// make cache almost full and verify items are in cache
	for i := uint64(0); i < capacity-2; i++ {
		_, _, err = cache.AccessItem(keyFromUint64(i))
		assert.Nil(t, err)
		verifyAccurateGenerationCounts(t, cache)
		count, err := countCachedItems(cache)
		assert.Nil(t, err)
		assert.Equal(t, count, i+1)
	}
	cache = OpenOnChainCuckooTable(storage, capacity)
	for i := uint64(0); i < capacity-2; i++ {
		header, err := cache.ReadHeader()
		assert.Nil(t, err)
		in, err = cache.IsInCache(&header, keyFromUint64(i))
		assert.Nil(t, err)
		assert.Equal(t, in, true)
		verifyAccurateGenerationCounts(t, cache)
	}
	cache = OpenOnChainCuckooTable(storage, capacity)
	header, err = cache.ReadHeader()
	assert.Nil(t, err)
	assert.Equal(t, header.InCacheCount, capacity-2)
	verifyAccurateGenerationCounts(t, cache)

	// add items beyond capacity and verify that something was evicted
	for i := capacity - 2; i < capacity+1; i++ {
		cache = OpenOnChainCuckooTable(storage, capacity)
		_, _, err = cache.AccessItem(keyFromUint64(i))
		assert.Nil(t, err)
		verifyAccurateGenerationCounts(t, cache)
	}
	foundThemAll := true
	for i := uint64(0); i < capacity+1; i++ {
		cache = OpenOnChainCuckooTable(storage, capacity)
		header, err := cache.ReadHeader()
		assert.Nil(t, err)
		in, err = cache.IsInCache(&header, keyFromUint64(i))
		assert.Nil(t, err)
		if !in {
			foundThemAll = false
		}
	}
	assert.Equal(t, foundThemAll, false)
	verifyAccurateGenerationCounts(t, cache)

	cache = OpenOnChainCuckooTable(storage, capacity)
	assert.Nil(t, sprayOnChainCache(cache, 98113084))
	cache = OpenOnChainCuckooTable(storage, capacity)
	verifyAccurateGenerationCounts(t, cache)
	_, _, err = cache.AccessItem(keyFromUint64(58712))
	assert.Nil(t, err)
	cache = OpenOnChainCuckooTable(storage, capacity)
	header, err = cache.ReadHeader()
	assert.Nil(t, err)
	in, err = cache.IsInCache(&header, keyFromUint64(58712))
	assert.Nil(t, err)
	assert.Equal(t, in, true)
}

func TestOnChainFlush(t *testing.T) {
	capacity := uint64(32)
	storage := onChainStorage.NewMockOnChainStorage()
	cache := OpenOnChainCuckooTable(storage, capacity)
	assert.Nil(t, cache.Initialize(capacity))

	assert.Nil(t, sprayOnChainCache(cache, 98113084))
	_, _, err := cache.AccessItem(keyFromUint64(42))
	assert.Nil(t, err)
	header, err := cache.ReadHeader()
	assert.Nil(t, err)
	in, err := cache.IsInCache(&header, keyFromUint64(42))
	assert.Nil(t, err)
	assert.Equal(t, in, true)
	assert.Nil(t, cache.FlushOneItem(keyFromUint64(42)))
	in, err = cache.IsInCache(&header, keyFromUint64(42))
	assert.Nil(t, err)
	assert.Equal(t, in, false)

	_, _, err = cache.AccessItem(keyFromUint64(42))
	assert.Nil(t, err)
	assert.Nil(t, cache.FlushAll())
	header, err = cache.ReadHeader()
	assert.Nil(t, err)
	in, err = cache.IsInCache(&header, keyFromUint64(42))
	assert.Nil(t, err)
	assert.Equal(t, in, false)
	header, err = cache.ReadHeader()
	assert.Nil(t, err)
	assert.Equal(t, header.InCacheCount, uint64(0))
}

func keyFromUint64(key uint64) CacheItemKey {
	h := crypto.Keccak256(binary.LittleEndian.AppendUint64([]byte{}, key))
	ret := [24]byte{}
	copy(ret[:], h[0:24])
	return ret
}

func sprayOnChainCache(cache *OnChainCuckooTable, seed uint64) error {
	header, err := cache.ReadHeader()
	if err != nil {
		return err
	}
	capacity := header.Capacity
	modulus := 11 * capacity / 7
	for i := uint64(seed); i < seed+capacity; i++ {
		item := seed + (i % modulus)
		if _, _, err = cache.AccessItem(keyFromUint64(item)); err != nil {
			return err
		}
	}
	return nil
}

func countCachedItems(cache *OnChainCuckooTable) (uint64, error) {
	return ForAllOnChainCachedItems(
		cache,
		func(_ CacheItemKey, _ bool, numSoFar uint64) (uint64, error) {
			return numSoFar + 1, nil
		},
		0,
	)
}

func verifyAccurateGenerationCounts(t *testing.T, cache *OnChainCuckooTable) {
	t.Helper()
	header, err := cache.ReadHeader()
	assert.Nil(t, err)
	manualLastGenCount, err := ForAllOnChainCachedItems[uint64](
		cache,
		func(key CacheItemKey, inLatestGeneration bool, soFar uint64) (uint64, error) {
			if inLatestGeneration {
				return soFar + 1, nil
			} else {
				return soFar, nil
			}
		},
		0,
	)
	assert.Nil(t, err)
	assert.Equal(t, manualLastGenCount, header.CurrentGenCount)
	manualBothGensCount, err := ForAllOnChainCachedItems[uint64](
		cache,
		func(key CacheItemKey, inLatestGeneration bool, soFar uint64) (uint64, error) {
			return soFar + 1, nil
		},
		0,
	)
	assert.Nil(t, err)
	assert.Equal(t, manualBothGensCount, header.InCacheCount)
}
