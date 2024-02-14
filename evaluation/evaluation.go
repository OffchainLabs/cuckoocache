package evaluation

import (
	"offchainlabs.com/cuckoo-cache"
	"offchainlabs.com/cuckoo-cache/cacheBackingStore"
	"offchainlabs.com/cuckoo-cache/cacheKeys"
	"offchainlabs.com/cuckoo-cache/onChainIndex"
	"offchainlabs.com/cuckoo-cache/onChainStorage"
)

func EvaluateOnData[CacheKey cacheKeys.LocalNodeCacheKey](
	onChainSize uint64,
	localSize uint64,
	accesses []CacheKey,
) (uint64, uint64, uint64, uint64) { // (onChainHits, localHits, storageReads, storageWrites)
	storage := onChainStorage.NewMockOnChainStorage()
	onChain := onChainIndex.OpenOnChainCuckooTable(storage, onChainSize)
	onChain.Initialize(onChainSize)
	cache := cuckoo_cache.NewLocalNodeCache[CacheKey](localSize, onChain, cacheBackingStore.NewMockBackingStore())

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
