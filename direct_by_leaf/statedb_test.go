package state_test

import (
	"context"
	"math/big"
	"testing"

	"github.com/lib/pq"
	"github.com/multiformats/go-multihash"
	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/statediff/indexer/database/sql/postgres"
	"github.com/ethereum/go-ethereum/statediff/indexer/ipld"

	state "github.com/cerc-io/ipld-eth-statedb/direct_by_leaf"
	util "github.com/cerc-io/ipld-eth-statedb/internal"
	"github.com/cerc-io/ipld-eth-statedb/sql"
)

var (
	testCtx = context.Background()

	// Fixture data
	// block one: contract account and slot are created
	// block two: slot is emptied
	// block three: slot has a new value added
	// block four: non-canonical block with non-canonical slot value is added to the database
	// block five: entire contract is destructed; another non-canonical block is created but it doesn't include an update for our slot
	// but it links back to the other non-canonical header (this is to test the ability to resolve canonicity by comparing how many
	// children reference back to a header)
	// block six: canonical block only, no relevant state changes (check that we still return emptied result at heights where it wasn't emptied)
	BlockNumber     = big.NewInt(1337)
	Header          = types.Header{Number: BlockNumber}
	BlockHash       = Header.Hash()
	BlockHash2      = crypto.Keccak256Hash([]byte("I am a random hash"))
	BlockNumber2    = BlockNumber.Uint64() + 1
	BlockHash3      = crypto.Keccak256Hash([]byte("I am another random hash"))
	BlockNumber3    = BlockNumber.Uint64() + 2
	BlockHash4      = crypto.Keccak256Hash([]byte("I am"))
	BlockNumber4    = BlockNumber.Uint64() + 3
	BlockHash5      = crypto.Keccak256Hash([]byte("I"))
	BlockNumber5    = BlockNumber.Uint64() + 4
	BlockHash6      = crypto.Keccak256Hash([]byte("am"))
	BlockNumber6    = BlockNumber.Uint64() + 5
	BlockParentHash = common.HexToHash("0123456701234567012345670123456701234567012345670123456701234567")

	NonCanonicalHash4 = crypto.Keccak256Hash([]byte("I am a random non canonical hash"))
	NonCanonicalHash5 = crypto.Keccak256Hash([]byte("I am also a random non canonical hash"))

	AccountPK, _   = crypto.HexToECDSA("8a1f9a8f95be41cd7ccb6168179afb4504aefe388d1e14474d32c45c72ce7b7a")
	AccountAddress = crypto.PubkeyToAddress(AccountPK.PublicKey) //0x703c4b2bD70c169f5717101CaeE543299Fc946C7
	AccountLeafKey = crypto.Keccak256Hash(AccountAddress[:])

	AccountCode     = []byte{0, 1, 2, 3, 4, 5, 6, 7}
	AccountCodeHash = crypto.Keccak256Hash(AccountCode)

	Account = types.StateAccount{
		Nonce:    uint64(0),
		Balance:  big.NewInt(1000),
		CodeHash: AccountCodeHash.Bytes(),
		Root:     common.Hash{},
	}

	StorageSlot        = common.HexToHash("0")
	StorageLeafKey     = crypto.Keccak256Hash(StorageSlot[:])
	StoredValue        = crypto.Keccak256Hash([]byte{1, 2, 3, 4, 5})
	StoragePartialPath = []byte{0, 1, 0, 2, 0, 4}

	// Encoded data
	accountRLP, _        = rlp.EncodeToBytes(&Account)
	accountAndLeafRLP, _ = rlp.EncodeToBytes(&[]interface{}{AccountLeafKey, accountRLP})
	AccountCID, _        = ipld.RawdataToCid(ipld.MEthStateTrie, accountAndLeafRLP, multihash.KECCAK_256)
	AccountCodeCID, _    = util.Keccak256ToCid(ipld.RawBinary, AccountCodeHash[:])

	StoredValueRLP, _         = rlp.EncodeToBytes(StoredValue)
	StoredValueRLP2, _        = rlp.EncodeToBytes("something")
	NonCanonStoredValueRLP, _ = rlp.EncodeToBytes("something else")
	StorageRLP, _             = rlp.EncodeToBytes(&[]interface{}{StoragePartialPath, StoredValueRLP})
	StorageCID, _             = ipld.RawdataToCid(ipld.MEthStorageTrie, StorageRLP, multihash.KECCAK_256)

	RemovedNodeStateCID   = "baglacgzayxjemamg64rtzet6pwznzrydydsqbnstzkbcoo337lmaixmfurya"
	RemovedNodeStorageCID = "bagmacgzayxjemamg64rtzet6pwznzrydydsqbnstzkbcoo337lmaixmfurya"
)

func TestPGXSuite(t *testing.T) {
	testConfig, err := postgres.DefaultConfig.WithEnv()
	require.NoError(t, err)
	pool, err := postgres.ConnectPGX(testCtx, testConfig)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		tx, err := pool.Begin(testCtx)
		require.NoError(t, err)
		statements := []string{
			`DELETE FROM eth.header_cids`,
			`DELETE FROM eth.state_cids`,
			`DELETE FROM eth.storage_cids`,
			`DELETE FROM ipld.blocks`,
		}
		for _, stm := range statements {
			_, err = tx.Exec(testCtx, stm)
			require.NoErrorf(t, err, "Exec(`%s`)", stm)
		}
		require.NoError(t, tx.Commit(testCtx))
	})

	database := sql.NewPGXDriverFromPool(context.Background(), pool)
	insertSuiteData(t, database)

	db := state.NewStateDatabase(database)
	require.NoError(t, err)
	testSuite(t, db)
}

func TestSQLXSuite(t *testing.T) {
	testConfig, err := postgres.DefaultConfig.WithEnv()
	require.NoError(t, err)
	pool, err := postgres.ConnectSQLX(testCtx, testConfig)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		tx, err := pool.Begin()
		require.NoError(t, err)
		statements := []string{
			`DELETE FROM eth.header_cids`,
			`DELETE FROM eth.state_cids`,
			`DELETE FROM eth.storage_cids`,
			`DELETE FROM ipld.blocks`,
		}
		for _, stm := range statements {
			_, err = tx.Exec(stm)
			require.NoErrorf(t, err, "Exec(`%s`)", stm)
		}
		require.NoError(t, tx.Commit())
	})

	database := sql.NewSQLXDriverFromPool(context.Background(), pool)
	insertSuiteData(t, database)

	db := state.NewStateDatabase(database)
	require.NoError(t, err)
	testSuite(t, db)
}

func insertSuiteData(t *testing.T, database sql.Database) {
	require.NoError(t, insertHeaderCID(database, BlockHash.String(), BlockParentHash.String(), BlockNumber.Uint64()))
	require.NoError(t, insertHeaderCID(database, BlockHash2.String(), BlockHash.String(), BlockNumber2))
	require.NoError(t, insertHeaderCID(database, BlockHash3.String(), BlockHash2.String(), BlockNumber3))
	require.NoError(t, insertHeaderCID(database, BlockHash4.String(), BlockHash3.String(), BlockNumber4))
	require.NoError(t, insertHeaderCID(database, NonCanonicalHash4.String(), BlockHash3.String(), BlockNumber4))
	require.NoError(t, insertHeaderCID(database, BlockHash5.String(), BlockHash4.String(), BlockNumber5))
	require.NoError(t, insertHeaderCID(database, NonCanonicalHash5.String(), NonCanonicalHash4.String(), BlockNumber5))
	require.NoError(t, insertHeaderCID(database, BlockHash6.String(), BlockHash5.String(), BlockNumber6))
	require.NoError(t, insertStateCID(database, stateModel{
		BlockNumber: BlockNumber.Uint64(),
		BlockHash:   BlockHash.String(),
		LeafKey:     AccountLeafKey.String(),
		CID:         AccountCID.String(),
		Diff:        true,
		Balance:     Account.Balance.Uint64(),
		Nonce:       Account.Nonce,
		CodeHash:    AccountCodeHash.String(),
		StorageRoot: Account.Root.String(),
		Removed:     false,
	}))
	require.NoError(t, insertStateCID(database, stateModel{
		BlockNumber: BlockNumber4,
		BlockHash:   NonCanonicalHash4.String(),
		LeafKey:     AccountLeafKey.String(),
		CID:         AccountCID.String(),
		Diff:        true,
		Balance:     big.NewInt(123).Uint64(),
		Nonce:       Account.Nonce,
		CodeHash:    AccountCodeHash.String(),
		StorageRoot: Account.Root.String(),
		Removed:     false,
	}))
	require.NoError(t, insertStateCID(database, stateModel{
		BlockNumber: BlockNumber5,
		BlockHash:   BlockHash5.String(),
		LeafKey:     AccountLeafKey.String(),
		CID:         RemovedNodeStateCID,
		Diff:        true,
		Removed:     true,
	}))
	require.NoError(t, insertStorageCID(database, storageModel{
		BlockNumber:    BlockNumber.Uint64(),
		BlockHash:      BlockHash.String(),
		LeafKey:        AccountLeafKey.String(),
		StorageLeafKey: StorageLeafKey.String(),
		StorageCID:     StorageCID.String(),
		Diff:           true,
		Value:          StoredValueRLP,
		Removed:        false,
	}))
	require.NoError(t, insertStorageCID(database, storageModel{
		BlockNumber:    BlockNumber2,
		BlockHash:      BlockHash2.String(),
		LeafKey:        AccountLeafKey.String(),
		StorageLeafKey: StorageLeafKey.String(),
		StorageCID:     RemovedNodeStorageCID,
		Diff:           true,
		Value:          []byte{},
		Removed:        true,
	}))
	require.NoError(t, insertStorageCID(database, storageModel{
		BlockNumber:    BlockNumber3,
		BlockHash:      BlockHash3.String(),
		LeafKey:        AccountLeafKey.String(),
		StorageLeafKey: StorageLeafKey.String(),
		StorageCID:     StorageCID.String(),
		Diff:           true,
		Value:          StoredValueRLP2,
		Removed:        false,
	}))
	require.NoError(t, insertStorageCID(database, storageModel{
		BlockNumber:    BlockNumber4,
		BlockHash:      NonCanonicalHash4.String(),
		LeafKey:        AccountLeafKey.String(),
		StorageLeafKey: StorageLeafKey.String(),
		StorageCID:     StorageCID.String(),
		Diff:           true,
		Value:          NonCanonStoredValueRLP,
		Removed:        false,
	}))
	require.NoError(t, insertContractCode(database))
}

func testSuite(t *testing.T, db state.StateDatabase) {
	t.Run("Database", func(t *testing.T) {
		size, err := db.ContractCodeSize(AccountCodeHash)
		require.NoError(t, err)
		require.Equal(t, len(AccountCode), size)

		code, err := db.ContractCode(AccountCodeHash)
		require.NoError(t, err)
		require.Equal(t, AccountCode, code)

		acct, err := db.StateAccount(AccountLeafKey, BlockHash)
		require.NoError(t, err)
		require.Equal(t, &Account, acct)

		acct2, err := db.StateAccount(AccountLeafKey, BlockHash2)
		require.NoError(t, err)
		require.Equal(t, &Account, acct2)

		acct3, err := db.StateAccount(AccountLeafKey, BlockHash3)
		require.NoError(t, err)
		require.Equal(t, &Account, acct3)

		// check that we don't get the non-canonical account
		acct4, err := db.StateAccount(AccountLeafKey, BlockHash4)
		require.NoError(t, err)
		require.Equal(t, &Account, acct4)

		acct5, err := db.StateAccount(AccountLeafKey, BlockHash5)
		require.NoError(t, err)
		require.Nil(t, acct5)

		acct6, err := db.StateAccount(AccountLeafKey, BlockHash6)
		require.NoError(t, err)
		require.Nil(t, acct6)

		val, err := db.StorageValue(AccountLeafKey, StorageLeafKey, BlockHash)
		require.NoError(t, err)
		require.Equal(t, StoredValueRLP, val)

		val2, err := db.StorageValue(AccountLeafKey, StorageLeafKey, BlockHash2)
		require.NoError(t, err)
		require.Nil(t, val2)

		val3, err := db.StorageValue(AccountLeafKey, StorageLeafKey, BlockHash3)
		require.NoError(t, err)
		require.Equal(t, StoredValueRLP2, val3)

		// this checks that we don't get the non-canonical result
		val4, err := db.StorageValue(AccountLeafKey, StorageLeafKey, BlockHash4)
		require.NoError(t, err)
		require.Equal(t, StoredValueRLP2, val4)

		// this checks that when the entire account was deleted, we return nil result for storage slot
		val5, err := db.StorageValue(AccountLeafKey, StorageLeafKey, BlockHash5)
		require.NoError(t, err)
		require.Nil(t, val5)

		val6, err := db.StorageValue(AccountLeafKey, StorageLeafKey, BlockHash6)
		require.NoError(t, err)
		require.Nil(t, val6)
	})

	t.Run("StateDB", func(t *testing.T) {
		sdb, err := state.New(BlockHash, db)
		require.NoError(t, err)

		checkAccountUnchanged := func() {
			require.Equal(t, Account.Balance, sdb.GetBalance(AccountAddress))
			require.Equal(t, Account.Nonce, sdb.GetNonce(AccountAddress))
			require.Equal(t, StoredValue, sdb.GetState(AccountAddress, StorageSlot))
			require.Equal(t, AccountCodeHash, sdb.GetCodeHash(AccountAddress))
			require.Equal(t, AccountCode, sdb.GetCode(AccountAddress))
			require.Equal(t, len(AccountCode), sdb.GetCodeSize(AccountAddress))
		}

		require.True(t, sdb.Exist(AccountAddress))
		checkAccountUnchanged()

		id := sdb.Snapshot()

		newStorage := crypto.Keccak256Hash([]byte{5, 4, 3, 2, 1})
		newCode := []byte{1, 3, 3, 7}

		sdb.SetBalance(AccountAddress, big.NewInt(300))
		sdb.AddBalance(AccountAddress, big.NewInt(200))
		sdb.SubBalance(AccountAddress, big.NewInt(100))
		sdb.SetNonce(AccountAddress, 42)
		sdb.SetState(AccountAddress, StorageSlot, newStorage)
		sdb.SetCode(AccountAddress, newCode)

		require.Equal(t, big.NewInt(400), sdb.GetBalance(AccountAddress))
		require.Equal(t, uint64(42), sdb.GetNonce(AccountAddress))
		require.Equal(t, newStorage, sdb.GetState(AccountAddress, StorageSlot))
		require.Equal(t, newCode, sdb.GetCode(AccountAddress))

		sdb.AddSlotToAccessList(AccountAddress, StorageSlot)
		require.True(t, sdb.AddressInAccessList(AccountAddress))
		hasAddr, hasSlot := sdb.SlotInAccessList(AccountAddress, StorageSlot)
		require.True(t, hasAddr)
		require.True(t, hasSlot)

		sdb.RevertToSnapshot(id)

		checkAccountUnchanged()
		require.False(t, sdb.AddressInAccessList(AccountAddress))
		hasAddr, hasSlot = sdb.SlotInAccessList(AccountAddress, StorageSlot)
		require.False(t, hasAddr)
		require.False(t, hasSlot)
	})
}

func insertHeaderCID(db sql.Database, blockHash, parentHash string, blockNumber uint64) error {
	cid, err := util.Keccak256ToCid(ipld.MEthHeader, common.HexToHash(blockHash).Bytes())
	if err != nil {
		return err
	}
	sql := `INSERT INTO eth.header_cids (
	block_number,
	block_hash,
	parent_hash,
	cid,
	td,
	node_ids,
	reward,
	state_root,
	tx_root,
	receipt_root,
	uncles_hash,
	bloom,
	timestamp,
	coinbase
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`
	_, err = db.Exec(testCtx, sql,
		blockNumber,
		blockHash,
		parentHash,
		cid.String(),
		0, pq.StringArray([]string{}), 0,
		Header.Root.String(),
		Header.TxHash.String(),
		Header.ReceiptHash.String(),
		Header.UncleHash.String(),
		[]byte{},
		Header.Time,
		Header.Coinbase.String(),
	)
	return err
}

type stateModel struct {
	BlockNumber uint64
	BlockHash   string
	LeafKey     string
	CID         string
	Diff        bool
	Balance     uint64
	Nonce       uint64
	CodeHash    string
	StorageRoot string
	Removed     bool
}

func insertStateCID(db sql.Database, cidModel stateModel) error {
	sql := `INSERT INTO eth.state_cids (
	block_number,
	header_id,
	state_leaf_key,
	cid,
	diff,
	balance,
	nonce,
	code_hash,
	storage_root,
	removed
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`
	_, err := db.Exec(testCtx, sql,
		cidModel.BlockNumber,
		cidModel.BlockHash,
		cidModel.LeafKey,
		cidModel.CID,
		cidModel.Diff,
		cidModel.Balance,
		cidModel.Nonce,
		cidModel.CodeHash,
		cidModel.StorageRoot,
		cidModel.Removed,
	)
	return err
}

type storageModel struct {
	BlockNumber    uint64
	BlockHash      string
	LeafKey        string
	StorageLeafKey string
	StorageCID     string
	Diff           bool
	Value          []byte
	Removed        bool
}

func insertStorageCID(db sql.Database, cidModel storageModel) error {
	sql := `INSERT INTO eth.storage_cids (
	block_number,
	header_id,
	state_leaf_key,
	storage_leaf_key,
	cid,
	diff,
	val,
	removed
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
	_, err := db.Exec(testCtx, sql,
		cidModel.BlockNumber,
		cidModel.BlockHash,
		cidModel.LeafKey,
		cidModel.StorageLeafKey,
		cidModel.StorageCID,
		cidModel.Diff,
		cidModel.Value,
		cidModel.Removed,
	)
	return err
}

func insertContractCode(db sql.Database) error {
	sql := `INSERT INTO ipld.blocks (block_number, key, data) VALUES ($1, $2, $3)`
	_, err := db.Exec(testCtx, sql, BlockNumber.Uint64(), AccountCodeCID.String(), AccountCode)
	return err
}
