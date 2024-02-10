package generational_cache

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

const LogMaxCacheSize = 16
const MaxCacheSize = 1 << LogMaxCacheSize

type CacheItemKey = common.Address

func CacheItemToBytes(item CacheItemKey) []byte {
	return item.Bytes()
}

const NumLanes = 8

type OnChainCuckooHeader struct {
	capacity          uint64
	currentGeneration uint64
	currentGenCount   uint64
	inCacheCount      uint64
}

type OnChainCuckoo struct {
	header OnChainCuckooHeader
	table  [][NumLanes]CuckooItem
}

type CuckooItem struct {
	itemKey    CacheItemKey
	generation uint64
}

func NewOnChainCuckoo(capacity uint64) *OnChainCuckoo {
	if capacity > MaxCacheSize {
		return nil
	}
	return &OnChainCuckoo{
		header: OnChainCuckooHeader{
			capacity:          capacity,
			currentGeneration: 2, // so that uninitialized CuckooItems don't look like they're in cache
		},
		table: make([][NumLanes]CuckooItem, capacity),
	}
}

func (occ *OnChainCuckoo) IsInCache(itemKey CacheItemKey) bool {
	itemKeyHash := crypto.Keccak256Hash(CacheItemToBytes(itemKey))
	for lane := uint64(0); lane < NumLanes; lane++ {
		slot := occ.getSlotForLane(itemKeyHash, lane)
		if occ.table[slot][lane].itemKey == itemKey && occ.table[slot][lane].generation != 0 {
			return occ.table[slot][lane].generation+1 >= occ.header.currentGeneration
		}
	}
	return false
}

func (occ *OnChainCuckoo) Len() uint64 {
	return occ.header.inCacheCount
}

func (occ *OnChainCuckoo) AccessItem(itemKey CacheItemKey) bool {
	itemKeyHash := crypto.Keccak256Hash(CacheItemToBytes(itemKey))
	expiredItemFoundInLane := uint64(NumLanes) // NumLanes means that no expired item has been found yet
	for lane := uint64(0); lane < NumLanes; lane++ {
		slot := occ.getSlotForLane(itemKeyHash, lane)
		if occ.table[slot][lane].itemKey == itemKey {
			cachedGeneration := occ.table[slot][lane].generation
			if cachedGeneration == occ.header.currentGeneration {
				return true
			} else if cachedGeneration+1 == occ.header.currentGeneration {
				occ.table[slot][lane].generation = occ.header.currentGeneration
				occ.header.currentGenCount += 1
				return true
			} else {
				// the item is in the table but is expired
				occ.advanceGenerationIfNeeded()
				occ.table[slot][lane].generation = occ.header.currentGeneration
				occ.header.currentGenCount += 1
				occ.header.inCacheCount += 1
				return false
			}
		} else if occ.table[slot][lane].generation+1 < occ.header.currentGeneration {
			expiredItemFoundInLane = lane
		}
	}
	occ.advanceGenerationIfNeeded()
	if expiredItemFoundInLane < NumLanes {
		// didn't find the item in the table, so replace an expired item
		slot := occ.getSlotForLane(itemKeyHash, expiredItemFoundInLane)
		occ.table[slot][expiredItemFoundInLane] = CuckooItem{itemKey, occ.header.currentGeneration}
		occ.header.currentGenCount += 1
		occ.header.inCacheCount += 1
	} else {
		slot := occ.getSlotForLane(itemKeyHash, 0)
		itemKeyToRelocate := occ.table[slot][0]
		occ.table[slot][0] = CuckooItem{itemKey: itemKey, generation: occ.header.currentGeneration}
		occ.header.currentGenCount += 1
		occ.header.inCacheCount += 1
		occ.relocateItem(itemKeyToRelocate, 1)
	}
	return false
}

func (occ *OnChainCuckoo) advanceGenerationIfNeeded() {
	for occ.header.inCacheCount >= occ.header.capacity || occ.header.currentGenCount > 4*occ.header.capacity/5 {
		occ.header.currentGeneration += 1
		occ.header.inCacheCount = occ.header.currentGenCount
		occ.header.currentGenCount = 0
	}
}

const SliceSizeBytes = (LogMaxCacheSize + 7) / 8

func (occ *OnChainCuckoo) getSlotForLane(itemKeyHash common.Hash, lane uint64) uint64 {
	ret := uint64(0)
	for i := lane * SliceSizeBytes; i < (lane+1)*SliceSizeBytes; i++ {
		ret = (ret << 8) + uint64(itemKeyHash[i])
	}
	return ret % occ.header.capacity
}

func (occ *OnChainCuckoo) relocateItem(cuckooItem CuckooItem, triesSoFar uint64) {
	if triesSoFar >= NumLanes {
		// we failed to find a place, even after several relocations, so just discard the item
		// this should happen with negligible probability
		if cuckooItem.generation == occ.header.currentGeneration {
			occ.header.currentGenCount -= 1
			occ.header.inCacheCount -= 1
		} else if cuckooItem.generation+1 == occ.header.currentGeneration {
			occ.header.inCacheCount -= 1
		}
	} else {
		itemKeyHash := crypto.Keccak256Hash(CacheItemToBytes(cuckooItem.itemKey))
		for lane := uint64(0); lane < NumLanes; lane++ {
			slot := occ.getSlotForLane(itemKeyHash, lane)
			if occ.table[slot][lane].generation+1 < occ.header.currentGeneration {
				occ.table[slot][lane] = cuckooItem
				return
			}
		}

		// we failed to find a place for the item, so relocate another item, recursively
		slot := occ.getSlotForLane(itemKeyHash, triesSoFar)
		displacedItem := occ.table[slot][triesSoFar]
		occ.table[slot][triesSoFar] = cuckooItem
		occ.relocateItem(displacedItem, triesSoFar+1)
	}
}

func ForAllOnChainCachedItems[Accumulator any](
	cache *OnChainCuckoo,
	f func(key CacheItemKey, inLatestGeneration bool, t Accumulator) Accumulator,
	t Accumulator,
) Accumulator {
	tt := t
	for slot := uint64(0); slot < cache.header.capacity; slot++ {
		for lane := 0; lane < NumLanes; lane++ {
			if cache.table[slot][lane].generation+1 >= cache.header.currentGeneration {
				tt = f(
					cache.table[slot][lane].itemKey,
					cache.table[slot][lane].generation == cache.header.currentGeneration,
					tt,
				)
			}
		}
	}
	return tt
}
