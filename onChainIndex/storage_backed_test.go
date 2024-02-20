// Copyright 2024, Offchain Labs, Inc.
// For license information, see https://github.com/OffchainLabs/nitro/blob/master/LICENSE

package onChainIndex

import (
	"github.com/offchainlabs/cuckoocache/onChainStorage"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestStorageBacked(t *testing.T) {
	capacity := uint64(64)
	storage := onChainStorage.NewMockOnChainStorage()
	sb := OpenOnChainCuckooTable(storage, capacity)

	// everything should be empty to start
	citem, err := sb.ReadTableEntry(17, 3)
	assert.Nil(t, err)
	assert.Equal(t, citem.ItemKey, [24]byte{})
	assert.Equal(t, citem.Generation, uint64(0))
	header, err := sb.ReadHeader()
	assert.Nil(t, err)
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
	assert.Nil(t, sb.WriteHeader(myHeader))
	header, err = sb.ReadHeader()
	assert.Nil(t, err)
	assert.Equal(t, header, myHeader)

	item00 := CuckooItem{
		ItemKey:    keyFromUint64(13),
		Generation: 13,
	}
	assert.Nil(t, sb.WriteTableEntry(0, 0, item00))
	header, err = sb.ReadHeader()
	assert.Nil(t, err)
	assert.Equal(t, header, myHeader)
	entry, err := sb.ReadTableEntry(0, 0)
	assert.Nil(t, err)
	assert.Equal(t, entry, item00)

	item39 := CuckooItem{
		ItemKey:    keyFromUint64(39),
		Generation: 39,
	}
	assert.Nil(t, sb.WriteTableEntry(9, 3, item39))
	header, err = sb.ReadHeader()
	assert.Nil(t, err)
	assert.Equal(t, header, myHeader)
	entry, err = sb.ReadTableEntry(0, 0)
	assert.Nil(t, err)
	assert.Equal(t, entry, item00)
	entry, err = sb.ReadTableEntry(9, 3)
	assert.Nil(t, err)
	assert.Equal(t, entry, item39)
}
