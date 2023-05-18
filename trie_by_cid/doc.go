// This package is a near complete copy of go-ethereum/trie and go-ethereum/core/state, modified to use
// a v0 IPFS blockstore as the backing DB, i.e. DB values are indexed by CID rather than hash.
package trie_by_cid
