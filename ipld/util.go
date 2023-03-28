package ipld

import (
	"github.com/ethereum/go-ethereum/statediff/indexer/ipld"
	"github.com/ipfs/go-cid"
	"github.com/multiformats/go-multihash"
)

const (
	RawBinary           = ipld.RawBinary
	MEthHeader          = ipld.MEthHeader
	MEthHeaderList      = ipld.MEthHeaderList
	MEthTxTrie          = ipld.MEthTxTrie
	MEthTx              = ipld.MEthTx
	MEthTxReceiptTrie   = ipld.MEthTxReceiptTrie
	MEthTxReceipt       = ipld.MEthTxReceipt
	MEthStateTrie       = ipld.MEthStateTrie
	MEthAccountSnapshot = ipld.MEthAccountSnapshot
	MEthStorageTrie     = ipld.MEthStorageTrie
	MEthLogTrie         = ipld.MEthLogTrie
	MEthLog             = ipld.MEthLog
)

var RawdataToCid = ipld.RawdataToCid

// var Keccak256ToCid = ipld.Keccak256ToCid

// // Keccak256ToCid converts keccak256 hash bytes into a v1 cid
// // (non-panicking function)
// func Keccak256ToCid(hash []byte, codecType uint64) (cid.Cid, error) {
// 	mh, err := multihash.Encode(hash, multihash.KECCAK_256)
// 	if err != nil {
// 		return cid.Cid{}, err
// 	}
// 	return cid.NewCidV1(codecType, mh), nil
// }
