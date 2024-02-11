package generational_cache

import "encoding/binary"

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
		val := binary.LittleEndian.Uint64(key.Bytes()[0:8])
		value = binary.LittleEndian.AppendUint64([]byte{}, val)
	}
	return append([]byte{}, value...)
}

func (mbs *MockCacheBackingStore) Write(key CacheItemKey, value []byte) {
	mbs.contents[key] = append([]byte{}, value...)
}
