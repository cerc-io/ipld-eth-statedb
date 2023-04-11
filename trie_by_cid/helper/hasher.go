package helper

import (
	"hash"

	"github.com/ethereum/go-ethereum/common"
	"golang.org/x/crypto/sha3"
)

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

func (h *testHasher) Update(key, val []byte) {
	h.hasher.Write(key)
	h.hasher.Write(val)
}

func (h *testHasher) Hash() common.Hash {
	return common.BytesToHash(h.hasher.Sum(nil))
}
