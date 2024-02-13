package cuckoo_cache

import "encoding/binary"

type LocalNodeCacheKey interface {
	ToCacheKey() [24]byte
	comparable
}

type Uint64LocalCacheKey struct {
	key uint64
}

func (ukey Uint64LocalCacheKey) ToCacheKey() [24]byte {
	ret := [24]byte{}
	copy(ret[:], binary.LittleEndian.AppendUint64([]byte{}, ukey.key))
	return ret
}
