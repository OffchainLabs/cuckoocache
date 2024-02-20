// Copyright 2024, Offchain Labs, Inc.
// For license information, see https://github.com/OffchainLabs/nitro/blob/master/LICENSE

package onChainStorage

import "github.com/ethereum/go-ethereum/common"

type OnChainStorage interface {
	Get(location common.Hash) (common.Hash, error)
	Set(location, value common.Hash) error
}

type MockOnChainStorage struct {
	contents   map[common.Hash]common.Hash
	readCount  uint64
	writeCount uint64
}

func NewMockOnChainStorage() OnChainStorage {
	return &MockOnChainStorage{contents: make(map[common.Hash]common.Hash)}
}

func (m *MockOnChainStorage) Get(location common.Hash) (common.Hash, error) {
	m.readCount++
	value, exists := m.contents[location]
	if exists {
		return value, nil
	} else {
		return common.Hash{}, nil
	}
}

func (m *MockOnChainStorage) Set(location, value common.Hash) error {
	m.writeCount++
	if value == (common.Hash{}) {
		delete(m.contents, location)
	} else {
		m.contents[location] = value
	}
	return nil
}

func (m *MockOnChainStorage) GetAccessCounts() (uint64, uint64) {
	return m.readCount, m.writeCount
}
