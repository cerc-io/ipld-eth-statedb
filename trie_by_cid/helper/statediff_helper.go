package helper

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/statediff"
	"github.com/ethereum/go-ethereum/statediff/indexer"
	"github.com/ethereum/go-ethereum/statediff/indexer/database/sql/postgres"
	"github.com/ethereum/go-ethereum/statediff/indexer/node"
)

var (
	ChainConfig = params.TestChainConfig

	mockTD = big.NewInt(1)
)

// IndexStateDiff indexes a single statediff.
// - uses TestChainConfig
// - block hash/number are left as zero
func IndexStateDiff(dbConfig postgres.Config, stateCache state.Database, rootA, rootB common.Hash) error {
	_, indexer, err := indexer.NewStateDiffIndexer(
		context.Background(), ChainConfig, node.Info{}, dbConfig)
	if err != nil {
		return err
	}
	defer indexer.Close() // fixme: hangs when using PGX driver

	builder := statediff.NewBuilder(stateCache)
	block := types.NewBlock(&types.Header{Root: rootB}, nil, nil, nil, NewHasher())

	// uses zero block hash/number, we only need the trie structure here
	args := statediff.Args{
		OldStateRoot: rootA,
		NewStateRoot: rootB,
	}
	diff, err := builder.BuildStateDiffObject(args, statediff.Params{})
	if err != nil {
		return err
	}
	tx, err := indexer.PushBlock(block, nil, mockTD)
	if err != nil {
		return err
	}
	// we don't need to index diff.Nodes since we are just interested in the trie
	for _, ipld := range diff.IPLDs {
		if err := indexer.PushIPLD(tx, ipld); err != nil {
			return err
		}
	}
	return tx.Submit(err)
}
