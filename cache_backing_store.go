package cuckoo_cache

import "github.com/ethereum/go-ethereum/crypto"

type CacheBackingStore interface {
	Read(key CacheItemKey) []byte
}

type MockCacheBackingStore struct {
	contents map[CacheItemKey][]byte
}

func NewMockBackingStore() CacheBackingStore {
	return &MockCacheBackingStore{contents: make(map[CacheItemKey][]byte)}
}

func (mbs *MockCacheBackingStore) Read(key CacheItemKey) []byte {
	value := mbs.contents[key]
	if value == nil {
		value = crypto.Keccak256(key[:])
	}
	return append([]byte{}, value...)
}

func (mbs *MockCacheBackingStore) Write(key CacheItemKey, value []byte) {
	mbs.contents[key] = append([]byte{}, value...)
}
