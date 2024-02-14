package cacheBackingStore

import (
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/offchainlabs/cuckoo-cache/onChainIndex"
)

type CacheBackingStore interface {
	Read(key onChainIndex.CacheItemKey) []byte
}

type MockCacheBackingStore struct {
	contents map[onChainIndex.CacheItemKey][]byte
}

func NewMockBackingStore() CacheBackingStore {
	return &MockCacheBackingStore{contents: make(map[onChainIndex.CacheItemKey][]byte)}
}

func (mbs *MockCacheBackingStore) Read(key onChainIndex.CacheItemKey) []byte {
	value := mbs.contents[key]
	if value == nil {
		value = crypto.Keccak256(key[:])
	}
	return append([]byte{}, value...)
}

func (mbs *MockCacheBackingStore) Write(key onChainIndex.CacheItemKey, value []byte) {
	mbs.contents[key] = append([]byte{}, value...)
}
