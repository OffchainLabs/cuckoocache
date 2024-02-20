// Copyright 2024, Offchain Labs, Inc.
// For license information, see https://github.com/OffchainLabs/nitro/blob/master/LICENSE

package onChainIndex

const LogMaxCacheSize = 16
const MaxCacheSize = 1 << LogMaxCacheSize

type CacheItemKey = [24]byte

const NumLanes = 8

type OnChainCuckooHeader struct {
	Capacity          uint64
	CurrentGeneration uint64
	CurrentGenCount   uint64
	InCacheCount      uint64
}

type CuckooItem struct {
	ItemKey    CacheItemKey
	Generation uint64
}

func (oc *OnChainCuckooTable) Initialize(capacity uint64) {
	header := OnChainCuckooHeader{
		Capacity:          capacity,
		CurrentGeneration: 3, // so that uninitialized CuckooItems look like they're double-expired
	}
	oc.WriteHeader(header)
}

func (oc *OnChainCuckooTable) IsInCache(header *OnChainCuckooHeader, itemKey CacheItemKey) bool {
	for lane := uint64(0); lane < NumLanes; lane++ {
		slot := header.getSlotForLane(itemKey, lane)
		cuckooItem := oc.ReadTableEntry(slot, lane)
		if cuckooItem.ItemKey == itemKey && cuckooItem.Generation != 0 {
			return cuckooItem.Generation+1 >= header.CurrentGeneration
		}
	}
	return false
}

func (oc *OnChainCuckooTable) AccessItem(itemKey CacheItemKey) (bool, uint64) { // hit, current generation after access
	hdr := oc.ReadHeader()
	header := &hdr
	for lane := uint64(0); lane < NumLanes; lane++ {
		slot := header.getSlotForLane(itemKey, lane)
		itemFromTable := oc.ReadTableEntry(slot, lane)
		if itemFromTable.ItemKey == itemKey {
			cachedGeneration := itemFromTable.Generation
			if cachedGeneration == header.CurrentGeneration {
				return true, header.CurrentGeneration
			} else if cachedGeneration+1 == header.CurrentGeneration {
				itemFromTable.Generation = header.CurrentGeneration
				oc.WriteTableEntry(slot, lane, itemFromTable)
				header.CurrentGenCount += 1
				_ = oc.advanceGenerationIfNeeded(header)
				oc.WriteHeader(*header)
				return true, header.CurrentGeneration
			} else {
				// the item is in the table but is expired
				itemFromTable.Generation = header.CurrentGeneration
				oc.WriteTableEntry(slot, lane, itemFromTable)
				header.CurrentGenCount += 1
				header.InCacheCount += 1
				_ = oc.advanceGenerationIfNeeded(header)
				oc.WriteHeader(*header)
				return false, header.CurrentGeneration
			}
		} else if itemFromTable.Generation+1 < header.CurrentGeneration {
			oc.WriteTableEntry(slot, lane, CuckooItem{ItemKey: itemKey, Generation: header.CurrentGeneration})
			header.CurrentGenCount += 1
			wasInOldGeneration := oc.findExactMatch(itemKey, header.CurrentGeneration-1, lane+1, header)
			if !wasInOldGeneration {
				header.InCacheCount += 1
			}
			_ = oc.advanceGenerationIfNeeded(header)
			oc.WriteHeader(*header)
			return wasInOldGeneration, header.CurrentGeneration
		}
	}

	slot := header.getSlotForLane(itemKey, 0)
	itemKeyToRelocate := oc.ReadTableEntry(slot, 0)
	oc.WriteTableEntry(
		slot,
		0,
		CuckooItem{ItemKey: itemKey, Generation: header.CurrentGeneration},
	)
	header.CurrentGenCount += 1
	header.InCacheCount += 1

	oc.relocateItem(itemKeyToRelocate, 1, header)
	_ = oc.advanceGenerationIfNeeded(header)
	oc.WriteHeader(*header)
	return false, header.CurrentGeneration
}

func (oc *OnChainCuckooTable) findExactMatch(itemKey CacheItemKey, generation uint64, startInLane uint64, header *OnChainCuckooHeader) bool {
	for lane := startInLane; lane < NumLanes; lane++ {
		slot := header.getSlotForLane(itemKey, lane)
		item := oc.ReadTableEntry(slot, lane)
		if item.ItemKey == itemKey {
			return item.Generation == generation
		} else if item.Generation < generation-1 {
			return false
		}
	}
	return false
}

func (oc *OnChainCuckooTable) FlushAll() {
	header := oc.ReadHeader()
	header.CurrentGeneration += 3
	header.CurrentGenCount = 0
	header.InCacheCount = 0
	oc.WriteHeader(header)
}

func (oc *OnChainCuckooTable) FlushOneItem(itemKey CacheItemKey) {
	header := oc.ReadHeader()
	for lane := uint64(0); lane < NumLanes; lane++ {
		slot := header.getSlotForLane(itemKey, lane)
		cuckooItem := oc.ReadTableEntry(slot, lane)
		if cuckooItem.Generation+3 <= header.CurrentGeneration {
			return
		} else if cuckooItem.ItemKey == itemKey && cuckooItem.Generation != 0 {
			cuckooItem.Generation = header.CurrentGeneration - 2
			oc.WriteTableEntry(slot, lane, cuckooItem)
		}
	}
}

func (oc *OnChainCuckooTable) advanceGenerationIfNeeded(header *OnChainCuckooHeader) bool {
	modifiedHeader := false
	for header.InCacheCount > header.Capacity || header.CurrentGenCount > 3*header.Capacity/4 {
		header.CurrentGeneration += 1
		header.InCacheCount = header.CurrentGenCount
		header.CurrentGenCount = 0
		modifiedHeader = true
	}
	return modifiedHeader
}

const SliceSizeBytes = (LogMaxCacheSize + 7) / 8

func (header *OnChainCuckooHeader) getSlotForLane(itemKey CacheItemKey, lane uint64) uint64 {
	ret := uint64(0)
	for i := lane * SliceSizeBytes; i < (lane+1)*SliceSizeBytes; i++ {
		ret = (ret << 8) + uint64(itemKey[i])
	}
	return ret % header.Capacity
}

func (oc *OnChainCuckooTable) relocateItem(
	cuckooItem CuckooItem,
	triesSoFar uint64,
	header *OnChainCuckooHeader,
) {
	if triesSoFar >= NumLanes {
		// we failed to find a place, even after several relocations, so just discard the item
		// this should happen with negligible probability
		if cuckooItem.Generation == header.CurrentGeneration {
			header.CurrentGenCount -= 1
			header.InCacheCount -= 1
		} else if cuckooItem.Generation+1 == header.CurrentGeneration {
			header.InCacheCount -= 1
		}
	} else {
		for lane := uint64(0); lane < NumLanes; lane++ {
			slot := header.getSlotForLane(cuckooItem.ItemKey, lane)
			thisItem := oc.ReadTableEntry(slot, lane)
			if thisItem.ItemKey == cuckooItem.ItemKey {
				if thisItem.Generation < cuckooItem.Generation {
					oc.WriteTableEntry(slot, lane, cuckooItem)
				}
				return
			} else if thisItem.Generation+1 < header.CurrentGeneration {
				oc.WriteTableEntry(slot, lane, cuckooItem)
				return
			}
		}

		// we failed to find a place for the item, so relocate another item, recursively
		slot := header.getSlotForLane(cuckooItem.ItemKey, triesSoFar)
		displacedItem := oc.ReadTableEntry(slot, triesSoFar)
		oc.WriteTableEntry(slot, triesSoFar, cuckooItem)
		oc.relocateItem(displacedItem, triesSoFar+1, header)
	}
}

func ForAllOnChainCachedItems[Accumulator any](
	cache *OnChainCuckooTable,
	f func(key CacheItemKey, inLatestGeneration bool, t Accumulator) Accumulator,
	t Accumulator,
) Accumulator {
	tt := t
	header := cache.ReadHeader()
	for slot := uint64(0); slot < header.Capacity; slot++ {
		for lane := uint64(0); lane < NumLanes; lane++ {
			thisItem := cache.ReadTableEntry(slot, lane)
			if thisItem.Generation+1 >= header.CurrentGeneration {
				tt = f(
					thisItem.ItemKey,
					thisItem.Generation == header.CurrentGeneration,
					tt,
				)
			}
		}
	}
	return tt
}
