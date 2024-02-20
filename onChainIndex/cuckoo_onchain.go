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

func (oc *OnChainCuckooTable) Initialize(capacity uint64) error {
	header := OnChainCuckooHeader{
		Capacity:          capacity,
		CurrentGeneration: 3, // so that uninitialized CuckooItems look like they're double-expired
	}
	return oc.WriteHeader(header)
}

func (oc *OnChainCuckooTable) IsInCache(header *OnChainCuckooHeader, itemKey CacheItemKey) (bool, error) {
	for lane := uint64(0); lane < NumLanes; lane++ {
		slot := header.getSlotForLane(itemKey, lane)
		cuckooItem, err := oc.ReadTableEntry(slot, lane)
		if err != nil {
			return false, err
		}
		if cuckooItem.ItemKey == itemKey && cuckooItem.Generation != 0 {
			return cuckooItem.Generation+1 >= header.CurrentGeneration, nil
		}
	}
	return false, nil
}

func (oc *OnChainCuckooTable) AccessItem(itemKey CacheItemKey) (bool, uint64, error) { // hit, current generation after access
	hdr, err := oc.ReadHeader()
	if err != nil {
		return false, 0, err
	}
	header := &hdr
	for lane := uint64(0); lane < NumLanes; lane++ {
		slot := header.getSlotForLane(itemKey, lane)
		itemFromTable, err := oc.ReadTableEntry(slot, lane)
		if err != nil {
			return false, 0, err
		}
		if itemFromTable.ItemKey == itemKey {
			cachedGeneration := itemFromTable.Generation
			if cachedGeneration == header.CurrentGeneration {
				return true, header.CurrentGeneration, nil
			} else if cachedGeneration+1 == header.CurrentGeneration {
				itemFromTable.Generation = header.CurrentGeneration
				if err := oc.WriteTableEntry(slot, lane, itemFromTable); err != nil {
					return false, 0, err
				}
				header.CurrentGenCount += 1
				_ = oc.advanceGenerationIfNeeded(header)
				if err := oc.WriteHeader(*header); err != nil {
					return false, 0, err
				}
				return true, header.CurrentGeneration, nil
			} else {
				// the item is in the table but is expired
				itemFromTable.Generation = header.CurrentGeneration
				if err := oc.WriteTableEntry(slot, lane, itemFromTable); err != nil {
					return false, 0, err
				}
				header.CurrentGenCount += 1
				header.InCacheCount += 1
				_ = oc.advanceGenerationIfNeeded(header)
				if err := oc.WriteHeader(*header); err != nil {
					return false, 0, err
				}
				return false, header.CurrentGeneration, nil
			}
		} else if itemFromTable.Generation+1 < header.CurrentGeneration {
			if err := oc.WriteTableEntry(
				slot,
				lane,
				CuckooItem{ItemKey: itemKey, Generation: header.CurrentGeneration},
			); err != nil {
				return false, 0, err
			}
			header.CurrentGenCount += 1
			wasInOldGeneration, err := oc.findExactMatch(itemKey, header.CurrentGeneration-1, lane+1, header)
			if err != nil {
				return false, 0, err
			}
			if !wasInOldGeneration {
				header.InCacheCount += 1
			}
			_ = oc.advanceGenerationIfNeeded(header)
			if err := oc.WriteHeader(*header); err != nil {
				return false, 0, err
			}
			return wasInOldGeneration, header.CurrentGeneration, nil
		}
	}

	slot := header.getSlotForLane(itemKey, 0)
	itemKeyToRelocate, err := oc.ReadTableEntry(slot, 0)
	if err != nil {
		return false, 0, err
	}
	if err := oc.WriteTableEntry(
		slot,
		0,
		CuckooItem{ItemKey: itemKey, Generation: header.CurrentGeneration},
	); err != nil {
		return false, 0, err
	}
	header.CurrentGenCount += 1
	header.InCacheCount += 1

	if err := oc.relocateItem(itemKeyToRelocate, 1, header); err != nil {
		return false, 0, err
	}
	_ = oc.advanceGenerationIfNeeded(header)
	if err := oc.WriteHeader(*header); err != nil {
		return false, 0, err
	}
	return false, header.CurrentGeneration, nil
}

func (oc *OnChainCuckooTable) findExactMatch(
	itemKey CacheItemKey,
	generation uint64,
	startInLane uint64,
	header *OnChainCuckooHeader,
) (bool, error) {
	for lane := startInLane; lane < NumLanes; lane++ {
		slot := header.getSlotForLane(itemKey, lane)
		item, err := oc.ReadTableEntry(slot, lane)
		if err != nil {
			return false, err
		}
		if item.ItemKey == itemKey {
			return item.Generation == generation, nil
		} else if item.Generation < generation-1 {
			return false, nil
		}
	}
	return false, nil
}

func (oc *OnChainCuckooTable) FlushAll() error {
	header, err := oc.ReadHeader()
	if err != nil {
		return err
	}
	header.CurrentGeneration += 3
	header.CurrentGenCount = 0
	header.InCacheCount = 0
	return oc.WriteHeader(header)
}

func (oc *OnChainCuckooTable) FlushOneItem(itemKey CacheItemKey) error {
	header, err := oc.ReadHeader()
	if err != nil {
		return err
	}
	for lane := uint64(0); lane < NumLanes; lane++ {
		slot := header.getSlotForLane(itemKey, lane)
		cuckooItem, err := oc.ReadTableEntry(slot, lane)
		if err != nil {
			return err
		}
		if cuckooItem.Generation+3 <= header.CurrentGeneration {
			return nil
		} else if cuckooItem.ItemKey == itemKey && cuckooItem.Generation != 0 {
			cuckooItem.Generation = header.CurrentGeneration - 2
			if err := oc.WriteTableEntry(slot, lane, cuckooItem); err != nil {
				return err
			}
		}
	}
	return nil
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
) error {
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
			thisItem, err := oc.ReadTableEntry(slot, lane)
			if err != nil {
				return err
			}
			if thisItem.ItemKey == cuckooItem.ItemKey {
				if thisItem.Generation < cuckooItem.Generation {
					if err := oc.WriteTableEntry(slot, lane, cuckooItem); err != nil {
						return err
					}
				}
				return nil
			} else if thisItem.Generation+1 < header.CurrentGeneration {
				return oc.WriteTableEntry(slot, lane, cuckooItem)
			}
		}

		// we failed to find a place for the item, so relocate another item, recursively
		slot := header.getSlotForLane(cuckooItem.ItemKey, triesSoFar)
		displacedItem, err := oc.ReadTableEntry(slot, triesSoFar)
		if err != nil {
			return err
		}
		if err := oc.WriteTableEntry(slot, triesSoFar, cuckooItem); err != nil {
			return err
		}
		return oc.relocateItem(displacedItem, triesSoFar+1, header)
	}
	return nil
}

func ForAllOnChainCachedItems[Accumulator any](
	cache *OnChainCuckooTable,
	f func(key CacheItemKey, inLatestGeneration bool, t Accumulator) (Accumulator, error),
	t Accumulator,
) (Accumulator, error) {
	tt := t
	header, err := cache.ReadHeader()
	if err != nil {
		return tt, err
	}
	for slot := uint64(0); slot < header.Capacity; slot++ {
		for lane := uint64(0); lane < NumLanes; lane++ {
			thisItem, err := cache.ReadTableEntry(slot, lane)
			if err != nil {
				return tt, err
			}
			if thisItem.Generation+1 >= header.CurrentGeneration {
				tt, err = f(
					thisItem.ItemKey,
					thisItem.Generation == header.CurrentGeneration,
					tt,
				)
				if err != nil {
					return tt, err
				}
			}
		}
	}
	return tt, nil
}
