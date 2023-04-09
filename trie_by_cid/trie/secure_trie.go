// Copyright 2015 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package trie

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/statediff/indexer/ipld"
)

// StateTrie wraps a trie with key hashing. In a secure trie, all
// access operations hash the key using keccak256. This prevents
// calling code from creating long chains of nodes that
// increase the access time.
//
// Contrary to a regular trie, a StateTrie can only be created with
// New and must have an attached database.
//
// StateTrie is not safe for concurrent use.
type StateTrie struct {
	trie       Trie
	hashKeyBuf [common.HashLength]byte
}

// NewStateTrie creates a trie with an existing root node from a backing database
// and optional intermediate in-memory node pool.
//
// If root is the zero hash or the sha3 hash of an empty string, the
// trie is initially empty. Otherwise, New will panic if db is nil
// and returns MissingNodeError if the root node cannot be found.
//
// Accessing the trie loads nodes from the database or node pool on demand.
// Loaded nodes are kept around until their 'cache generation' expires.
// A new cache generation is created by each call to Commit.
// cachelimit sets the number of past cache generations to keep.
//
// Retrieves IPLD blocks by CID encoded as "eth-state-trie"
func NewStateTrie(owner common.Hash, root common.Hash, db *Database) (*StateTrie, error) {
	return newStateTrie(owner, root, db, ipld.MEthStateTrie)
}

// NewStorageTrie is identical to NewStateTrie, but retrieves IPLD blocks encoded
// as "eth-storage-trie"
func NewStorageTrie(owner common.Hash, root common.Hash, db *Database) (*StateTrie, error) {
	return newStateTrie(owner, root, db, ipld.MEthStorageTrie)
}

func newStateTrie(owner common.Hash, root common.Hash, db *Database, codec uint64) (*StateTrie, error) {
	if db == nil {
		panic("NewStateTrie called without a database")
	}
	trie, err := New(owner, root, db, codec)
	if err != nil {
		return nil, err
	}
	return &StateTrie{trie: *trie}, nil
}

// TryGet returns the value for key stored in the trie.
// The value bytes must not be modified by the caller.
// If a node was not found in the database, a MissingNodeError is returned.
func (t *StateTrie) TryGet(key []byte) ([]byte, error) {
	return t.trie.TryGet(t.hashKey(key))
}

func (t *StateTrie) TryGetAccount(key []byte) (*types.StateAccount, error) {
	var ret types.StateAccount
	res, err := t.TryGet(key)
	if err != nil {
		// log.Error(fmt.Sprintf("Unhandled trie error: %v", err))
		panic(fmt.Sprintf("Unhandled trie error: %v", err))
		return &ret, err
	}
	if res == nil {
		return nil, nil
	}
	err = rlp.DecodeBytes(res, &ret)
	return &ret, err
}

// Hash returns the root hash of StateTrie. It does not write to the
// database and can be used even if the trie doesn't have one.
func (t *StateTrie) Hash() common.Hash {
	return t.trie.Hash()
}

// hashKey returns the hash of key as an ephemeral buffer.
// The caller must not hold onto the return value because it will become
// invalid on the next call to hashKey or secKey.
func (t *StateTrie) hashKey(key []byte) []byte {
	h := newHasher(false)
	h.sha.Reset()
	h.sha.Write(key)
	h.sha.Read(t.hashKeyBuf[:])
	returnHasherToPool(h)
	return t.hashKeyBuf[:]
}
