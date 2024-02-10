package generational_cache

import "encoding/binary"

type CacheBackingStore interface {
	Read(key CacheItemKey) *CacheItemValue
}

type MockCacheBackingStore struct {
	contents map[CacheItemKey]*CacheItemValue
}

func NewMockBackingStore() CacheBackingStore {
	return &MockCacheBackingStore{contents: make(map[CacheItemKey]*CacheItemValue)}
}

func (mbs *MockCacheBackingStore) Read(key CacheItemKey) *CacheItemValue {
	value := mbs.contents[key]
	if value == nil {
		val := binary.LittleEndian.Uint64(key.Bytes()[0:8])
		value = (*CacheItemValue)(&val)
	}
	return value
}

func (mbs *MockCacheBackingStore) Write(key CacheItemKey, value *CacheItemValue) {
	mbs.contents[key] = value
}
