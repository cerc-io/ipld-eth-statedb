# ipld-eth-statedb

Implementation of the geth [vm.StateDB](https://github.com/ethereum/go-ethereum/blob/master/core/vm/interface.go#L28) on top of
[ipld-eth-db](https://github.com/cerc-io/ipld-eth-db), to allow us to plug into existing EVM functionality. Analogous to
[ipfs-ethdb](https://github.com/cerc-io/ipfs-ethdb) but at one database abstraction level higher. This allows us to
bypass the trie-traversal access pattern normally used by the EVM (and which ipfs-ethdb allows us to replicate ontop of our
Postgres IPLD blockstore in the "public.blocks" table) and access state and storage directly in the "state_cids" and
"storage_cids" tables.


Note: "IPFS" is chosen in the name of "ipfs-ethdb" as it can function through an IPFS BlockService abstraction or directly ontop of an IPLD blockstore, whereas this repository
is very tightly coupled to the schema in ipld-eth-db.

The top-level package contains the implementation of the `vm.StateDB` interface that accesses state directly using the
`state_cids` and `storage_cids` tables in ipld-eth-db. The `trie_by_cid` package contains an alternative implementation
which accesses state in `ipld.blocks` through the typical trie traversal access pattern (using CIDs instead of raw
keccak256 hashes), it is used for benchmarking and for functionality which requires performing a trie traversal
(things which must collect intermediate nodes, e.g. `eth_getProof` and `eth_getSlice`).
