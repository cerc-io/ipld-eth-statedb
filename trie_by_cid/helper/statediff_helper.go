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
	// ChainDB     = rawdb.NewMemoryDatabase()
	ChainConfig = params.TestChainConfig
	// BankFunds   = new(big.Int).Mul(big.NewInt(1e4), big.NewInt(params.Ether)) // i.e. 10,000eth

	mockTD = big.NewInt(1)
	// ctx    = context.Background()
	// signer = types.NewLondonSigner(ChainConfig.ChainID)
)

func IndexChain(dbConfig postgres.Config, stateCache state.Database, rootA, rootB common.Hash) error {
	_, indexer, err := indexer.NewStateDiffIndexer(
		context.Background(),
		ChainConfig,
		node.Info{},
		// node.Info{
		// 	GenesisBlock: Genesis.Hash().String(),
		// 	NetworkID:    "test_network",
		// 	ID:           "test_node",
		// 	ClientName:   "geth",
		// 	ChainID:      ChainConfig.ChainID.Uint64(),
		// },
		dbConfig)
	if err != nil {
		return err
	}
	defer indexer.Close() // fixme: hangs when using PGX driver

	// generating statediff payload for block, and transform the data into Postgres
	builder := statediff.NewBuilder(stateCache)
	block := types.NewBlock(&types.Header{Root: rootB}, nil, nil, nil, NewHasher())

	// todo: use dummy block hashes to just produce trie structure for testing
	args := statediff.Args{
		OldStateRoot: rootA,
		NewStateRoot: rootB,
		// BlockNumber:  block.Number(),
		// BlockHash:    block.Hash(),
	}
	diff, err := builder.BuildStateDiffObject(args, statediff.Params{})
	if err != nil {
		return err
	}
	tx, err := indexer.PushBlock(block, nil, mockTD)
	if err != nil {
		return err
	}
	// for _, node := range diff.Nodes {
	// 	err := indexer.PushStateNode(tx, node, block.Hash().String())
	// 	if err != nil {
	// 		return err
	// 	}
	// }
	for _, ipld := range diff.IPLDs {
		if err := indexer.PushIPLD(tx, ipld); err != nil {
			return err
		}
	}
	return tx.Submit(err)

	// if err = tx.Submit(err); err != nil {
	// 	return err
	// }
	// return nil
}
