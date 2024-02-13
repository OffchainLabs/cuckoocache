package cuckoo_cache

import (
	"offchainlabs.com/cuckoo-cache/cacheKeys"
	onChain2 "offchainlabs.com/cuckoo-cache/onChain"
	"offchainlabs.com/cuckoo-cache/storage"
)

func EvaluateOnData[CacheKey cacheKeys.LocalNodeCacheKey](
	onChainSize uint64,
	localSize uint64,
	accesses []CacheKey,
) (uint64, uint64) { // (onChainHits, localHits)
	onChain := onChain2.OpenOnChainCuckooTable(storage.NewMockOnChainStorage(), onChainSize)
	onChain.Initialize(onChainSize)
	cache := NewLocalNodeCache[CacheKey](localSize, onChain, NewMockBackingStore())

	onChainHits := uint64(0)
	localHits := uint64(0)
	for _, key := range accesses {
		if IsInLocalNodeCache(cache, key) {
			localHits++
		}
		_, hit := ReadItemFromLocalCache(cache, key)
		if hit {
			onChainHits++
		}
	}

	return onChainHits, localHits
}
