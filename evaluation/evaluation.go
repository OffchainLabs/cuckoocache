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
) (uint64, uint64, uint64, uint64) { // (onChainHits, localHits, storageReads, storageWrites)
	storage := onChainStorage.NewMockOnChainStorage()
	onChain := onChainIndex.OpenOnChainCuckooTable(storage, onChainSize)
	onChain.Initialize(onChainSize)
	cache := cuckoo_cache.NewLocalNodeCache[KeyType](localSize, onChain, cacheBackingStore.NewMockBackingStore[KeyType]())

	onChainHits := uint64(0)
	localHits := uint64(0)
	storageReadsBefore, storageWritesBefore := storage.(*onChainStorage.MockOnChainStorage).GetAccessCounts()
	for _, key := range accesses {
		if cuckoo_cache.IsInLocalNodeCache(cache, key) {
			localHits++
		}
		_, hit := cuckoo_cache.ReadItemFromLocalCache(cache, key)
		if hit {
			onChainHits++
		}
	}

	storageReads, storageWrites := storage.(*onChainStorage.MockOnChainStorage).GetAccessCounts()
	storageReads -= storageReadsBefore
	storageWrites -= storageWritesBefore

	return onChainHits, localHits, storageReads, storageWrites
}
