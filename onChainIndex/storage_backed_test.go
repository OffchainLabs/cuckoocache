package onChainIndex

import (
	"github.com/offchainlabs/cuckoo-cache/onChainStorage"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestStorageBacked(t *testing.T) {
	capacity := uint64(64)
	storage := onChainStorage.NewMockOnChainStorage()
	sb := OpenOnChainCuckooTable(storage, capacity)

	// everything should be empty to start
	citem := sb.ReadTableEntry(3, 17)
	assert.Equal(t, citem.ItemKey, [24]byte{})
	assert.Equal(t, citem.Generation, uint64(0))
	header := sb.ReadHeader()
	assert.Equal(t, header.InCacheCount, uint64(0))
	assert.Equal(t, header.Capacity, uint64(0))
	assert.Equal(t, header.CurrentGenCount, uint64(0))
	assert.Equal(t, header.CurrentGeneration, uint64(0))

	myHeader := OnChainCuckooHeader{
		Capacity:          37,
		CurrentGeneration: 99,
		CurrentGenCount:   106,
		InCacheCount:      3,
	}
	sb.WriteHeader(myHeader)
	assert.Equal(t, sb.ReadHeader(), myHeader)

	item00 := CuckooItem{
		ItemKey:    keyFromUint64(13),
		Generation: 13,
	}
	sb.WriteTableEntry(0, 0, item00)
	assert.Equal(t, sb.ReadHeader(), myHeader)
	assert.Equal(t, sb.ReadTableEntry(0, 0), item00)

	item39 := CuckooItem{
		ItemKey:    keyFromUint64(39),
		Generation: 39,
	}
	sb.WriteTableEntry(3, 9, item39)
	assert.Equal(t, sb.ReadHeader(), myHeader)
	assert.Equal(t, sb.ReadTableEntry(0, 0), item00)
	assert.Equal(t, sb.ReadTableEntry(3, 9), item39)
}
