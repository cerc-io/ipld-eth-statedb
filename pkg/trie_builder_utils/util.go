package trie_builder_utils

import (
	"bytes"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// BuildAndReportKeySetWithBranchToDepth takes a depth argument
// and returns the first two slots (that when hashed into trie keys) that intersect at or below that provided depth
// it hashes the slots and converts to nibbles before finding the intersection
// it also returns the nibble and hex string representations of the two intersecting keys
// this is useful for identifying what contract slots need to be occupied to cause branching in the storage trie
// at or below a provided height
func BuildAndReportKeySetWithBranchToDepth(depth int) (string, string, []byte, []byte, string, string) {
	slots, storageLeafKeys, storageLeafKeyStrs, key1, key2 := buildKeySetWithBranchToDepth(depth)
	var slot1 string
	var slot2 string
	var key1Str string
	var key2Str string
	for i, storageLeafKey := range storageLeafKeys {
		if bytes.Equal(storageLeafKey, key1) {
			slot1 = slots[i]
			key1Str = storageLeafKeyStrs[i]
		}
		if bytes.Equal(storageLeafKey, key2) {
			slot2 = slots[i]
			key2Str = storageLeafKeyStrs[i]
		}
	}
	return slot1, slot2, key1, key2, key1Str, key2Str
}

func buildKeySetWithBranchToDepth(depth int) ([]string, [][]byte, []string, []byte, []byte) {
	slots := make([]string, 0)
	storageLeafKeys := make([][]byte, 0)
	storageLeafKeyStrs := make([]string, 0)
	i := 0
	j := 1
	k := depth
	if depth > 5 {
		k = 10000
	}
	if depth > 7 {
		k = 50000
	}
	for {
		slots = append(slots, common.BigToHash(big.NewInt(int64(i))).String())
		storageLeafKeys = append(storageLeafKeys, LeafKeyToHexNibbles(crypto.Keccak256(common.BigToHash(big.NewInt(int64(i))).Bytes())))
		storageLeafKeyStrs = append(storageLeafKeyStrs, crypto.Keccak256Hash(common.BigToHash(big.NewInt(int64(i))).Bytes()).String())
		i++
		if len(storageLeafKeys) > k*j {
			j++
			ok, key1, key2 := checkBranchDepthOfSet(storageLeafKeys, depth)
			if ok {
				return slots, storageLeafKeys, storageLeafKeyStrs, key1, key2
			}
		}
	}
}

func checkBranchDepthOfSet(storageLeafKeys [][]byte, depth int) (bool, []byte, []byte) {
	for i, key1 := range storageLeafKeys {
		for j, key2 := range storageLeafKeys {
			if i == j {
				continue
			}
			var ok bool
			var growingPrefix []byte
			for _, by := range key1 {
				ok, growingPrefix = containsPrefix(key2, growingPrefix, []byte{by})
				if ok {
					if len(growingPrefix) >= depth {
						return true, key1, key2
					}
					continue
				} else {
					break
				}
			}
		}
	}
	return false, nil, nil
}

func containsPrefix(key, growingPrefix, potentialAddition []byte) (bool, []byte) {
	if bytes.HasPrefix(key, append(growingPrefix, potentialAddition...)) {
		return true, append(growingPrefix, potentialAddition...)
	}
	return false, growingPrefix
}

func LeafKeyToHexNibbles(compactLeafKey []byte) []byte {
	if len(compactLeafKey) == 0 {
		return compactLeafKey
	}
	return keybytesToHex(compactLeafKey)
}

func keybytesToHex(str []byte) []byte {
	l := len(str) * 2
	var nibbles = make([]byte, l)
	for i, b := range str {
		nibbles[i*2] = b / 16
		nibbles[i*2+1] = b % 16
	}
	return nibbles
}
