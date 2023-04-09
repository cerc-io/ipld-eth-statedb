// Copyright 2018 The go-ethereum Authors
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
	"errors"

	"github.com/VictoriaMetrics/fastcache"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/trie"
)

type CidBytes = []byte

func isEmpty(key CidBytes) bool {
	return len(key) == 0
}

// Database is an intermediate read-only layer between the trie data structures and
// the disk database. This trie Database is thread safe in providing individual,
// independent node access.
type Database struct {
	diskdb ethdb.KeyValueStore // Persistent storage for matured trie nodes
	cleans *fastcache.Cache    // GC friendly memory cache of clean node RLPs
}

// Config defines all necessary options for database.
// (re-export)
type Config = trie.Config

// NewDatabase creates a new trie database to store ephemeral trie content before
// its written out to disk or garbage collected. No read cache is created, so all
// data retrievals will hit the underlying disk database.
func NewDatabase(diskdb ethdb.KeyValueStore) *Database {
	return NewDatabaseWithConfig(diskdb, nil)
}

// NewDatabaseWithConfig creates a new trie database to store ephemeral trie content
// before it's written out to disk or garbage collected. It also acts as a read cache
// for nodes loaded from disk.
func NewDatabaseWithConfig(diskdb ethdb.KeyValueStore, config *Config) *Database {
	var cleans *fastcache.Cache
	if config != nil && config.Cache > 0 {
		if config.Journal == "" {
			cleans = fastcache.New(config.Cache * 1024 * 1024)
		} else {
			cleans = fastcache.LoadFromFileOrNew(config.Journal, config.Cache*1024*1024)
		}
	}
	db := &Database{
		diskdb: diskdb,
		cleans: cleans,
	}
	return db
}

// DiskDB retrieves the persistent storage backing the trie database.
func (db *Database) DiskDB() ethdb.KeyValueStore {
	return db.diskdb
}

// node retrieves a cached trie node from memory, or returns nil if none can be
// found in the memory cache.
func (db *Database) node(key CidBytes) (node, error) {
	// Retrieve the node from the clean cache if available
	if db.cleans != nil {
		if enc := db.cleans.Get(nil, key); enc != nil {
			// The returned value from cache is in its own copy,
			// safe to use mustDecodeNodeUnsafe for decoding.
			return decodeNodeUnsafe(key, enc)
		}
	}

	// Content unavailable in memory, attempt to retrieve from disk
	enc, err := db.diskdb.Get(key)
	if err != nil {
		return nil, err
	}
	if enc == nil {
		return nil, nil
	}
	if db.cleans != nil {
		db.cleans.Set(key, enc)
	}
	// The returned value from database is in its own copy,
	// safe to use mustDecodeNodeUnsafe for decoding.
	return decodeNodeUnsafe(key, enc)
}

// Node retrieves an encoded cached trie node from memory. If it cannot be found
// cached, the method queries the persistent database for the content.
func (db *Database) Node(key CidBytes) ([]byte, error) {
	// It doesn't make sense to retrieve the metaroot
	if isEmpty(key) {
		return nil, errors.New("not found")
	}
	// Retrieve the node from the clean cache if available
	if db.cleans != nil {
		if enc := db.cleans.Get(nil, key); enc != nil {
			return enc, nil
		}
	}

	// Content unavailable in memory, attempt to retrieve from disk
	enc, err := db.diskdb.Get(key)
	if err != nil {
		return nil, err
	}

	if len(enc) != 0 {
		if db.cleans != nil {
			db.cleans.Set(key[:], enc)
		}
		return enc, nil
	}
	return nil, errors.New("not found")
}
