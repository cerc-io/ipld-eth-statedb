// Copyright 2014 The go-ethereum Authors
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

package trie_test

import (
	"fmt"
	"math/big"
	"math/rand"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
	geth_trie "github.com/ethereum/go-ethereum/trie"

	"github.com/cerc-io/ipld-eth-statedb/trie_by_cid/trie"
)

func TestTrieEmpty(t *testing.T) {
	trie := trie.NewEmpty(trie.NewDatabase(rawdb.NewMemoryDatabase()))
	res := trie.Hash()
	exp := types.EmptyRootHash
	if res != exp {
		t.Errorf("expected %x got %x", exp, res)
	}
}

func TestTrieMissingRoot(t *testing.T) {
	root := common.HexToHash("0beec7b5ea3f0fdbc95d0dd47f3c5bc275da8a33")
	tr, err := newStateTrie(root, trie.NewDatabase(rawdb.NewMemoryDatabase()))
	if tr != nil {
		t.Error("New returned non-nil trie for invalid root")
	}
	if _, ok := err.(*trie.MissingNodeError); !ok {
		t.Errorf("New returned wrong error: %v", err)
	}
}

func TestTrieBasic(t *testing.T) {
	edb := rawdb.NewMemoryDatabase()
	db := geth_trie.NewDatabase(edb)
	origtrie := geth_trie.NewEmpty(db)
	origtrie.Update([]byte("foo"), packValue(842))
	expected := commitTrie(t, db, origtrie)
	tr := indexTrie(t, edb, expected)
	got := tr.Hash()
	if expected != got {
		t.Errorf("got %x expected %x", got, expected)
	}
	checkValue(t, tr, []byte("foo"))
}

func TestTrieTiny(t *testing.T) {
	// Create a realistic account trie to hash
	_, accounts := makeAccounts(5)
	edb := rawdb.NewMemoryDatabase()
	db := geth_trie.NewDatabase(edb)
	origtrie := geth_trie.NewEmpty(db)

	type testCase struct {
		key, account []byte
		root         common.Hash
	}
	cases := []testCase{
		{
			common.Hex2Bytes("0000000000000000000000000000000000000000000000000000000000001337"),
			accounts[3],
			common.HexToHash("8c6a85a4d9fda98feff88450299e574e5378e32391f75a055d470ac0653f1005"),
		}, {
			common.Hex2Bytes("0000000000000000000000000000000000000000000000000000000000001338"),
			accounts[4],
			common.HexToHash("ec63b967e98a5720e7f720482151963982890d82c9093c0d486b7eb8883a66b1"),
		}, {
			common.Hex2Bytes("0000000000000000000000000000000000000000000000000000000000001339"),
			accounts[4],
			common.HexToHash("0608c1d1dc3905fa22204c7a0e43644831c3b6d3def0f274be623a948197e64a"),
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("case %d", i), func(t *testing.T) {
			origtrie.Update(tc.key, tc.account)
			trie := indexTrie(t, edb, commitTrie(t, db, origtrie))
			if exp, root := tc.root, trie.Hash(); exp != root {
				t.Errorf("got %x, exp %x", root, exp)
			}
			checkValue(t, trie, tc.key)
		})
	}
}

func TestTrieMedium(t *testing.T) {
	// Create a realistic account trie to hash
	addresses, accounts := makeAccounts(1000)
	edb := rawdb.NewMemoryDatabase()
	db := geth_trie.NewDatabase(edb)
	origtrie := geth_trie.NewEmpty(db)
	var keys [][]byte
	for i := 0; i < len(addresses); i++ {
		key := crypto.Keccak256(addresses[i][:])
		if i%50 == 0 {
			keys = append(keys, key)
		}
		origtrie.Update(key, accounts[i])
	}
	tr := indexTrie(t, edb, commitTrie(t, db, origtrie))

	root := tr.Hash()
	exp := common.HexToHash("72f9d3f3fe1e1dd7b8936442e7642aef76371472d94319900790053c493f3fe6")
	if exp != root {
		t.Errorf("got %x, exp %x", root, exp)
	}

	for _, key := range keys {
		checkValue(t, tr, key)
	}
}

// Make deterministically random accounts
func makeAccounts(size int) (addresses [][20]byte, accounts [][]byte) {
	random := rand.New(rand.NewSource(0))
	addresses = make([][20]byte, size)
	for i := 0; i < len(addresses); i++ {
		data := make([]byte, 20)
		random.Read(data)
		copy(addresses[i][:], data)
	}
	accounts = make([][]byte, len(addresses))
	for i := 0; i < len(accounts); i++ {
		var (
			nonce = uint64(random.Int63())
			root  = types.EmptyRootHash
			code  = crypto.Keccak256(nil)
		)
		// The big.Rand function is not deterministic with regards to 64 vs 32 bit systems,
		// and will consume different amount of data from the rand source.
		// balance = new(big.Int).Rand(random, new(big.Int).Exp(common.Big2, common.Big256, nil))
		// Therefore, we instead just read via byte buffer
		numBytes := random.Uint32() % 33 // [0, 32] bytes
		balanceBytes := make([]byte, numBytes)
		random.Read(balanceBytes)
		balance := new(big.Int).SetBytes(balanceBytes)
		acct := &types.StateAccount{Nonce: nonce, Balance: balance, Root: root, CodeHash: code}
		data, _ := rlp.EncodeToBytes(acct)
		accounts[i] = data
	}
	return addresses, accounts
}

func checkValue(t *testing.T, tr *trie.Trie, key []byte) {
	val, err := tr.TryGet(key)
	if err != nil {
		t.Fatalf("error getting node: %s", err)
	}
	if len(val) == 0 {
		t.Errorf("failed to get value for %x", key)
	}
}
