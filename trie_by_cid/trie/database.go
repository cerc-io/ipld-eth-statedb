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
	"runtime"
	"sync"
	"time"

	"github.com/VictoriaMetrics/fastcache"
	"github.com/cerc-io/ipld-eth-statedb/internal"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/trie"
	log "github.com/sirupsen/logrus"
)

// Database is an intermediate write layer between the trie data structures and
// the disk database. The aim is to accumulate trie writes in-memory and only
// periodically flush a couple tries to disk, garbage collecting the remainder.
//
// Note, the trie Database is **not** thread safe in its mutations, but it **is**
// thread safe in providing individual, independent node access. The rationale
// behind this split design is to provide read access to RPC handlers and sync
// servers even while the trie is executing expensive garbage collection.
type Database struct {
	diskdb ethdb.Database // Persistent storage for matured trie nodes

	cleans  *fastcache.Cache            // GC friendly memory cache of clean node RLPs
	dirties map[common.Hash]*cachedNode // Data and references relationships of dirty trie nodes
	oldest  common.Hash                 // Oldest tracked node, flush-list head
	newest  common.Hash                 // Newest tracked node, flush-list tail

	gctime  time.Duration      // Time spent on garbage collection since last commit
	gcnodes uint64             // Nodes garbage collected since last commit
	gcsize  common.StorageSize // Data storage garbage collected since last commit

	flushtime  time.Duration      // Time spent on data flushing since last commit
	flushnodes uint64             // Nodes flushed since last commit
	flushsize  common.StorageSize // Data storage flushed since last commit

	dirtiesSize  common.StorageSize // Storage size of the dirty node cache (exc. metadata)
	childrenSize common.StorageSize // Storage size of the external children tracking
	preimages    *preimageStore     // The store for caching preimages

	lock sync.RWMutex
}

// Config defines all necessary options for database.
// (re-export)
type Config = trie.Config

// NewDatabase creates a new trie database to store ephemeral trie content before
// its written out to disk or garbage collected. No read cache is created, so all
// data retrievals will hit the underlying disk database.
func NewDatabase(diskdb ethdb.Database) *Database {
	return NewDatabaseWithConfig(diskdb, nil)
}

// NewDatabaseWithConfig creates a new trie database to store ephemeral trie content
// before its written out to disk or garbage collected. It also acts as a read cache
// for nodes loaded from disk.
func NewDatabaseWithConfig(diskdb ethdb.Database, config *Config) *Database {
	var cleans *fastcache.Cache
	if config != nil && config.Cache > 0 {
		if config.Journal == "" {
			cleans = fastcache.New(config.Cache * 1024 * 1024)
		} else {
			cleans = fastcache.LoadFromFileOrNew(config.Journal, config.Cache*1024*1024)
		}
	}
	var preimage *preimageStore
	if config != nil && config.Preimages {
		preimage = newPreimageStore(diskdb)
	}
	db := &Database{
		diskdb: diskdb,
		cleans: cleans,
		dirties: map[common.Hash]*cachedNode{{}: {
			children: make(map[common.Hash]uint16),
		}},
		preimages: preimage,
	}
	return db
}

// insert inserts a simplified trie node into the memory database.
// All nodes inserted by this function will be reference tracked
// and in theory should only used for **trie nodes** insertion.
func (db *Database) insert(hash common.Hash, size int, node node) {
	// If the node's already cached, skip
	if _, ok := db.dirties[hash]; ok {
		return
	}
	memcacheDirtyWriteMeter.Mark(int64(size))

	// Create the cached entry for this node
	entry := &cachedNode{
		node:      node,
		size:      uint16(size),
		flushPrev: db.newest,
	}
	entry.forChilds(func(child common.Hash) {
		if c := db.dirties[child]; c != nil {
			c.parents++
		}
	})
	db.dirties[hash] = entry

	// Update the flush-list endpoints
	if db.oldest == (common.Hash{}) {
		db.oldest, db.newest = hash, hash
	} else {
		db.dirties[db.newest].flushNext, db.newest = hash, hash
	}
	db.dirtiesSize += common.StorageSize(common.HashLength + entry.size)
}

// Node retrieves an encoded cached trie node from memory. If it cannot be found
// cached, the method queries the persistent database for the content.
func (db *Database) Node(hash common.Hash, codec uint64) ([]byte, error) {
	// It doesn't make sense to retrieve the metaroot
	if hash == (common.Hash{}) {
		return nil, errors.New("not found")
	}
	// Retrieve the node from the clean cache if available
	if db.cleans != nil {
		if enc := db.cleans.Get(nil, hash[:]); enc != nil {
			memcacheCleanHitMeter.Mark(1)
			memcacheCleanReadMeter.Mark(int64(len(enc)))
			return enc, nil
		}
	}
	// Retrieve the node from the dirty cache if available
	db.lock.RLock()
	dirty := db.dirties[hash]
	db.lock.RUnlock()

	if dirty != nil {
		memcacheDirtyHitMeter.Mark(1)
		memcacheDirtyReadMeter.Mark(int64(dirty.size))
		return dirty.rlp(), nil
	}
	memcacheDirtyMissMeter.Mark(1)

	// Content unavailable in memory, attempt to retrieve from disk
	cid, err := internal.Keccak256ToCid(codec, hash[:])
	if err != nil {
		return nil, err
	}
	enc, err := db.diskdb.Get(cid.Bytes())
	if err != nil {
		return nil, err
	}
	if len(enc) != 0 {
		if db.cleans != nil {
			db.cleans.Set(hash[:], enc)
			memcacheCleanMissMeter.Mark(1)
			memcacheCleanWriteMeter.Mark(int64(len(enc)))
		}
		return enc, nil
	}
	return nil, errors.New("not found")
}

// Nodes retrieves the hashes of all the nodes cached within the memory database.
// This method is extremely expensive and should only be used to validate internal
// states in test code.
func (db *Database) Nodes() []common.Hash {
	db.lock.RLock()
	defer db.lock.RUnlock()

	var hashes = make([]common.Hash, 0, len(db.dirties))
	for hash := range db.dirties {
		if hash != (common.Hash{}) { // Special case for "root" references/nodes
			hashes = append(hashes, hash)
		}
	}
	return hashes
}

// Reference adds a new reference from a parent node to a child node.
// This function is used to add reference between internal trie node
// and external node(e.g. storage trie root), all internal trie nodes
// are referenced together by database itself.
func (db *Database) Reference(child common.Hash, parent common.Hash) {
	db.lock.Lock()
	defer db.lock.Unlock()

	db.reference(child, parent)
}

// reference is the private locked version of Reference.
func (db *Database) reference(child common.Hash, parent common.Hash) {
	// If the node does not exist, it's a node pulled from disk, skip
	node, ok := db.dirties[child]
	if !ok {
		return
	}
	// If the reference already exists, only duplicate for roots
	if db.dirties[parent].children == nil {
		db.dirties[parent].children = make(map[common.Hash]uint16)
		db.childrenSize += cachedNodeChildrenSize
	} else if _, ok = db.dirties[parent].children[child]; ok && parent != (common.Hash{}) {
		return
	}
	node.parents++
	db.dirties[parent].children[child]++
	if db.dirties[parent].children[child] == 1 {
		db.childrenSize += common.HashLength + 2 // uint16 counter
	}
}

// Dereference removes an existing reference from a root node.
func (db *Database) Dereference(root common.Hash) {
	// Sanity check to ensure that the meta-root is not removed
	if root == (common.Hash{}) {
		log.Error("Attempted to dereference the trie cache meta root")
		return
	}
	db.lock.Lock()
	defer db.lock.Unlock()

	nodes, storage, start := len(db.dirties), db.dirtiesSize, time.Now()
	db.dereference(root, common.Hash{})

	db.gcnodes += uint64(nodes - len(db.dirties))
	db.gcsize += storage - db.dirtiesSize
	db.gctime += time.Since(start)

	memcacheGCTimeTimer.Update(time.Since(start))
	memcacheGCSizeMeter.Mark(int64(storage - db.dirtiesSize))
	memcacheGCNodesMeter.Mark(int64(nodes - len(db.dirties)))

	log.Debug("Dereferenced trie from memory database", "nodes", nodes-len(db.dirties), "size", storage-db.dirtiesSize, "time", time.Since(start),
		"gcnodes", db.gcnodes, "gcsize", db.gcsize, "gctime", db.gctime, "livenodes", len(db.dirties), "livesize", db.dirtiesSize)
}

// dereference is the private locked version of Dereference.
func (db *Database) dereference(child common.Hash, parent common.Hash) {
	// Dereference the parent-child
	node := db.dirties[parent]

	if node.children != nil && node.children[child] > 0 {
		node.children[child]--
		if node.children[child] == 0 {
			delete(node.children, child)
			db.childrenSize -= (common.HashLength + 2) // uint16 counter
		}
	}
	// If the child does not exist, it's a previously committed node.
	node, ok := db.dirties[child]
	if !ok {
		return
	}
	// If there are no more references to the child, delete it and cascade
	if node.parents > 0 {
		// This is a special cornercase where a node loaded from disk (i.e. not in the
		// memcache any more) gets reinjected as a new node (short node split into full,
		// then reverted into short), causing a cached node to have no parents. That is
		// no problem in itself, but don't make maxint parents out of it.
		node.parents--
	}
	if node.parents == 0 {
		// Remove the node from the flush-list
		switch child {
		case db.oldest:
			db.oldest = node.flushNext
			db.dirties[node.flushNext].flushPrev = common.Hash{}
		case db.newest:
			db.newest = node.flushPrev
			db.dirties[node.flushPrev].flushNext = common.Hash{}
		default:
			db.dirties[node.flushPrev].flushNext = node.flushNext
			db.dirties[node.flushNext].flushPrev = node.flushPrev
		}
		// Dereference all children and delete the node
		node.forChilds(func(hash common.Hash) {
			db.dereference(hash, child)
		})
		delete(db.dirties, child)
		db.dirtiesSize -= common.StorageSize(common.HashLength + int(node.size))
		if node.children != nil {
			db.childrenSize -= cachedNodeChildrenSize
		}
	}
}

// Update inserts the dirty nodes in provided nodeset into database and
// link the account trie with multiple storage tries if necessary.
func (db *Database) Update(nodes *MergedNodeSet) error {
	db.lock.Lock()
	defer db.lock.Unlock()

	// Insert dirty nodes into the database. In the same tree, it must be
	// ensured that children are inserted first, then parent so that children
	// can be linked with their parent correctly.
	//
	// Note, the storage tries must be flushed before the account trie to
	// retain the invariant that children go into the dirty cache first.
	var order []common.Hash
	for owner := range nodes.sets {
		if owner == (common.Hash{}) {
			continue
		}
		order = append(order, owner)
	}
	if _, ok := nodes.sets[common.Hash{}]; ok {
		order = append(order, common.Hash{})
	}
	for _, owner := range order {
		subset := nodes.sets[owner]
		subset.forEachWithOrder(func(path string, n *memoryNode) {
			if n.isDeleted() {
				return // ignore deletion
			}
			db.insert(n.hash, int(n.size), n.node)
		})
	}
	// Link up the account trie and storage trie if the node points
	// to an account trie leaf.
	if set, present := nodes.sets[common.Hash{}]; present {
		for _, n := range set.leaves {
			var account types.StateAccount
			if err := rlp.DecodeBytes(n.blob, &account); err != nil {
				return err
			}
			if account.Root != types.EmptyRootHash {
				db.reference(account.Root, n.parent)
			}
		}
	}
	return nil
}

// Size returns the current storage size of the memory cache in front of the
// persistent database layer.
func (db *Database) Size() (common.StorageSize, common.StorageSize) {
	db.lock.RLock()
	defer db.lock.RUnlock()

	// db.dirtiesSize only contains the useful data in the cache, but when reporting
	// the total memory consumption, the maintenance metadata is also needed to be
	// counted.
	var metadataSize = common.StorageSize((len(db.dirties) - 1) * cachedNodeSize)
	var metarootRefs = common.StorageSize(len(db.dirties[common.Hash{}].children) * (common.HashLength + 2))
	var preimageSize common.StorageSize
	if db.preimages != nil {
		preimageSize = db.preimages.size()
	}
	return db.dirtiesSize + db.childrenSize + metadataSize - metarootRefs, preimageSize
}

// GetReader retrieves a node reader belonging to the given state root.
func (db *Database) GetReader(root common.Hash, codec uint64) Reader {
	return &hashReader{db: db, codec: codec}
}

// hashReader is reader of hashDatabase which implements the Reader interface.
type hashReader struct {
	db    *Database
	codec uint64
}

// Node retrieves the trie node with the given node hash.
func (reader *hashReader) Node(owner common.Hash, path []byte, hash common.Hash) (node, error) {
	blob, err := reader.NodeBlob(owner, path, hash)
	if err != nil {
		return nil, err
	}
	return decodeNodeUnsafe(hash[:], blob)
}

// NodeBlob retrieves the RLP-encoded trie node blob with the given node hash.
func (reader *hashReader) NodeBlob(_ common.Hash, _ []byte, hash common.Hash) ([]byte, error) {
	return reader.db.Node(hash, reader.codec)
}

// saveCache saves clean state cache to given directory path
// using specified CPU cores.
func (db *Database) saveCache(dir string, threads int) error {
	if db.cleans == nil {
		return nil
	}
	log.Info("Writing clean trie cache to disk", "path", dir, "threads", threads)

	start := time.Now()
	err := db.cleans.SaveToFileConcurrent(dir, threads)
	if err != nil {
		log.Error("Failed to persist clean trie cache", "error", err)
		return err
	}
	log.Info("Persisted the clean trie cache", "path", dir, "elapsed", common.PrettyDuration(time.Since(start)))
	return nil
}

// SaveCache atomically saves fast cache data to the given dir using all
// available CPU cores.
func (db *Database) SaveCache(dir string) error {
	return db.saveCache(dir, runtime.GOMAXPROCS(0))
}

// SaveCachePeriodically atomically saves fast cache data to the given dir with
// the specified interval. All dump operation will only use a single CPU core.
func (db *Database) SaveCachePeriodically(dir string, interval time.Duration, stopCh <-chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			db.saveCache(dir, 1)
		case <-stopCh:
			return
		}
	}
}

// Scheme returns the node scheme used in the database.
func (db *Database) Scheme() string {
	return rawdb.HashScheme
}
