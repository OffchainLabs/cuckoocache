package cuckoo_cache

import (
	"github.com/stretchr/testify/assert"
	"offchainlabs.com/cuckoo-cache/cacheKeys"
	"testing"
)

func TestEvaluation(t *testing.T) {
	on, local := EvaluateOnData(32, 64, []cacheKeys.Uint64LocalCacheKey{})
	assert.Equal(t, on, uint64(0))
	assert.Equal(t, local, uint64(0))

	accesses := []cacheKeys.Uint64LocalCacheKey{}
	for i := 0; i < 571; i++ {
		accesses = append(accesses, cacheKeys.NewUint64LocalCacheKey(uint64(i)))
	}
	on, local = EvaluateOnData(32, 64, accesses)
	assert.Equal(t, on, uint64(0))
	assert.Equal(t, local, uint64(0))

	tempAccesses := []cacheKeys.Uint64LocalCacheKey{}
	for i := 0; i < 16; i++ {
		tempAccesses = append(tempAccesses, cacheKeys.NewUint64LocalCacheKey(uint64(i)))
	}
	accesses = append(tempAccesses, tempAccesses...)
	accesses = append(accesses, tempAccesses...)
	on, local = EvaluateOnData(32, 64, accesses)
	assert.Equal(t, on, uint64(32))
	assert.Equal(t, local, uint64(32))

	tempAccesses = []cacheKeys.Uint64LocalCacheKey{}
	for i := 0; i < 32; i++ {
		tempAccesses = append(tempAccesses, cacheKeys.NewUint64LocalCacheKey(uint64(i)))
	}
	accesses = append(tempAccesses, tempAccesses...)
	accesses = append(accesses, tempAccesses...)
	on, local = EvaluateOnData(32, 64, accesses)
	assert.Equal(t, on, uint64(64))
	assert.Equal(t, local, uint64(64))
}
