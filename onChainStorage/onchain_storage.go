package onChainStorage

import "github.com/ethereum/go-ethereum/common"

type OnChainStorage interface {
	Read(location common.Hash) common.Hash
	Write(location, value common.Hash)
}

type MockOnChainStorage struct {
	contents   map[common.Hash]common.Hash
	readCount  uint64
	writeCount uint64
}

func NewMockOnChainStorage() OnChainStorage {
	return &MockOnChainStorage{contents: make(map[common.Hash]common.Hash)}
}

func (m *MockOnChainStorage) Read(location common.Hash) common.Hash {
	m.readCount++
	value, exists := m.contents[location]
	if exists {
		return value
	} else {
		return common.Hash{}
	}
}

func (m *MockOnChainStorage) Write(location, value common.Hash) {
	m.writeCount++
	if value == (common.Hash{}) {
		delete(m.contents, location)
	} else {
		m.contents[location] = value
	}
}

func (m *MockOnChainStorage) GetAccessCounts() (uint64, uint64) {
	return m.readCount, m.writeCount
}
