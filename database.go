package ipld_eth_statedb

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/jackc/pgx/pgxpool"
)

type Database interface {
	ContractCode(addrHash common.Hash, codeHash common.Hash) ([]byte, error)
	ContractCodeSize(addrHash common.Hash, codeHash common.Hash) (int, error)
	OpenTrie(root common.Hash) (state.Trie, error)
	OpenStorageTrie(addrHash common.Hash, root common.Hash) (state.Trie, error)
	CopyTrie(trie state.Trie) state.Trie
}

type StateDatabase struct {
	pgxpool.Pool
}

func (sd *StateDatabase) ContractCode(addrHash common.Hash, codeHash common.Hash) ([]byte, error) {
	panic("implement me")
}

func (sd *StateDatabase) ContractCodeSize(addrHash common.Hash, codeHash common.Hash) (int, error) {
	panic("implement me")
}

func (sd *StateDatabase) OpenTrie(root common.Hash) (state.Trie, error) {
	panic("replace my usage")
}

func (sd *StateDatabase) OpenStorageTrie(addrHash common.Hash, root common.Hash) (state.Trie, error) {
	panic("replace my usage")
}

func (sd *StateDatabase) CopyTrie(trie state.Trie) state.Trie {
	panic("replace my usage")
}
