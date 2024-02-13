package onChain

import (
	"encoding/binary"
	"github.com/ethereum/go-ethereum/common"
	"offchainlabs.com/cuckoo-cache/storage"
)

type OnChainCuckooTable struct {
	storage       storage.OnChainStorage
	cacheCapacity uint64
}

func OpenOnChainCuckooTable(storage storage.OnChainStorage, cacheCapacity uint64) *OnChainCuckooTable {
	return &OnChainCuckooTable{storage, cacheCapacity}
}

func (sb *OnChainCuckooTable) ReadHeader() OnChainCuckooHeader {
	buf := sb.storage.Read(common.Hash{})
	return OnChainCuckooHeader{
		Capacity:          binary.LittleEndian.Uint64(buf[0:8]),
		CurrentGeneration: binary.LittleEndian.Uint64(buf[8:16]),
		CurrentGenCount:   binary.LittleEndian.Uint64(buf[16:24]),
		InCacheCount:      binary.LittleEndian.Uint64(buf[24:32]),
	}
}

func (sb *OnChainCuckooTable) WriteHeader(header OnChainCuckooHeader) {
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
	sb.storage.Write(common.Hash{}, buf)
}

func locationForTableEntry(slot, lane uint64) common.Hash {
	val := 1 + lane + NumLanes*slot
	return common.BytesToHash(binary.LittleEndian.AppendUint64([]byte{}, val))
}

func (sb *OnChainCuckooTable) ReadTableEntry(slot, lane uint64) CuckooItem {
	buf := sb.storage.Read(locationForTableEntry(slot, lane))
	itemKey := [24]byte{}
	copy(itemKey[:], buf[0:24])
	return CuckooItem{
		ItemKey:    itemKey,
		Generation: binary.LittleEndian.Uint64(buf[24:32]),
	}
}

func (sb *OnChainCuckooTable) WriteTableEntry(slot, lane uint64, cuckooItem CuckooItem) {
	buf := binary.LittleEndian.AppendUint64(cuckooItem.ItemKey[:], cuckooItem.Generation)
	sb.storage.Write(locationForTableEntry(slot, lane), common.BytesToHash(buf))
}
