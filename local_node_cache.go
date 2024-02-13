package cuckoo_cache

import (
	"offchainlabs.com/cuckoo-cache/cacheBackingStore"
	"offchainlabs.com/cuckoo-cache/cacheKeys"
	"offchainlabs.com/cuckoo-cache/onChainIndex"
)

type CacheItemValue []byte

type LocalNodeCache[CacheKey cacheKeys.LocalNodeCacheKey] struct {
	onChain       *onChainIndex.OnChainCuckooTable
	localCapacity uint64
	numInCache    uint64
	index         map[CacheKey]*LruNode[CacheKey]
	lru           *LruNode[CacheKey]
	mru           *LruNode[CacheKey]
	backingStore  cacheBackingStore.CacheBackingStore
}

type LruNode[CacheKey cacheKeys.LocalNodeCacheKey] struct {
	itemKey    CacheKey
	itemValue  []byte
	moreRecent *LruNode[CacheKey]
	lessRecent *LruNode[CacheKey]
	generation uint64
}

// Create a new local node cache. If syncFromOnChain is true, the cache is warmed up by loading all items
// in the on-chain cache.
//
// Otherwise, this cold-starts the local cache. If the on-chain cache is not empty, then this
// local cache might eventually have up to <localCapacity> cache misses on items that are currently in the
// on-chain cache. Within two generation-shifts of the on-chain cache, this local cache will have established the
// subset property, i.e. that every object in the on-chain cache is in this cache.
// Once established, that property will persist forever.
func NewLocalNodeCache[CacheKey cacheKeys.LocalNodeCacheKey](
	localCapacity uint64,
	onChain *onChainIndex.OnChainCuckooTable,
	backingStore cacheBackingStore.CacheBackingStore,
) *LocalNodeCache[CacheKey] {
	header := onChain.ReadHeader()
	if localCapacity < header.Capacity {
		// local node cache must be at least as big as the on-chain table's capacity
		// otherwise there might be repeated hits in the on-chain table that miss in this node cache
		localCapacity = header.Capacity
	}
	cache := &LocalNodeCache[CacheKey]{
		onChain:       onChain,
		localCapacity: localCapacity,
		numInCache:    0,
		index:         make(map[CacheKey]*LruNode[CacheKey]),
		lru:           nil,
		mru:           nil,
		backingStore:  backingStore,
	}
	return cache
}

func IsInLocalNodeCache[CacheKey cacheKeys.LocalNodeCacheKey](cache *LocalNodeCache[CacheKey], key CacheKey) bool {
	return cache.index[key] != nil
}

func ReadItemFromLocalCache[CacheKey cacheKeys.LocalNodeCacheKey](
	cache *LocalNodeCache[CacheKey],
	key CacheKey,
) ([]byte, bool) { // (data, wasHitInCache)
	hitOnChain, generationAfterAccess := cache.onChain.AccessItem(key.ToCacheKey())

	node := cache.index[key]
	if node == nil {
		// item is not in cache, so bring it in as the MRU
		if cache.numInCache == cache.localCapacity {
			// cache is already full, so evict the least recently used item
			delete(cache.index, cache.lru.itemKey)
			cache.lru = cache.lru.moreRecent
			cache.lru.lessRecent = nil
			cache.numInCache -= 1
		}
		node = &LruNode[CacheKey]{
			itemKey:    key,
			itemValue:  cache.backingStore.Read(key.ToCacheKey()),
			moreRecent: nil,
			lessRecent: cache.mru,
			generation: generationAfterAccess,
		}
		if node.lessRecent != nil {
			node.lessRecent.moreRecent = node
		}
		cache.mru = node
		if cache.lru == nil {
			cache.lru = node
		}
		cache.index[key] = node
		cache.numInCache += 1
	} else {
		// item is already in the cache, so make it the MRU
		node.generation = generationAfterAccess
		if cache.mru != node {
			if cache.lru == node {
				cache.lru = node.moreRecent
			}
			if node.lessRecent != nil {
				node.lessRecent.moreRecent = node.moreRecent
			}
			if node.moreRecent != nil {
				node.moreRecent.lessRecent = node.lessRecent
			}
			node.moreRecent = nil
			node.lessRecent = cache.mru
			if node.lessRecent != nil {
				node.lessRecent.moreRecent = node
			}
			cache.mru = node
		}
	}
	return node.itemValue, hitOnChain
}

func ForAllInLocalNodeCache[CacheKey cacheKeys.LocalNodeCacheKey, Accumulator any](
	cache *LocalNodeCache[CacheKey],
	f func(key CacheKey, value []byte, t Accumulator) Accumulator,
	t Accumulator,
) Accumulator {
	tt := t
	for node := cache.mru; node != nil; node = node.lessRecent {
		tt = f(node.itemKey, node.itemValue, tt)
	}
	return tt
}
