package cacheKeys

import (
	"encoding/binary"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"offchainlabs.com/cuckoo-cache/onChainIndex"
)

type LocalNodeCacheKey interface {
	ToCacheKey() [24]byte
	comparable
}

type Uint64LocalCacheKey struct {
	key      uint64
	cacheKey onChainIndex.CacheItemKey
}

func NewUint64LocalCacheKey(key uint64) Uint64LocalCacheKey {
	cacheKey := [24]byte{}
	copy(cacheKey[:], crypto.Keccak256(binary.LittleEndian.AppendUint64([]byte{}, key)))
	return Uint64LocalCacheKey{key, cacheKey}
}

func (ukey Uint64LocalCacheKey) ToCacheKey() [24]byte {
	return ukey.cacheKey
}

type AddressLocalCacheKey struct {
	address common.Address
}

func (key AddressLocalCacheKey) ToCacheKey() [24]byte {
	// addresses are generated by hashing, so no need to hash again
	// we fill the last four bytes with duplicates of the first four, which might be useful
	ret := [24]byte{}
	copy(ret[0:20], key.address.Bytes())
	copy(ret[20:24], ret[0:4])
	return ret
}
