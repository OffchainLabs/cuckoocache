// Copyright 2024, Offchain Labs, Inc.
// For license information, see https://github.com/OffchainLabs/nitro/blob/master/LICENSE

package cacheBackingStore

import (
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/offchainlabs/cuckoocache/cacheKeys"
)

type CacheBackingStore[KeyType cacheKeys.LocalNodeCacheKey] struct {
	Read func(key KeyType) []byte
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
