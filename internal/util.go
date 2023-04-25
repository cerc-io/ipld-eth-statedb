package internal

import (
	"testing"
	"time"

	pgipfsethdb "github.com/cerc-io/ipfs-ethdb/v5/postgres/v0"
	"github.com/ipfs/go-cid"
	"github.com/multiformats/go-multihash"
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
