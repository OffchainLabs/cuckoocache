package cuckoo_cache

import (
	"github.com/ethereum/go-ethereum/crypto"
	"offchainlabs.com/cuckoo-cache/onChain"
)

type CacheBackingStore interface {
	Read(key onChain.CacheItemKey) []byte
}

type MockCacheBackingStore struct {
	contents map[onChain.CacheItemKey][]byte
}

func NewMockBackingStore() CacheBackingStore {
	return &MockCacheBackingStore{contents: make(map[onChain.CacheItemKey][]byte)}
}

func (mbs *MockCacheBackingStore) Read(key onChain.CacheItemKey) []byte {
	value := mbs.contents[key]
	if value == nil {
		value = crypto.Keccak256(key[:])
	}
	return append([]byte{}, value...)
}

func (mbs *MockCacheBackingStore) Write(key onChain.CacheItemKey, value []byte) {
	mbs.contents[key] = append([]byte{}, value...)
}
