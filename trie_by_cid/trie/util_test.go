package trie

import (
	"bytes"
	"context"
	"fmt"
	"math/big"
	"math/rand"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	gethstate "github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/rlp"
	gethtrie "github.com/ethereum/go-ethereum/trie"
	"github.com/jmoiron/sqlx"

	pgipfsethdb "github.com/cerc-io/ipfs-ethdb/v5/postgres/v0"
	"github.com/ethereum/go-ethereum/statediff/indexer/database/sql/postgres"
	"github.com/ethereum/go-ethereum/statediff/test_helpers"

	"github.com/cerc-io/ipld-eth-statedb/internal"
	"github.com/cerc-io/ipld-eth-statedb/trie_by_cid/helper"
)

var (
	dbConfig, _ = postgres.DefaultConfig.WithEnv()
	trieConfig  = Config{Cache: 256}
)

type kvi struct {
	k []byte
	v int64
}

type kvMap map[string]*kvi

type kvsi struct {
	k string
	v int64
}

// NewAccountTrie is a shortcut to create a trie using the StateTrieCodec (ie. IPLD MEthStateTrie codec).
func NewAccountTrie(id *ID, db NodeReader) (*Trie, error) {
	return New(id, db, StateTrieCodec)
}

// makeTestTrie create a sample test trie to test node-wise reconstruction.
func makeTestTrie(t testing.TB) (*Database, *StateTrie, map[string][]byte) {
	// Create an empty trie
	triedb := NewDatabase(rawdb.NewMemoryDatabase())
	trie, err := NewStateTrie(TrieID(common.Hash{}), triedb, StateTrieCodec)
	if err != nil {
		t.Fatal(err)
	}

	// Fill it with some arbitrary data
	content := make(map[string][]byte)
	for i := byte(0); i < 255; i++ {
		// Map the same data under multiple keys
		key, val := common.LeftPadBytes([]byte{1, i}, 32), []byte{i}
		content[string(key)] = val
		trie.Update(key, val)

		key, val = common.LeftPadBytes([]byte{2, i}, 32), []byte{i}
		content[string(key)] = val
		trie.Update(key, val)

		// Add some other data to inflate the trie
		for j := byte(3); j < 13; j++ {
			key, val = common.LeftPadBytes([]byte{j, i}, 32), []byte{j, i}
			content[string(key)] = val
			trie.Update(key, val)
		}
	}
	root, nodes := trie.Commit(false)
	if err := triedb.Update(NewWithNodeSet(nodes)); err != nil {
		panic(fmt.Errorf("failed to commit db %v", err))
	}
	// Re-create the trie based on the new state
	trie, err = NewStateTrie(TrieID(root), triedb, StateTrieCodec)
	if err != nil {
		t.Fatal(err)
	}
	return triedb, trie, content
}

func forHashedNodes(tr *Trie) map[string][]byte {
	var (
		it    = tr.NodeIterator(nil)
		nodes = make(map[string][]byte)
	)
	for it.Next(true) {
		if it.Hash() == (common.Hash{}) {
			continue
		}
		nodes[string(it.Path())] = common.CopyBytes(it.NodeBlob())
	}
	return nodes
}

func diffTries(trieA, trieB *Trie) (map[string][]byte, map[string][]byte, map[string][]byte) {
	var (
		nodesA = forHashedNodes(trieA)
		nodesB = forHashedNodes(trieB)
		inA    = make(map[string][]byte) // hashed nodes in trie a but not b
		inB    = make(map[string][]byte) // hashed nodes in trie b but not a
		both   = make(map[string][]byte) // hashed nodes in both tries but different value
	)
	for path, blobA := range nodesA {
		if blobB, ok := nodesB[path]; ok {
			if bytes.Equal(blobA, blobB) {
				continue
			}
			both[path] = blobA
			continue
		}
		inA[path] = blobA
	}
	for path, blobB := range nodesB {
		if _, ok := nodesA[path]; ok {
			continue
		}
		inB[path] = blobB
	}
	return inA, inB, both
}

func packValue(val int64) []byte {
	acct := &types.StateAccount{
		Balance:  big.NewInt(val),
		CodeHash: test_helpers.NullCodeHash.Bytes(),
		Root:     test_helpers.EmptyContractRoot,
	}
	acct_rlp, err := rlp.EncodeToBytes(acct)
	if err != nil {
		panic(err)
	}
	return acct_rlp
}

// func unpackValue(val []byte) int64 {
// 	var acct types.StateAccount
// 	if err := rlp.DecodeBytes(val, &acct); err != nil {
// 		panic(err)
// 	}
// 	return acct.Balance.Int64()
// }

func updateTrie(tr *gethtrie.Trie, vals []kvsi) (kvMap, error) {
	all := kvMap{}
	for _, val := range vals {
		all[string(val.k)] = &kvi{[]byte(val.k), val.v}
		tr.Update([]byte(val.k), packValue(val.v))
	}
	return all, nil
}

func commitTrie(t testing.TB, db *gethtrie.Database, tr *gethtrie.Trie) common.Hash {
	t.Helper()
	root, nodes := tr.Commit(false)
	if err := db.Update(gethtrie.NewWithNodeSet(nodes)); err != nil {
		t.Fatal(err)
	}
	if err := db.Commit(root, false); err != nil {
		t.Fatal(err)
	}
	return root
}

func makePgIpfsEthDB(t testing.TB) ethdb.Database {
	pg_db, err := postgres.ConnectSQLX(context.Background(), dbConfig)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := TearDownDB(pg_db); err != nil {
			t.Fatal(err)
		}
	})
	return pgipfsethdb.NewDatabase(pg_db, internal.MakeCacheConfig(t))
}

// commit a LevelDB state trie, index to IPLD and return new trie
func indexTrie(t testing.TB, edb ethdb.Database, root common.Hash) *Trie {
	t.Helper()
	dbConfig.Driver = postgres.PGX
	err := helper.IndexStateDiff(dbConfig, gethstate.NewDatabase(edb), common.Hash{}, root)
	if err != nil {
		t.Fatal(err)
	}

	ipfs_db := makePgIpfsEthDB(t)
	tr, err := New(TrieID(root), NewDatabase(ipfs_db), StateTrieCodec)
	if err != nil {
		t.Fatal(err)
	}
	return tr
}

// generates a random Geth LevelDB trie of n key-value pairs and corresponding value map
func randomGethTrie(n int, db *gethtrie.Database) (*gethtrie.Trie, kvMap) {
	trie := gethtrie.NewEmpty(db)
	var vals []*kvi
	for i := byte(0); i < 100; i++ {
		e := &kvi{common.LeftPadBytes([]byte{i}, 32), int64(i)}
		e2 := &kvi{common.LeftPadBytes([]byte{i + 10}, 32), int64(i)}
		vals = append(vals, e, e2)
	}
	for i := 0; i < n; i++ {
		k := randBytes(32)
		v := rand.Int63()
		vals = append(vals, &kvi{k, v})
	}
	all := kvMap{}
	for _, val := range vals {
		all[string(val.k)] = &kvi{[]byte(val.k), val.v}
		trie.Update([]byte(val.k), packValue(val.v))
	}
	return trie, all
}

// TearDownDB is used to tear down the watcher dbs after tests
func TearDownDB(db *sqlx.DB) error {
	tx, err := db.Beginx()
	if err != nil {
		return err
	}
	statements := []string{
		`DELETE FROM nodes`,
		`DELETE FROM ipld.blocks`,
		`DELETE FROM eth.header_cids`,
		`DELETE FROM eth.uncle_cids`,
		`DELETE FROM eth.transaction_cids`,
		`DELETE FROM eth.receipt_cids`,
		`DELETE FROM eth.state_cids`,
		`DELETE FROM eth.storage_cids`,
		`DELETE FROM eth.log_cids`,
		`DELETE FROM eth_meta.watched_addresses`,
	}
	for _, stm := range statements {
		if _, err = tx.Exec(stm); err != nil {
			return fmt.Errorf("error executing `%s`: %w", stm, err)
		}
	}
	return tx.Commit()
}
