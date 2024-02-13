package evaluation

import (
	"offchainlabs.com/cuckoo-cache"
	"offchainlabs.com/cuckoo-cache/cacheKeys"
	"offchainlabs.com/cuckoo-cache/onChainIndex"
	"offchainlabs.com/cuckoo-cache/onChainStorage"
)

func EvaluateOnData[CacheKey cacheKeys.LocalNodeCacheKey](
	onChainSize uint64,
	localSize uint64,
	accesses []CacheKey,
) (uint64, uint64) { // (onChainHits, localHits)
	onChain := onChainIndex.OpenOnChainCuckooTable(onChainStorage.NewMockOnChainStorage(), onChainSize)
	onChain.Initialize(onChainSize)
	cache := cuckoo_cache.NewLocalNodeCache[CacheKey](localSize, onChain, cuckoo_cache.NewMockBackingStore())

	onChainHits := uint64(0)
	localHits := uint64(0)
	for _, key := range accesses {
		if cuckoo_cache.IsInLocalNodeCache(cache, key) {
			localHits++
		}
		_, hit := cuckoo_cache.ReadItemFromLocalCache(cache, key)
		if hit {
			onChainHits++
		}
	}

	return onChainHits, localHits
}
