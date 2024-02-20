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
}

func OpenOnChainCuckooTable(storage onChainStorage.OnChainStorage, cacheCapacity uint64) *OnChainCuckooTable {
	return &OnChainCuckooTable{storage, cacheCapacity}
}

func (sb *OnChainCuckooTable) ReadHeader() (OnChainCuckooHeader, error) {
	buf, err := sb.storage.Get(common.Hash{})
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
	return sb.storage.Set(common.Hash{}, buf)
}

func locationForTableEntry(slot, lane uint64) common.Hash {
	val := 1 + lane + NumLanes*slot
	return common.BytesToHash(binary.LittleEndian.AppendUint64([]byte{}, val))
}

func (sb *OnChainCuckooTable) ReadTableEntry(slot, lane uint64) (CuckooItem, error) {
	buf, err := sb.storage.Get(locationForTableEntry(slot, lane))
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
	return sb.storage.Set(locationForTableEntry(slot, lane), common.BytesToHash(buf))
}
