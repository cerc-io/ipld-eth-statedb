package helper

import (
	"hash"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"golang.org/x/crypto/sha3"
)

var _ types.TrieHasher = &testHasher{}

// testHasher (copied from go-ethereum/core/types/block_test.go)
// satisfies types.TrieHasher
type testHasher struct {
	hasher hash.Hash
}

func NewHasher() *testHasher {
	return &testHasher{hasher: sha3.NewLegacyKeccak256()}
}

func (h *testHasher) Reset() {
	h.hasher.Reset()
}

func (h *testHasher) Update(key, val []byte) error {
	_, err := h.hasher.Write(key)
	if err != nil {
		return err
	}
	_, err = h.hasher.Write(val)
	return err
}

func (h *testHasher) Hash() common.Hash {
	return common.BytesToHash(h.hasher.Sum(nil))
}
