package util

import (
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
