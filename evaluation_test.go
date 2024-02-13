package generational_cache

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestEvaluation(t *testing.T) {
	on, local := EvaluateOnData(32, 64, []Uint64LocalCacheKey{})
	assert.Equal(t, on, uint64(0))
	assert.Equal(t, local, uint64(0))

	accesses := []Uint64LocalCacheKey{}
	for i := 0; i < 571; i++ {
		accesses = append(accesses, Uint64LocalCacheKey{uint64(i)})
	}
	on, local = EvaluateOnData(32, 64, accesses)
	assert.Equal(t, on, uint64(0))
	assert.Equal(t, local, uint64(0))

	tempAccesses := []Uint64LocalCacheKey{}
	for i := 0; i < 16; i++ {
		tempAccesses = append(tempAccesses, Uint64LocalCacheKey{uint64(i)})
	}
	accesses = append(tempAccesses, tempAccesses...)
	accesses = append(accesses, tempAccesses...)
	on, local = EvaluateOnData(32, 64, accesses)
	assert.Equal(t, on, uint64(32))
	assert.Equal(t, local, uint64(32))

	tempAccesses = []Uint64LocalCacheKey{}
	for i := 0; i < 32; i++ {
		tempAccesses = append(tempAccesses, Uint64LocalCacheKey{uint64(i)})
	}
	accesses = append(tempAccesses, tempAccesses...)
	accesses = append(accesses, tempAccesses...)
	on, local = EvaluateOnData(32, 64, accesses)
	assert.Equal(t, on, uint64(32))
	assert.Equal(t, local, uint64(64))
}
