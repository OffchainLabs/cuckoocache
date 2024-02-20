// Copyright 2024, Offchain Labs, Inc.
// For license information, see https://github.com/OffchainLabs/nitro/blob/master/LICENSE

package onChainIndex

import (
	"encoding/binary"
	"github.com/ethereum/go-ethereum/common"
	"github.com/offchainlabs/cuckoocache/onChainStorage"
)

type OnChainCuckooTable struct {
	storage       onChainStorage.OnChainStorage
	cacheCapacity uint64
	header        onChainStorage.OnChainStorageSlot
	slots         []onChainStorage.OnChainStorageSlot
}

func OpenOnChainCuckooTable(storage onChainStorage.OnChainStorage, cacheCapacity uint64) *OnChainCuckooTable {
	return &OnChainCuckooTable{
		storage:       storage,
		cacheCapacity: cacheCapacity,
		header:        storage.Slot(common.Hash{}),
		slots:         make([]onChainStorage.OnChainStorageSlot, cacheCapacity*NumLanes),
	}
}

func (sb *OnChainCuckooTable) ReadHeader() (OnChainCuckooHeader, error) {
	buf, err := sb.header.Get()
	if err != nil {
		return OnChainCuckooHeader{}, nil
	}
	return OnChainCuckooHeader{
		Capacity:          binary.LittleEndian.Uint64(buf[0:8]),
		CurrentGeneration: binary.LittleEndian.Uint64(buf[8:16]),
		CurrentGenCount:   binary.LittleEndian.Uint64(buf[16:24]),
		InCacheCount:      binary.LittleEndian.Uint64(buf[24:32]),
	}, nil
}

func (sb *OnChainCuckooTable) WriteHeader(header OnChainCuckooHeader) error {
	buf := common.BytesToHash(
		binary.LittleEndian.AppendUint64(
			binary.LittleEndian.AppendUint64(
				binary.LittleEndian.AppendUint64(
					binary.LittleEndian.AppendUint64([]byte{}, header.Capacity),
					header.CurrentGeneration,
				),
				header.CurrentGenCount,
			),
			header.InCacheCount,
		),
	)
	return sb.header.Set(buf)
}

func (sb *OnChainCuckooTable) slotForTableEntry(slot, lane uint64) onChainStorage.OnChainStorageSlot {
	slotNum := lane*sb.cacheCapacity + slot
	theSlot := sb.slots[slotNum]
	if theSlot == nil {
		theSlot = sb.storage.Slot(common.BytesToHash(binary.LittleEndian.AppendUint64([]byte{}, slotNum+1)))
		sb.slots[slotNum] = theSlot
	}
	return theSlot
}

func (sb *OnChainCuckooTable) ReadTableEntry(slot, lane uint64) (CuckooItem, error) {
	buf, err := sb.slotForTableEntry(slot, lane).Get()
	if err != nil {
		return CuckooItem{}, err
	}
	itemKey := [24]byte{}
	copy(itemKey[:], buf[0:24])
	return CuckooItem{
		ItemKey:    itemKey,
		Generation: binary.LittleEndian.Uint64(buf[24:32]),
	}, nil
}

func (sb *OnChainCuckooTable) WriteTableEntry(slot, lane uint64, cuckooItem CuckooItem) error {
	buf := binary.LittleEndian.AppendUint64(cuckooItem.ItemKey[:], cuckooItem.Generation)
	return sb.slotForTableEntry(slot, lane).Set(common.BytesToHash(buf))
}
