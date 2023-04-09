package state

import (
	"errors"

	"github.com/VictoriaMetrics/fastcache"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/statediff/indexer/ipld"
	lru "github.com/hashicorp/golang-lru"

	"github.com/cerc-io/ipld-eth-utils/bycid/trie"
)

const (
	// Number of codehash->size associations to keep.
	codeSizeCacheSize = 100000

	// Cache size granted for caching clean code.
	codeCacheSize = 64 * 1024 * 1024
)

// Database wraps access to tries and contract code.
type Database interface {
	// OpenTrie opens the main account trie.
	OpenTrie(root common.Hash) (Trie, error)

	// OpenStorageTrie opens the storage trie of an account.
	OpenStorageTrie(addrHash, root common.Hash) (Trie, error)

	// ContractCode retrieves a particular contract's code.
	ContractCode(codeHash common.Hash) ([]byte, error)

	// ContractCodeSize retrieves a particular contracts code's size.
	ContractCodeSize(codeHash common.Hash) (int, error)

	// TrieDB retrieves the low level trie database used for data storage.
	TrieDB() *trie.Database
}

// Trie is a Ethereum Merkle Patricia trie.
type Trie interface {
	TryGet(key []byte) ([]byte, error)
	TryGetAccount(key []byte) (*types.StateAccount, error)
	Hash() common.Hash
	// NodeIterator(startKey []byte) trie.NodeIterator
	Prove(key []byte, fromLevel uint, proofDb ethdb.KeyValueWriter) error
}

// NewDatabase creates a backing store for state. The returned database is safe for
// concurrent use, but does not retain any recent trie nodes in memory. To keep some
// historical state in memory, use the NewDatabaseWithConfig constructor.
func NewDatabase(db ethdb.Database) Database {
	return NewDatabaseWithConfig(db, nil)
}

// NewDatabaseWithConfig creates a backing store for state. The returned database
// is safe for concurrent use and retains a lot of collapsed RLP trie nodes in a
// large memory cache.
func NewDatabaseWithConfig(db ethdb.Database, config *trie.Config) Database {
	csc, _ := lru.New(codeSizeCacheSize)
	return &cachingDB{
		db:            trie.NewDatabaseWithConfig(db, config),
		codeSizeCache: csc,
		codeCache:     fastcache.New(codeCacheSize),
	}
}

type cachingDB struct {
	db            *trie.Database
	codeSizeCache *lru.Cache
	codeCache     *fastcache.Cache
}

// OpenTrie opens the main account trie at a specific root hash.
func (db *cachingDB) OpenTrie(root common.Hash) (Trie, error) {
	tr, err := trie.NewStateTrie(common.Hash{}, root, db.db)
	if err != nil {
		return nil, err
	}
	return tr, nil
}

// OpenStorageTrie opens the storage trie of an account.
func (db *cachingDB) OpenStorageTrie(addrHash, root common.Hash) (Trie, error) {
	tr, err := trie.NewStorageTrie(addrHash, root, db.db)
	if err != nil {
		return nil, err
	}
	return tr, nil
}

// ContractCode retrieves a particular contract's code.
func (db *cachingDB) ContractCode(codeHash common.Hash) ([]byte, error) {
	if code := db.codeCache.Get(nil, codeHash.Bytes()); len(code) > 0 {
		return code, nil
	}
	// TODO - use non panicking
	codeCID := ipld.Keccak256ToCid(ipld.RawBinary, codeHash.Bytes())
	// if err != nil {
	// 	return nil, err
	// }
	code, err := db.db.DiskDB().Get(codeCID.Bytes())
	if err != nil {
		return nil, err
	}
	if len(code) > 0 {
		db.codeCache.Set(codeHash.Bytes(), code)
		db.codeSizeCache.Add(codeHash, len(code))
		return code, nil
	}
	return nil, errors.New("not found")
}

// ContractCodeSize retrieves a particular contracts code's size.
func (db *cachingDB) ContractCodeSize(codeHash common.Hash) (int, error) {
	if cached, ok := db.codeSizeCache.Get(codeHash); ok {
		return cached.(int), nil
	}
	code, err := db.ContractCode(codeHash)
	return len(code), err
}

// TrieDB retrieves any intermediate trie-node caching layer.
func (db *cachingDB) TrieDB() *trie.Database {
	return db.db
}
