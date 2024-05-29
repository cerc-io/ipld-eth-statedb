package state

import (
	"context"
	"math/big"
	"math/rand"
	"testing"

	pgipfsethdb "github.com/cerc-io/ipfs-ethdb/v5/postgres/v0"
	"github.com/cerc-io/plugeth-statediff/indexer/database/sql/postgres"
	helpers "github.com/cerc-io/plugeth-statediff/test_helpers"
	"github.com/cerc-io/plugeth-statediff/test_helpers/chaingen"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/params"

	"github.com/cerc-io/ipld-eth-statedb/internal"
	"github.com/cerc-io/ipld-eth-statedb/internal/testdata"
)

const (
	chainLength = 5
	startBlock  = 1
)

var (
	testCtx     = context.Background()
	DBConfig, _ = postgres.TestConfig.WithEnv()
)

var (
	bank, acct1, acct2 common.Address
	caddr1, caddr2     common.Address

	contractSpec   = chaingen.MustParseContract(testdata.TestContractABI, testdata.TestContractCode)
	BankKey, _     = crypto.HexToECDSA("0000000000000000000000000000000000000000000000000000000000000001")
	Account1Key, _ = crypto.HexToECDSA("0000000000000000000000000000000000000000000000000000000000000002")
	Account2Key, _ = crypto.HexToECDSA("0000000000000000000000000000000000000000000000000000000000000003")
	BankFunds      = big.NewInt(params.Ether * 2)
)

func newPgIpfsEthdb(t *testing.T) (ethdb.Database, func()) {
	pool, err := postgres.ConnectSQLX(testCtx, DBConfig)
	if err != nil {
		t.Fatal(err)
	}
	db := pgipfsethdb.NewDatabase(pool, internal.MakeCacheConfig(t))
	cleanup := func() {
		err := helpers.ClearDB(pool)
		if err != nil {
			panic(err)
		}
	}
	return db, cleanup
}

// generates and indexes a chain with arbitrary data, returning the final state root
func generateAndIndexChain(t *testing.T) common.Hash {
	testDB := rawdb.NewMemoryDatabase()
	chainConfig := &*params.TestChainConfig
	mockTD := big.NewInt(1337)

	// Make the test blockchain and state
	gen := makeGenContext(chainConfig, testDB)
	blocks, receipts, chain := gen.MakeChain(chainLength)
	defer chain.Stop()

	indexer, err := helpers.NewIndexer(context.Background(), chainConfig, gen.Genesis.Hash(), DBConfig)
	if err != nil {
		t.Fatal(err)
	}

	if err := helpers.IndexChain(indexer, helpers.IndexChainParams{
		StateCache:      chain.StateCache(),
		Blocks:          blocks,
		Receipts:        receipts,
		TotalDifficulty: mockTD,
	}); err != nil {
		t.Fatal(err)
	}
	return blocks[len(blocks)-1].Root()
}

// A GenContext which exactly replicates the chain generator used in existing tests
func makeGenContext(chainConfig *params.ChainConfig, db ethdb.Database) *chaingen.GenContext {
	gen := chaingen.NewGenContext(chainConfig, db)
	bank = gen.AddOwnedAccount(BankKey)
	acct1 = gen.AddOwnedAccount(Account1Key)
	acct2 = gen.AddOwnedAccount(Account2Key)
	gen.AddContract("Test", contractSpec)

	rng := rand.New(rand.NewSource(0))
	gen.AddFunction(func(i int, block *core.BlockGen) {
		if err := genChain(gen, i, block, rng); err != nil {
			panic(err)
		}
	})
	gen.Genesis = helpers.GenesisBlockForTesting(
		db, bank, BankFunds, big.NewInt(params.InitialBaseFee), params.MaxGasLimit,
	)
	return gen
}

func genChain(gen *chaingen.GenContext, i int, block *core.BlockGen, rng *rand.Rand) error {
	var err error
	switch i {
	case 0: // In block 1, the test bank sends account #1 and #2 some ether.
		amt := 1e15 + rng.Int63n(1e15)
		tx1, err := gen.CreateSendTx(bank, acct1, big.NewInt(amt))
		if err != nil {
			panic(err)
		}
		block.AddTx(tx1)
		tx2, err := gen.CreateSendTx(bank, acct2, big.NewInt(amt))
		if err != nil {
			panic(err)
		}
		block.AddTx(tx2)

	case 1: // Block 2: deploy contracts
		caddr1, err = gen.DeployContract(acct1, "Test")
		if err != nil {
			panic(err)
		}
		caddr2, err = gen.DeployContract(acct2, "Test")
		if err != nil {
			panic(err)
		}
	case 2: // Block 3: call contract functions from bank
		block.SetCoinbase(acct1)

		value := rng.Int63n(1000)
		tx1, err := gen.CreateCallTx(bank, caddr1, "Put", big.NewInt(value))
		if err != nil {
			panic(err)
		}
		block.AddTx(tx1)

		value = rng.Int63n(1000)
		tx2, err := gen.CreateCallTx(bank, caddr2, "Put", big.NewInt(value))
		if err != nil {
			panic(err)
		}
		block.AddTx(tx2)

	case 3: // Block 4: call contract functions from accounts 1 & 2
		block.SetCoinbase(acct2)

		value := rng.Int63n(1000)
		tx1, err := gen.CreateCallTx(acct1, caddr2, "Put", big.NewInt(value))
		if err != nil {
			panic(err)
		}
		block.AddTx(tx1)

		value = rng.Int63n(1000)
		tx2, err := gen.CreateCallTx(acct2, caddr1, "Put", big.NewInt(value))
		if err != nil {
			panic(err)
		}
		block.AddTx(tx2)
	}
	return nil
}
