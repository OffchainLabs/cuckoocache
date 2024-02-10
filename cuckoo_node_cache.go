package generational_cache

type CacheItemValue uint64

type NodeCacheCuckoo struct {
	onChain       *OnChainCuckoo
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

// This cold-starts a local LRU cache. If the on-chain cache is not empty, then this local cache
// might eventually have up to <localCapacity> cache misses on items that are in the on-chain cache.
// /Within two generation-shifts of the on-chain cache, this local cache will have established the
// subset property, i.e. that every object in the on-chain cache is in this cache.
// Once established, that property will persist forever.
func NewNodeCacheCuckoo(
	localCapacity uint64,
	onChain *OnChainCuckoo,
	backingStore CacheBackingStore,
) *NodeCacheCuckoo {
	if localCapacity < onChain.header.capacity {
		// local node cache must be at least as big as the onchain table's capacity
		// otherwise there might be repeated hits in the onchain table that miss in the node cache
		localCapacity = onChain.header.capacity
	}
	return &NodeCacheCuckoo{
		onChain:       onChain,
		localCapacity: localCapacity,
		numInCache:    0,
		index:         make(map[CacheItemKey]*LruNode),
		lru:           nil,
		mru:           nil,
		backingStore:  backingStore,
	}
}

func (cache *NodeCacheCuckoo) IsInCache(key CacheItemKey) bool {
	return cache.index[key] != nil
}

func (cache *NodeCacheCuckoo) ReadItem(key CacheItemKey) *CacheItemValue {
	cache.onChain.AccessItem(key)
	node := cache.index[key]
	if node == nil {
		if cache.numInCache == cache.localCapacity {
			// evict the least recently used item
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

func ForAllCachedItems[Accumulator any](
	cache *NodeCacheCuckoo,
	f func(key CacheItemKey, value *CacheItemValue, t Accumulator) Accumulator,
	t Accumulator,
) Accumulator {
	tt := t
	for node := cache.mru; node != nil; node = node.lessRecent {
		tt = f(node.itemKey, node.itemValue, tt)
	}
	return tt
}
