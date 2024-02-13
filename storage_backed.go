package cuckoo_cache

import (
	"encoding/binary"
	"github.com/ethereum/go-ethereum/common"
)

type OnChainCuckooTable struct {
	storage       OnChainStorage
	cacheCapacity uint64
}

func OpenOnChainCuckooTable(storage OnChainStorage, cacheCapacity uint64) *OnChainCuckooTable {
	return &OnChainCuckooTable{storage, cacheCapacity}
}

func (sb *OnChainCuckooTable) readHeader() OnChainCuckooHeader {
	buf := sb.storage.Read(common.Hash{})
	return OnChainCuckooHeader{
		capacity:          binary.LittleEndian.Uint64(buf[0:8]),
		currentGeneration: binary.LittleEndian.Uint64(buf[8:16]),
		currentGenCount:   binary.LittleEndian.Uint64(buf[16:24]),
		inCacheCount:      binary.LittleEndian.Uint64(buf[24:32]),
	}
}

func (sb *OnChainCuckooTable) writeHeader(header OnChainCuckooHeader) {
	buf := common.BytesToHash(
		binary.LittleEndian.AppendUint64(
			binary.LittleEndian.AppendUint64(
				binary.LittleEndian.AppendUint64(
					binary.LittleEndian.AppendUint64([]byte{}, header.capacity),
					header.currentGeneration,
				),
				header.currentGenCount,
			),
			header.inCacheCount,
		),
	)
	sb.storage.Write(common.Hash{}, buf)
}

func locationForTableEntry(slot, lane uint64) common.Hash {
	val := 1 + lane + NumLanes*slot
	return common.BytesToHash(binary.LittleEndian.AppendUint64([]byte{}, val))
}

func (sb *OnChainCuckooTable) readTableEntry(slot, lane uint64) CuckooItem {
	buf := sb.storage.Read(locationForTableEntry(slot, lane))
	itemKey := [24]byte{}
	copy(itemKey[:], buf[0:24])
	return CuckooItem{
		itemKey:    itemKey,
		generation: binary.LittleEndian.Uint64(buf[24:32]),
	}
}

func (sb *OnChainCuckooTable) writeTableEntry(slot, lane uint64, cuckooItem CuckooItem) {
	buf := binary.LittleEndian.AppendUint64(cuckooItem.itemKey[:], cuckooItem.generation)
	sb.storage.Write(locationForTableEntry(slot, lane), common.BytesToHash(buf))
}
