// Copyright 2024, Offchain Labs, Inc.
// For license information, see https://github.com/OffchainLabs/nitro/blob/master/LICENSE

package evaluation

import (
	"github.com/offchainlabs/cuckoocache"
	"github.com/offchainlabs/cuckoocache/cacheBackingStore"
	"github.com/offchainlabs/cuckoocache/cacheKeys"
	"github.com/offchainlabs/cuckoocache/onChainIndex"
	"github.com/offchainlabs/cuckoocache/onChainStorage"
)

func EvaluateOnData[KeyType cacheKeys.LocalNodeCacheKey](
	onChainSize uint64,
	localSize uint64,
	accesses []KeyType,
) (uint64, uint64, uint64, uint64, error) { // (onChainHits, localHits, storageReads, storageWrites)
	storage := onChainStorage.NewMockOnChainStorage()
	onChain := onChainIndex.OpenOnChainCuckooTable(storage, onChainSize)
	if err := onChain.Initialize(onChainSize); err != nil {
		return 0, 0, 0, 0, err
	}
	cache, err := cuckoocache.NewLocalNodeCache[KeyType](localSize, onChain, cacheBackingStore.NewMockBackingStore[KeyType]())
	if err != nil {
		return 0, 0, 0, 0, err
	}

	onChainHits := uint64(0)
	localHits := uint64(0)
	storageReadsBefore, storageWritesBefore := storage.(*onChainStorage.MockOnChainStorage).GetAccessCounts()
	for _, key := range accesses {
		if cuckoocache.IsInLocalNodeCache(cache, key) {
			localHits++
		}
		_, hit, err := cuckoocache.ReadItemFromLocalCache(cache, key)
		if err != nil {
			return 0, 0, 0, 0, err
		}
		if hit {
			onChainHits++
		}
	}

	storageReads, storageWrites := storage.(*onChainStorage.MockOnChainStorage).GetAccessCounts()
	storageReads -= storageReadsBefore
	storageWrites -= storageWritesBefore

	return onChainHits, localHits, storageReads, storageWrites, nil
}
