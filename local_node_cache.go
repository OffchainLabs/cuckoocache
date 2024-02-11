package generational_cache

type CacheItemValue uint64

type LocalNodeCache struct {
	onChain       *OnChainCuckooTable
	localCapacity uint64
	numInCache    uint64
	index         map[CacheItemKey]*LruNode
	lru           *LruNode
	mru           *LruNode
	backingStore  CacheBackingStore
}

type LruNode struct {
	itemKey    CacheItemKey
	itemValue  *CacheItemValue
	moreRecent *LruNode
	lessRecent *LruNode
}

// Create a new local node cache. If syncFromOnChain is true, the cache is warmed up by loading all items
// in the on-chain cache.
//
// Otherwise, this cold-starts the local cache. If the on-chain cache is not empty, then this
// local cache might eventually have up to <localCapacity> cache misses on items that are currently in the
// on-chain cache. Within two generation-shifts of the on-chain cache, this local cache will have established the
// subset property, i.e. that every object in the on-chain cache is in this cache.
// Once established, that property will persist forever.
func NewLocalNodeCache(
	localCapacity uint64,
	onChain *OnChainCuckooTable,
	backingStore CacheBackingStore,
	syncFromOnChain bool,
) *LocalNodeCache {
	header := onChain.readHeader()
	if localCapacity < header.capacity {
		// local node cache must be at least as big as the on-chain table's capacity
		// otherwise there might be repeated hits in the on-chain table that miss in this node cache
		localCapacity = header.capacity
	}
	cache := &LocalNodeCache{
		onChain:       onChain,
		localCapacity: localCapacity,
		numInCache:    0,
		index:         make(map[CacheItemKey]*LruNode),
		lru:           nil,
		mru:           nil,
		backingStore:  backingStore,
	}
	if syncFromOnChain {
		cache.SyncFromOnChain()
	}
	return cache
}

func (cache *LocalNodeCache) IsInCache(key CacheItemKey) bool {
	return cache.index[key] != nil
}

func (cache *LocalNodeCache) ReadItem(key CacheItemKey) *CacheItemValue {
	cache.onChain.AccessItem(key)
	return cache.readItemNoOnChainUpdate(key)
}

func (cache *LocalNodeCache) readItemNoOnChainUpdate(key CacheItemKey) *CacheItemValue {
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
		node = &LruNode{
			itemKey:    key,
			itemValue:  cache.backingStore.Read(key),
			moreRecent: nil,
			lessRecent: cache.mru,
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
	return node.itemValue
}

func (cache *LocalNodeCache) SyncFromOnChain() {
	newGeneration := ForAllOnChainCachedItems(
		cache.onChain,
		func(key CacheItemKey, inLatestGeneration bool, newGenSoFar []CacheItemKey) []CacheItemKey {
			if inLatestGeneration {
				return append(newGenSoFar, key)
			} else {
				_ = cache.readItemNoOnChainUpdate(key)
				return newGenSoFar
			}
		},
		[]CacheItemKey{},
	)
	for _, key := range newGeneration {
		_ = cache.readItemNoOnChainUpdate(key)
	}
}

func ForAllInLocalNodeCache[Accumulator any](
	cache *LocalNodeCache,
	f func(key CacheItemKey, value *CacheItemValue, t Accumulator) Accumulator,
	t Accumulator,
) Accumulator {
	tt := t
	for node := cache.mru; node != nil; node = node.lessRecent {
		tt = f(node.itemKey, node.itemValue, tt)
	}
	return tt
}
