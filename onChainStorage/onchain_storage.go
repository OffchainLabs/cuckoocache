package onChainStorage

import "github.com/ethereum/go-ethereum/common"

type OnChainStorage interface {
	Read(location common.Hash) common.Hash
	Write(location, value common.Hash)
}

type MockOnChainStorage struct {
	contents map[common.Hash]common.Hash
}

func NewMockOnChainStorage() OnChainStorage {
	return &MockOnChainStorage{contents: make(map[common.Hash]common.Hash)}
}

func (m *MockOnChainStorage) Read(location common.Hash) common.Hash {
	value, exists := m.contents[location]
	if exists {
		return value
	} else {
		return common.Hash{}
	}
}

func (m *MockOnChainStorage) Write(location, value common.Hash) {
	if value == (common.Hash{}) {
		delete(m.contents, location)
	} else {
		m.contents[location] = value
	}
}
