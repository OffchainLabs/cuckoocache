This implements an on-chain cache index, and corresponding off-chain local node cache, to allow better pricing of accesses to read-only data, 
such as contract code, from the Ethereum state tree. 

### On-chain and off-chain components

On-chain we store a cache index, which tracks a set of items
that are deemed to be in-cache. The on-chain index does not
store the actual data, just a data structure allowing to
determine whether a particular item is in-cache. The state of
the on-chain index is part of the consensus state of the 
Arbitrum chain.

Off-chain we store an actual cache, which will be managed
separately by each node's execution engine. Different nodes 
can have differently sized caches.

The mechanism maintains an *inclusion property* which 
guarantees that if an item is in the on-chain index, then it
is in the cache of every local node. This makes it safe to
charge less gas for accesses to items that are in-cache.

### How to use this

To use the mechanism with an Arbitrum Nitro chain, you need
to implement three interfaces:

`onChainStorage.OnChainStorage` is called by the cache to 
read and write the on-chain storage that backs the on-chain
cache index. The interface provides a key-value store
with both keys and values having type `common.Hash`.

`cacheKeys.LocalNodeCacheKey` is the type of key used to index the 
local node's cache. For example, if cache items are indexed
by `common.Address` then you should provide an implementation
of `cacheKeys.LocalNodeCacheKey` that contains a `common.Address`.
(For `common.Address` this is already provided, as type
type `cacheKeys.AddressLocalCacheKey`)
Implementations of this interface must provide 
a `ToCacheKey()` method that returns a 24-byte digest of 
the key, which will typically be a truncated hash of the key.
The digest should be a hash or similar pseudorandom-like
function of the key, to get the best efficiency from the
cache.

`cacheBackingStore.CacheBackingStore` is called by the cache 
to fetch an item (presumably from some database) that is 
going to be cached.

The main configuration choice is how large the cache will be.
First, choose the capacity of the on-chain index. Then each
node can choose the capacity of its own local cache.

*The capacity of every local node cache must be greater than or
equal to the capacity of the on-chain index.* This is required
in order to guarantee the inclusion property. So you should
choose the capacity of the on-chain index to be the capacity
that you're willing to force upon the minimally-resourced
node.

### Cache replacement policies

The local node cache uses an LRU (Least Recently Used)
replacement policy.

The on-chain index uses a generational cache replacement
policy, in order to reduce the number of accesses to storage.
(This is sufficient to guarantee the inclusion property, because
we can prove that a generational cache of capacity `N` always
contains a subset of the items that would be in an LRU cache
of capacity `N` or greater.)

Generational caching keeps a current generation number, which
increments occasionally. Every item is tagged with the latest
generation in which it was accessed. If the current generation 
is `G` then an item is in-cache if its latest access was in 
generation `G` or `G-1`. If the capacity of the cache is `C`
the generation number increments if an access would cause the
number of in-cache items to be greater than `C` or if it would
cause the number of items in the current generation to be
greater than `3C/4`.

Generational replacement can be seen as an approximation to
LRU. The main advantage of generational over LRU is that
generational makes many fewer writes to storage. With
generational, an access to an item in the latest generation 
requires no changes to the index, and any other access requires
only one item in the index to be created or modified.
By contrast, a typical implementation of LRU would require 
modifying at least three places in storage on every access
(except for accesses to the most recently used item). So we
optimize by using generational in the on-chain index, where
state changes are expensive because the state is in consensus;
but we use LRU in the local node cache where state changes are
cheaper.
