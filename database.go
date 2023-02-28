package ipld_eth_statedb

import (
	"context"
	"errors"

	"github.com/VictoriaMetrics/fastcache"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/statediff/indexer/ipld"
	"github.com/ethereum/go-ethereum/trie"
	lru "github.com/hashicorp/golang-lru"
	"github.com/jackc/pgx/pgxpool"
)

const (
	// Number of codehash->size associations to keep.
	codeSizeCacheSize = 100000

	// Cache size granted for caching clean code.
	codeCacheSize = 64 * 1024 * 1024
)

var (
	// not found error
	errNotFound = errors.New("not found")
)

// Database interface is a union of the subset of the geth state.Database interface required
// to support the vm.StateDB implementation as well as methods specific to this Postgres based implementation
type Database interface {
	ContractCode(addrHash common.Hash, codeHash common.Hash) ([]byte, error)
	ContractCodeSize(addrHash common.Hash, codeHash common.Hash) (int, error)
	StateAccount(address common.Address) (*types.StateAccount, error)
	StorageSlot(addressHash, slotHash common.Hash) ([]byte, error)
}

var _ Database = &stateDatabase{}

type stateDatabase struct {
	pgdb          *pgxpool.Pool
	trieDB        *trie.Database
	codeSizeCache *lru.Cache
	codeCache     *fastcache.Cache
}

func NewStateDatabase(pgdb *pgxpool.Pool, ethdb ethdb.Database, config *trie.Config) (*stateDatabase, error) {
	csc, _ := lru.New(codeSizeCacheSize)
	return &stateDatabase{
		pgdb:          pgdb,
		trieDB:        trie.NewDatabaseWithConfig(ethdb, config),
		codeSizeCache: csc,
		codeCache:     fastcache.New(codeCacheSize),
	}, nil
}

func (sd *stateDatabase) ContractCode(_, codeHash common.Hash) ([]byte, error) {
	if code := sd.codeCache.Get(nil, codeHash.Bytes()); len(code) > 0 {
		return code, nil
	}
	cid := ipld.Keccak256ToCid(ipld.RawBinary, codeHash.Bytes())
	code := make([]byte, 0)
	if err := sd.pgdb.QueryRow(context.Background(), GetContractCodePgStr, cid).Scan(&code); err != nil {
		return nil, errNotFound
	}
	if len(code) > 0 {
		sd.codeCache.Set(codeHash.Bytes(), code)
		sd.codeSizeCache.Add(codeHash, len(code))
		return code, nil
	}
	return nil, errNotFound
}

func (sd *stateDatabase) ContractCodeSize(_, codeHash common.Hash) (int, error) {
	if cached, ok := sd.codeSizeCache.Get(codeHash); ok {
		return cached.(int), nil
	}
	code, err := sd.ContractCode(common.Hash{}, codeHash)
	return len(code), err
}

func (sd *stateDatabase) StateAccount(address common.Address) (*types.StateAccount, error) {
	panic("implement me")
}

func (sd *stateDatabase) StorageSlot(addressHash, slotHash common.Hash) ([]byte, error) {
	panic("implement me")
}