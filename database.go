package ipld_eth_statedb

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"github.com/VictoriaMetrics/fastcache"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/statediff/indexer/ipld"
	lru "github.com/hashicorp/golang-lru"
	"github.com/jackc/pgx/v4/pgxpool"

	util "github.com/cerc-io/ipld-eth-statedb/internal"
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
	ContractCode(codeHash common.Hash) ([]byte, error)
	ContractCodeSize(codeHash common.Hash) (int, error)
	StateAccount(addressHash, blockHash common.Hash) (*types.StateAccount, error)
	StorageValue(addressHash, slotHash, blockHash common.Hash) ([]byte, error)
}

var _ Database = &stateDatabase{}

type stateDatabase struct {
	pgdb          *pgxpool.Pool
	codeSizeCache *lru.Cache
	codeCache     *fastcache.Cache
}

// NewStateDatabaseWithPool returns a new Database implementation using the provided postgres connection pool
func NewStateDatabaseWithPool(pgDb *pgxpool.Pool) (*stateDatabase, error) {
	csc, _ := lru.New(codeSizeCacheSize)
	return &stateDatabase{
		pgdb:          pgDb,
		codeSizeCache: csc,
		codeCache:     fastcache.New(codeCacheSize),
	}, nil
}

// NewStateDatabase returns a new Database implementation using the passed parameters
func NewStateDatabase(ctx context.Context, conf Config) (*stateDatabase, error) {
	pgDb, err := NewPGXPool(ctx, conf)
	if err != nil {
		return nil, err
	}
	return NewStateDatabaseWithPool(pgDb)
}

// ContractCode satisfies Database, it returns the contract code for a given codehash
func (sd *stateDatabase) ContractCode(codeHash common.Hash) ([]byte, error) {
	if code := sd.codeCache.Get(nil, codeHash.Bytes()); len(code) > 0 {
		return code, nil
	}
	c, err := util.Keccak256ToCid(ipld.RawBinary, codeHash.Bytes())
	if err != nil {
		return nil, fmt.Errorf("cannot derive CID from provided codehash: %s", err.Error())
	}
	code := make([]byte, 0)
	if err := sd.pgdb.QueryRow(context.Background(), GetContractCodePgStr, c).Scan(&code); err != nil {
		return nil, err
	}
	if len(code) > 0 {
		sd.codeCache.Set(codeHash.Bytes(), code)
		sd.codeSizeCache.Add(codeHash, len(code))
		return code, nil
	}
	return nil, errNotFound
}

// ContractCodeSize satisfies Database, it returns the length of the code for a provided codehash
func (sd *stateDatabase) ContractCodeSize(codeHash common.Hash) (int, error) {
	if cached, ok := sd.codeSizeCache.Get(codeHash); ok {
		return cached.(int), nil
	}
	code, err := sd.ContractCode(codeHash)
	return len(code), err
}

// StateAccount satisfies Database, it returns the types.StateAccount for a provided address and block hash
func (sd *stateDatabase) StateAccount(addressHash, blockHash common.Hash) (*types.StateAccount, error) {
	res := StateAccountResult{}
	err := sd.pgdb.QueryRow(context.Background(), GetStateAccount, addressHash.Hex(), blockHash.Hex()).
		Scan(&res.Balance, &res.Nonce, &res.CodeHash, &res.StorageRoot, &res.Removed)
	if err != nil {
		return nil, err
	}
	if res.Removed {
		// TODO: check expected behavior for deleted/non existing accounts
		return nil, nil
	}
	bal := new(big.Int)
	bal.SetString(res.Balance, 10)
	return &types.StateAccount{
		Nonce:    res.Nonce,
		Balance:  bal,
		Root:     common.HexToHash(res.StorageRoot),
		CodeHash: common.HexToHash(res.CodeHash).Bytes(),
	}, nil
}

// StorageValue satisfies Database, it returns the RLP-encoded storage value for the provided address, slot,
// and block hash
func (sd *stateDatabase) StorageValue(addressHash, slotHash, blockHash common.Hash) ([]byte, error) {
	res := StorageSlotResult{}
	err := sd.pgdb.QueryRow(context.Background(), GetStorageSlot,
		addressHash.Hex(), slotHash.Hex(), blockHash.Hex()).
		Scan(&res.Value, &res.Removed, &res.StateLeafRemoved)
	if err != nil {
		return nil, err
	}
	if res.Removed || res.StateLeafRemoved {
		// TODO: check expected behavior for deleted/non existing accounts
		return nil, nil
	}
	return res.Value, nil
}
