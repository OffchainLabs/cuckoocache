package cuckoo_cache

import (
	"github.com/ethereum/go-ethereum/crypto"
)

const LogMaxCacheSize = 16
const MaxCacheSize = 1 << LogMaxCacheSize

type CacheItemKey = [24]byte

const NumLanes = 8

type OnChainCuckooHeader struct {
	capacity          uint64
	currentGeneration uint64
	currentGenCount   uint64
	inCacheCount      uint64
}

type CuckooItem struct {
	itemKey    CacheItemKey
	generation uint64
}

func (oc *OnChainCuckooTable) Initialize(capacity uint64) {
	header := OnChainCuckooHeader{
		capacity:          capacity,
		currentGeneration: 2, // so that uninitialized CuckooItems don't look like they're in cache
	}
	oc.writeHeader(header)
}

func (oc *OnChainCuckooTable) IsInCache(header *OnChainCuckooHeader, itemKey CacheItemKey) bool {
	itemKeyHash := crypto.Keccak256(itemKey[:])
	for lane := uint64(0); lane < NumLanes; lane++ {
		slot := header.getSlotForLane(itemKeyHash, lane)
		cuckooItem := oc.readTableEntry(slot, lane)
		if cuckooItem.itemKey == itemKey && cuckooItem.generation != 0 {
			return cuckooItem.generation+1 >= header.currentGeneration
		}
	}
	return false
}

func (oc *OnChainCuckooTable) AccessItem(itemKey CacheItemKey) (bool, uint64) { // hit, current generation after access
	itemKeyHash := crypto.Keccak256(itemKey[:])
	hdr := oc.readHeader()
	header := &hdr
	expiredItemFoundInLane := uint64(NumLanes) // NumLanes means that no expired item has been found yet
	doubleExpiredFound := false
	for lane := uint64(0); (!doubleExpiredFound) && (lane < NumLanes); lane++ {
		slot := header.getSlotForLane(itemKeyHash, lane)
		itemFromTable := oc.readTableEntry(slot, lane)
		if itemFromTable.itemKey == itemKey {
			cachedGeneration := itemFromTable.generation
			if cachedGeneration == header.currentGeneration {
				return true, header.currentGeneration
			} else if cachedGeneration+1 == header.currentGeneration {
				itemFromTable.generation = header.currentGeneration
				if expiredItemFoundInLane < lane {
					oc.writeTableEntry(slot, lane, itemFromTable)
				} else {
					oc.writeTableEntry(slot, lane, itemFromTable)
				}
				header.currentGenCount += 1
				oc.writeHeader(*header)
				return true, header.currentGeneration
			} else {
				// the item is in the table but is expired
				_ = oc.advanceGenerationIfNeeded(header)
				itemFromTable.generation = header.currentGeneration
				if expiredItemFoundInLane < lane {
					oc.writeTableEntry(slot, lane, itemFromTable)
				} else {
					oc.writeTableEntry(slot, lane, itemFromTable)
				}
				header.currentGenCount += 1
				header.inCacheCount += 1
				oc.writeHeader(*header)
				return false, header.currentGeneration
			}
		} else if itemFromTable.generation+1 < header.currentGeneration {
			expiredItemFoundInLane = lane
			if itemFromTable.generation+2 < header.currentGeneration {
				// we can stop searching for the item we want, because if the item were in-cache,
				// it would have overwritten this item in the past
				doubleExpiredFound = true
			}
		}
	}
	_ = oc.advanceGenerationIfNeeded(header)
	if expiredItemFoundInLane < NumLanes {
		// didn't find the item in the table, so replace an expired item
		slot := header.getSlotForLane(itemKeyHash, expiredItemFoundInLane)
		oc.writeTableEntry(
			slot,
			expiredItemFoundInLane,
			CuckooItem{itemKey, header.currentGeneration},
		)
		header.currentGenCount += 1
		header.inCacheCount += 1
	} else {
		slot := header.getSlotForLane(itemKeyHash, 0)
		itemKeyToRelocate := oc.readTableEntry(slot, 0)
		oc.writeTableEntry(
			slot,
			0,
			CuckooItem{itemKey: itemKey, generation: header.currentGeneration},
		)
		header.currentGenCount += 1
		header.inCacheCount += 1
		oc.relocateItem(itemKeyToRelocate, 1, header)
	}
	oc.writeHeader(*header)
	return false, header.currentGeneration
}

func (oc *OnChainCuckooTable) advanceGenerationIfNeeded(header *OnChainCuckooHeader) bool {
	modifiedHeader := false
	for header.inCacheCount >= header.capacity || header.currentGenCount > 4*header.capacity/5 {
		header.currentGeneration += 1
		header.inCacheCount = header.currentGenCount
		header.currentGenCount = 0
		modifiedHeader = true
	}
	return modifiedHeader
}

const SliceSizeBytes = (LogMaxCacheSize + 7) / 8

func (header *OnChainCuckooHeader) getSlotForLane(itemKeyHash []byte, lane uint64) uint64 {
	ret := uint64(0)
	for i := lane * SliceSizeBytes; i < (lane+1)*SliceSizeBytes; i++ {
		ret = (ret << 8) + uint64(itemKeyHash[i])
	}
	return ret % header.capacity
}

func (oc *OnChainCuckooTable) relocateItem(
	cuckooItem CuckooItem,
	triesSoFar uint64,
	header *OnChainCuckooHeader,
) {
	if triesSoFar >= NumLanes {
		// we failed to find a place, even after several relocations, so just discard the item
		// this should happen with negligible probability
		if cuckooItem.generation == header.currentGeneration {
			header.currentGenCount -= 1
			header.inCacheCount -= 1
		} else if cuckooItem.generation+1 == header.currentGeneration {
			header.inCacheCount -= 1
		}
	} else {
		itemKeyHash := crypto.Keccak256(cuckooItem.itemKey[:])
		for lane := uint64(0); lane < NumLanes; lane++ {
			slot := header.getSlotForLane(itemKeyHash, lane)
			thisItem := oc.readTableEntry(slot, lane)
			if thisItem.generation+1 < header.currentGeneration {
				oc.writeTableEntry(slot, lane, cuckooItem)
				return
			}
		}

		// we failed to find a place for the item, so relocate another item, recursively
		slot := header.getSlotForLane(itemKeyHash, triesSoFar)
		displacedItem := oc.readTableEntry(slot, triesSoFar)
		oc.writeTableEntry(slot, triesSoFar, cuckooItem)
		oc.relocateItem(displacedItem, triesSoFar+1, header)
	}
}

func ForAllOnChainCachedItems[Accumulator any](
	cache *OnChainCuckooTable,
	f func(key CacheItemKey, inLatestGeneration bool, t Accumulator) Accumulator,
	t Accumulator,
) Accumulator {
	tt := t
	header := cache.readHeader()
	for slot := uint64(0); slot < header.capacity; slot++ {
		for lane := uint64(0); lane < NumLanes; lane++ {
			thisItem := cache.readTableEntry(slot, lane)
			if thisItem.generation+1 >= header.currentGeneration {
				tt = f(
					thisItem.itemKey,
					thisItem.generation == header.currentGeneration,
					tt,
				)
			}
		}
	}
	return tt
}
