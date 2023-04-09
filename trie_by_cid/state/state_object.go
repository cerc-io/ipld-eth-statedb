package state

import (
	"bytes"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/metrics"
	"github.com/ethereum/go-ethereum/rlp"
)

var (
	// emptyRoot is the known root hash of an empty trie.
	// this is calculated as: emptyRoot = crypto.Keccak256(rlp.Encode([][]byte{}))
	// that is, the keccak356 hash of the rlp encoding of an empty trie node (empty byte slice array)
	emptyRoot = common.HexToHash("56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421")
	// emptyCodeHash is the CodeHash for an EOA, for an account without contract code deployed
	emptyCodeHash = crypto.Keccak256(nil)
)

type Code []byte

func (c Code) String() string {
	return string(c) //strings.Join(Disassemble(c), " ")
}

type Storage map[common.Hash]common.Hash

func (s Storage) String() (str string) {
	for key, value := range s {
		str += fmt.Sprintf("%X : %X\n", key, value)
	}

	return
}

func (s Storage) Copy() Storage {
	cpy := make(Storage, len(s))
	for key, value := range s {
		cpy[key] = value
	}

	return cpy
}

// stateObject represents an Ethereum account which is being accessed.
//
// The usage pattern is as follows:
// First you need to obtain a state object.
// Account values can be accessed through the object.
type stateObject struct {
	address  common.Address
	addrHash common.Hash // hash of ethereum address of the account
	data     types.StateAccount
	db       *StateDB

	// Caches.
	trie Trie // storage trie, which becomes non-nil on first access
	code Code // contract bytecode, which gets set when code is loaded

	originStorage Storage // Storage cache of original entries to dedup rewrites, reset for every transaction
	fakeStorage   Storage // Fake storage which constructed by caller for debugging purpose.
}

// empty returns whether the account is considered empty.
func (s *stateObject) empty() bool {
	return s.data.Nonce == 0 && s.data.Balance.Sign() == 0 && bytes.Equal(s.data.CodeHash, emptyCodeHash)
}

// newObject creates a state object.
func newObject(db *StateDB, address common.Address, data types.StateAccount) *stateObject {
	if data.Balance == nil {
		data.Balance = new(big.Int)
	}
	if data.CodeHash == nil {
		data.CodeHash = emptyCodeHash
	}
	if data.Root == (common.Hash{}) {
		data.Root = emptyRoot
	}
	return &stateObject{
		db:            db,
		address:       address,
		addrHash:      crypto.Keccak256Hash(address[:]),
		data:          data,
		originStorage: make(Storage),
	}
}

// setError remembers the first non-nil error it is called with.
func (s *stateObject) setError(err error) {
	s.db.setError(err)
}

func (s *stateObject) getTrie(db Database) Trie {
	if s.trie == nil {
		// // Try fetching from prefetcher first
		// // We don't prefetch empty tries
		// if s.data.Root != emptyRoot && s.db.prefetcher != nil {
		// 	// When the miner is creating the pending state, there is no
		// 	// prefetcher
		// 	s.trie = s.db.prefetcher.trie(s.addrHash, s.data.Root)
		// }
		if s.trie == nil {
			var err error
			s.trie, err = db.OpenStorageTrie(s.addrHash, s.data.Root)
			if err != nil {
				s.trie, _ = db.OpenStorageTrie(s.addrHash, common.Hash{})
				s.setError(fmt.Errorf("can't create storage trie: %w", err))
			}
		}
	}
	return s.trie
}

// GetCommittedState retrieves a value from the committed account storage trie.
func (s *stateObject) GetState(db Database, key common.Hash) common.Hash {
	// If the fake storage is set, only lookup the state here(in the debugging mode)
	if s.fakeStorage != nil {
		return s.fakeStorage[key]
	}
	// If we have a cached value, return that
	if value, cached := s.originStorage[key]; cached {
		return value
	}
	// If no live objects are available, load from the database.
	start := time.Now()
	enc, err := s.getTrie(db).TryGet(key.Bytes())
	if metrics.EnabledExpensive {
		s.db.StorageReads += time.Since(start)
	}
	if err != nil {
		s.setError(err)
		return common.Hash{}
	}
	var value common.Hash
	if len(enc) > 0 {
		_, content, _, err := rlp.Split(enc)
		if err != nil {
			s.setError(err)
		}
		value.SetBytes(content)
	}
	s.originStorage[key] = value
	return value
}

//
// Attribute accessors
//

// Address returns the address of the contract/account
func (s *stateObject) Address() common.Address {
	return s.address
}

// Code returns the contract code associated with this object, if any.
func (s *stateObject) Code(db Database) []byte {
	if s.code != nil {
		return s.code
	}
	if bytes.Equal(s.CodeHash(), emptyCodeHash) {
		return nil
	}
	code, err := db.ContractCode(common.BytesToHash(s.CodeHash()))
	if err != nil {
		s.setError(fmt.Errorf("can't load code hash %x: %v", s.CodeHash(), err))
	}
	s.code = code
	return code
}

// CodeSize returns the size of the contract code associated with this object,
// or zero if none. This method is an almost mirror of Code, but uses a cache
// inside the database to avoid loading codes seen recently.
func (s *stateObject) CodeSize(db Database) int {
	if s.code != nil {
		return len(s.code)
	}
	if bytes.Equal(s.CodeHash(), emptyCodeHash) {
		return 0
	}
	size, err := db.ContractCodeSize(common.BytesToHash(s.CodeHash()))
	if err != nil {
		s.setError(fmt.Errorf("can't load code size %x: %v", s.CodeHash(), err))
	}
	return size
}

func (s *stateObject) CodeHash() []byte {
	return s.data.CodeHash
}

func (s *stateObject) Balance() *big.Int {
	return s.data.Balance
}

func (s *stateObject) Nonce() uint64 {
	return s.data.Nonce
}

// Never called, but must be present to allow stateObject to be used
// as a vm.Account interface that also satisfies the vm.ContractRef
// interface. Interfaces are awesome.
func (s *stateObject) Value() *big.Int {
	panic("Value on stateObject should never be called")
}
