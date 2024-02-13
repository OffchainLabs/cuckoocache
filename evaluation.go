package generational_cache

func EvaluateOnData[CacheKey LocalNodeCacheKey](
	onChainSize uint64,
	localSize uint64,
	accesses []CacheKey,
) (uint64, uint64) { // (onChainHits, localHits)
	onChain := OpenOnChainCuckooTable(NewMockOnChainStorage(), onChainSize)
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
