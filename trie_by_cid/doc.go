// This package is a near complete copy of go-ethereum/trie and go-ethereum/core/state, modified to use
// a v0 IPFS blockstore as the backing DB, i.e. DB values are indexed by CID rather than hash.
package trie_by_cid

// Note: This is written so as to minimize the difference from the geth packages to allow easier
// patching during upgrades.
//
// Changes to the original functionality should be marked with (ipld-eth-statedb change)
// The main changes are within the triedb/hashdb package.
//
// Writes are currently disabled and will panic, but they may be useful for testing if possible.
//
// Tests from the original source are preserved as much as reasonably possible.  Memory DBs are
// replaced with Postgres connections within the state package, but not the trie packages, since
// they mostly check the structural consistency of the trie.
