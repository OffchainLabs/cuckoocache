package generational_cache

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestStorageBacked(t *testing.T) {
	capacity := uint64(64)
	storage := NewMockOnChainStorage()
	sb := newStorageBackedOnChainCuckoo(storage, capacity)

	// everything should be empty to start
	citem := sb.readTableEntry(3, 17)
	assert.Equal(t, citem.itemKey, common.BytesToAddress(make([]byte, 20)))
	assert.Equal(t, citem.generation, uint64(0))
	header := sb.readHeader()
	assert.Equal(t, header.inCacheCount, uint64(0))
	assert.Equal(t, header.capacity, uint64(0))
	assert.Equal(t, header.currentGenCount, uint64(0))
	assert.Equal(t, header.currentGeneration, uint64(0))

	myHeader := OnChainCuckooHeader{
		capacity:          37,
		currentGeneration: 99,
		currentGenCount:   106,
		inCacheCount:      3,
	}
	sb.writeHeader(myHeader)
	assert.Equal(t, sb.readHeader(), myHeader)

	item00 := CuckooItem{
		itemKey:    keyFromUint64(13),
		generation: 13,
	}
	sb.writeTableEntry(0, 0, item00)
	assert.Equal(t, sb.readHeader(), myHeader)
	assert.Equal(t, sb.readTableEntry(0, 0), item00)

	item39 := CuckooItem{
		itemKey:    keyFromUint64(39),
		generation: 39,
	}
	sb.writeTableEntry(3, 9, item39)
	assert.Equal(t, sb.readHeader(), myHeader)
	assert.Equal(t, sb.readTableEntry(0, 0), item00)
	assert.Equal(t, sb.readTableEntry(3, 9), item39)
}
