# ipld-eth-statedb

This contains multiple implementations of the geth [vm.StateDB](https://github.com/ethereum/go-ethereum/blob/master/core/vm/interface.go#L28) and supporting types for different use cases.

## Package `direct_by_leaf`

A read-only implementation which uses the schema defined in [ipld-eth-db](https://github.com/cerc-io/ipld-eth-db), to allow direct querying by state and storage node leaf key, bypassing the trie-traversal access pattern normally used by the EVM.
This operates at one abstraction level higher than [ipfs-ethdb](https://github.com/cerc-io/ipfs-ethdb), and is suitable for providing fast state reads.

## Package `trie_by_cid`

A read-write implementation which uses a Postgres IPLD v0 Blockstore as the backing `ethdb.Database`. Specifically this passes v1 CIDs of Keccak-256 hashes to the database in place of plain hashes, and can be used in combination with a [ipfs-ethdb/postgres/v0](https://github.com/cerc-io/ipfs-ethdb/tree/v5/postgres/v0) `Database` instance, or an IPLD BlockService providing a v0 Blockstore.

This implementation uses trie traversal to access state, and is capable of computing state root hashes and performing full EVM operations. It's also suitable for scenarios requiring trie traversal and access to intermediate state nodes (e.g. `eth_getProof` and `eth_getSlice` on [ipld-eth-server](https://github.com/cerc-io/ipld-eth-server)).
