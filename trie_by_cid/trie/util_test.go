package trie_test

import (
	"fmt"
	"math/big"
	"math/rand"
	"testing"

	"github.com/jmoiron/sqlx"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	geth_state "github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/rlp"
	geth_trie "github.com/ethereum/go-ethereum/trie"

	pgipfsethdb "github.com/cerc-io/ipfs-ethdb/v5/postgres/v0"
	"github.com/cerc-io/ipld-eth-statedb/trie_by_cid/helper"
	"github.com/cerc-io/ipld-eth-statedb/trie_by_cid/state"
	"github.com/cerc-io/ipld-eth-statedb/trie_by_cid/trie"
	"github.com/ethereum/go-ethereum/statediff/indexer/database/sql/postgres"
	"github.com/ethereum/go-ethereum/statediff/indexer/ipld"
	"github.com/ethereum/go-ethereum/statediff/test_helpers"
)

type kv struct {
	k []byte
	v int64
}

type kvMap map[string]*kv

type kvs struct {
	k string
	v int64
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

func unpackValue(val []byte) int64 {
	var acct types.StateAccount
	if err := rlp.DecodeBytes(val, &acct); err != nil {
		panic(err)
	}
	return acct.Balance.Int64()
}

func updateTrie(tr *geth_trie.Trie, vals []kvs) (kvMap, error) {
	all := kvMap{}
	for _, val := range vals {
		all[string(val.k)] = &kv{[]byte(val.k), val.v}
		tr.Update([]byte(val.k), packValue(val.v))
	}
	return all, nil
}

func commitTrie(t testing.TB, db *geth_trie.Database, tr *geth_trie.Trie) common.Hash {
	t.Helper()
	root, nodes := tr.Commit(false)
	if err := db.Update(geth_trie.NewWithNodeSet(nodes)); err != nil {
		t.Fatal(err)
	}
	if err := db.Commit(root, false); err != nil {
		t.Fatal(err)
	}
	return root
}

// commit a LevelDB state trie, index to IPLD and return new trie
func indexTrie(t testing.TB, edb ethdb.Database, root common.Hash) *trie.Trie {
	t.Helper()
	dbConfig.Driver = postgres.PGX
	err := helper.IndexChain(dbConfig, geth_state.NewDatabase(edb), common.Hash{}, root)
	if err != nil {
		t.Fatal(err)
	}

	pg_db, err := postgres.ConnectSQLX(ctx, dbConfig)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := TearDownDB(pg_db); err != nil {
			t.Fatal(err)
		}
	})

	ipfs_db := pgipfsethdb.NewDatabase(pg_db, makeCacheConfig(t))
	sdb_db := state.NewDatabase(ipfs_db)
	tr, err := trie.New(common.Hash{}, root, sdb_db.TrieDB(), ipld.MEthStateTrie)
	if err != nil {
		t.Fatal(err)
	}
	return tr
}

// generates a random Geth LevelDB trie of n key-value pairs and corresponding value map
func randomGethTrie(n int, db *geth_trie.Database) (*geth_trie.Trie, kvMap) {
	trie := geth_trie.NewEmpty(db)
	var vals []*kv
	for i := byte(0); i < 100; i++ {
		e := &kv{common.LeftPadBytes([]byte{i}, 32), int64(i)}
		e2 := &kv{common.LeftPadBytes([]byte{i + 10}, 32), int64(i)}
		vals = append(vals, e, e2)
	}
	for i := 0; i < n; i++ {
		k := randBytes(32)
		v := rand.Int63()
		vals = append(vals, &kv{k, v})
	}
	all := kvMap{}
	for _, val := range vals {
		all[string(val.k)] = &kv{[]byte(val.k), val.v}
		trie.Update([]byte(val.k), packValue(val.v))
	}
	return trie, all
}

// generates a random IPLD-indexed trie
func randomTrie(t testing.TB, n int) (*trie.Trie, kvMap) {
	edb := rawdb.NewMemoryDatabase()
	db := geth_trie.NewDatabase(edb)
	orig, vals := randomGethTrie(n, db)
	root := commitTrie(t, db, orig)
	trie := indexTrie(t, edb, root)
	return trie, vals
}

func randBytes(n int) []byte {
	r := make([]byte, n)
	rand.Read(r)
	return r
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
