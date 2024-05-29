package internal

import (
	"testing"
	"time"

	pgipfsethdb "github.com/cerc-io/ipfs-ethdb/v5/postgres/v0"
	"github.com/cerc-io/plugeth-statediff/indexer/ipld"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ipfs/go-cid"
	"github.com/multiformats/go-multihash"
)

var (
	StateTrieCodec   uint64 = ipld.MEthStateTrie
	StorageTrieCodec uint64 = ipld.MEthStorageTrie
)

func Keccak256ToCid(codec uint64, h []byte) (cid.Cid, error) {
	buf, err := multihash.Encode(h, multihash.KECCAK_256)
	if err != nil {
		return cid.Cid{}, err
	}
	return cid.NewCidV1(codec, buf), nil
}

// returns a cache config with unique name (groupcache names are global)
func MakeCacheConfig(t testing.TB) pgipfsethdb.CacheConfig {
	return pgipfsethdb.CacheConfig{
		Name:           t.Name(),
		Size:           3000000, // 3MB
		ExpiryDuration: time.Hour,
	}
}

// ReadLegacyTrieNode retrieves the legacy trie node with the given
// associated node hash.
func ReadLegacyTrieNode(db ethdb.KeyValueReader, hash common.Hash, codec uint64) ([]byte, error) {
	cid, err := Keccak256ToCid(codec, hash[:])
	if err != nil {
		return nil, err
	}
	enc, err := db.Get(cid.Bytes())
	if err != nil {
		return nil, err
	}
	return enc, nil
}

// HasLegacyTrieNode checks if the trie node with the provided hash is present in db.
func HasLegacyTrieNode(db ethdb.KeyValueReader, hash common.Hash, codec uint64) bool {
	cid, err := Keccak256ToCid(codec, hash[:])
	if err != nil {
		return false
	}
	ok, _ := db.Has(cid.Bytes())
	return ok
}

func WriteLegacyTrieNode(db ethdb.KeyValueWriter, hash common.Hash, data []byte, codec uint64) error {
	//
	panic("Writing to the DB is disabled")
	cid, err := Keccak256ToCid(codec, hash[:])
	if err != nil {
		return err
	}
	return db.Put(cid.Bytes(), data)
}

func ReadCode(db ethdb.KeyValueReader, hash common.Hash) ([]byte, error) {
	return ReadLegacyTrieNode(db, hash, ipld.RawBinary)
}

func WriteCode(db ethdb.KeyValueWriter, hash common.Hash, code []byte) error {
	return WriteLegacyTrieNode(db, hash, code, ipld.RawBinary)
}
