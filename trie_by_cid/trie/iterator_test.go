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
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/statediff/indexer/database/sql/postgres"
	geth_trie "github.com/ethereum/go-ethereum/trie"

	pgipfsethdb "github.com/cerc-io/ipfs-ethdb/v5/postgres/v0"
	"github.com/cerc-io/ipld-eth-statedb/trie_by_cid/trie"
)

var (
	cacheConfig = pgipfsethdb.CacheConfig{
		Name:           "db",
		Size:           3000000, // 3MB
		ExpiryDuration: time.Hour,
	}
	dbConfig, _ = postgres.DefaultConfig.WithEnv()
	trieConfig  = trie.Config{Cache: 256}

	ctx = context.Background()
)

var testdata1 = []kvs{
	{"barb", 0},
	{"bard", 1},
	{"bars", 2},
	{"bar", 3},
	{"fab", 4},
	{"food", 5},
	{"foos", 6},
	{"foo", 7},
}

func TestEmptyIterator(t *testing.T) {
	trie := trie.NewEmpty(trie.NewDatabase(rawdb.NewMemoryDatabase()))
	iter := trie.NodeIterator(nil)

	seen := make(map[string]struct{})
	for iter.Next(true) {
		seen[string(iter.Path())] = struct{}{}
	}
	if len(seen) != 0 {
		t.Fatal("Unexpected trie node iterated")
	}
}

func TestIterator(t *testing.T) {
	edb := rawdb.NewMemoryDatabase()
	db := geth_trie.NewDatabase(edb)
	origtrie := geth_trie.NewEmpty(db)
	vals := []kvs{
		{"one", 1},
		{"two", 2},
		{"three", 3},
		{"four", 4},
		{"five", 5},
		{"ten", 10},
	}
	all, err := updateTrie(origtrie, vals)
	if err != nil {
		t.Fatal(err)
	}
	// commit and index data
	root := commitTrie(t, db, origtrie)
	tr := indexTrie(t, edb, root)

	found := make(map[string]int64)
	it := trie.NewIterator(tr.NodeIterator(nil))
	for it.Next() {
		found[string(it.Key)] = unpackValue(it.Value)
	}

	if len(found) != len(all) {
		t.Errorf("number of iterated values do not match: want %d, found %d", len(all), len(found))
	}
	for k, kv := range all {
		if found[k] != kv.v {
			t.Errorf("iterator value mismatch for %s: got %q want %q", k, found[k], kv.v)
		}
	}
}

func checkIteratorOrder(want []kvs, it *trie.Iterator) error {
	for it.Next() {
		if len(want) == 0 {
			return fmt.Errorf("didn't expect any more values, got key %q", it.Key)
		}
		if !bytes.Equal(it.Key, []byte(want[0].k)) {
			return fmt.Errorf("wrong key: got %q, want %q", it.Key, want[0].k)
		}
		want = want[1:]
	}
	if len(want) > 0 {
		return fmt.Errorf("iterator ended early, want key %q", want[0])
	}
	return nil
}

func TestIteratorSeek(t *testing.T) {
	edb := rawdb.NewMemoryDatabase()
	db := geth_trie.NewDatabase(edb)
	orig := geth_trie.NewEmpty(geth_trie.NewDatabase(rawdb.NewMemoryDatabase()))
	if _, err := updateTrie(orig, testdata1); err != nil {
		t.Fatal(err)
	}
	root := commitTrie(t, db, orig)
	tr := indexTrie(t, edb, root)

	// Seek to the middle.
	it := trie.NewIterator(tr.NodeIterator([]byte("fab")))
	if err := checkIteratorOrder(testdata1[4:], it); err != nil {
		t.Fatal(err)
	}

	// Seek to a non-existent key.
	it = trie.NewIterator(tr.NodeIterator([]byte("barc")))
	if err := checkIteratorOrder(testdata1[1:], it); err != nil {
		t.Fatal(err)
	}

	// Seek beyond the end.
	it = trie.NewIterator(tr.NodeIterator([]byte("z")))
	if err := checkIteratorOrder(nil, it); err != nil {
		t.Fatal(err)
	}
}

// returns a cache config with unique name (groupcache names are global)
func makeCacheConfig(t testing.TB) pgipfsethdb.CacheConfig {
	return pgipfsethdb.CacheConfig{
		Name:           t.Name(),
		Size:           3000000, // 3MB
		ExpiryDuration: time.Hour,
	}
}
