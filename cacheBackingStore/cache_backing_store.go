package cacheBackingStore

import (
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/offchainlabs/cuckoo-cache/cacheKeys"
	"github.com/offchainlabs/cuckoo-cache/onChainIndex"
)

type CacheBackingStore[KeyType cacheKeys.LocalNodeCacheKey] struct {
	Read func(key KeyType) []byte
}

type MockCacheBackingStore struct {
	contents map[onChainIndex.CacheItemKey][]byte
}

func NewMockBackingStore[KeyType cacheKeys.LocalNodeCacheKey]() CacheBackingStore[KeyType] {
	contents := make(map[KeyType][]byte)
	return CacheBackingStore[KeyType]{
		Read: func(key KeyType) []byte {
			value := contents[key]
			if value == nil {
				buf := key.ToCacheKey()
				value = crypto.Keccak256(buf[:])
			}
			return append([]byte{}, value...)
		},
	}
}
