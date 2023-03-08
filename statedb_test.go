package ipld_eth_statedb_test

import (
	"context"
	"math/big"
	"os"
	"strconv"
	"testing"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/multiformats/go-multihash"
	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/statediff/indexer/ipld"

	statedb "github.com/cerc-io/ipld-eth-statedb"
	util "github.com/cerc-io/ipld-eth-statedb/internal"
)

var (
	testCtx = context.Background()

	// Fixture data
	BlockNumber     = big.NewInt(1337)
	Header          = types.Header{Number: BlockNumber}
	BlockHash       = Header.Hash()
	RandomHash      = crypto.Keccak256Hash([]byte("I am a random hash"))
	RandomHash2     = crypto.Keccak256Hash([]byte("I am another random hash"))
	RandomHash3     = crypto.Keccak256Hash([]byte("I am"))
	RandomHash4     = crypto.Keccak256Hash([]byte("I"))
	BlockParentHash = common.HexToHash("0123456701234567012345670123456701234567012345670123456701234567")

	AccountPK, _   = crypto.HexToECDSA("8a1f9a8f95be41cd7ccb6168179afb4504aefe388d1e14474d32c45c72ce7b7a")
	AccountAddress = crypto.PubkeyToAddress(AccountPK.PublicKey) //0x703c4b2bD70c169f5717101CaeE543299Fc946C7
	AccountLeafKey = crypto.Keccak256Hash(AccountAddress.Bytes())

	AccountCode     = []byte{0, 1, 2, 3, 4, 5, 6, 7}
	AccountCodeHash = crypto.Keccak256Hash(AccountCode)

	Account = types.StateAccount{
		Nonce:    uint64(0),
		Balance:  big.NewInt(1000),
		CodeHash: AccountCodeHash.Bytes(),
		Root:     common.Hash{},
	}

	StorageLeafKey     = crypto.Keccak256Hash(common.HexToHash("0").Bytes())
	StoredValue        = crypto.Keccak256Hash([]byte{1, 2, 3, 4, 5})
	StoragePartialPath = []byte{0, 1, 0, 2, 0, 4}

	// Encoded data
	headerRLP, _ = rlp.EncodeToBytes(&Header)
	HeaderCID, _ = ipld.RawdataToCid(ipld.MEthStateTrie, headerRLP, multihash.KECCAK_256)

	accountRLP, _        = rlp.EncodeToBytes(&Account)
	accountAndLeafRLP, _ = rlp.EncodeToBytes(&[]interface{}{AccountLeafKey, accountRLP})
	AccountCID, _        = ipld.RawdataToCid(ipld.MEthStateTrie, accountAndLeafRLP, multihash.KECCAK_256)
	AccountCodeCID, _    = util.Keccak256ToCid(ipld.RawBinary, AccountCodeHash.Bytes())

	StoredValueRLP, _  = rlp.EncodeToBytes(StoredValue)
	StoredValueRLP2, _ = rlp.EncodeToBytes("something")
	StorageRLP, _      = rlp.EncodeToBytes(&[]interface{}{StoragePartialPath, StoredValueRLP})
	StorageCID, _      = ipld.RawdataToCid(ipld.MEthStorageTrie, StorageRLP, multihash.KECCAK_256)

	RemovedNodeStateCID   = "baglacgzayxjemamg64rtzet6pwznzrydydsqbnstzkbcoo337lmaixmfurya"
	RemovedNodeStorageCID = "bagmacgzayxjemamg64rtzet6pwznzrydydsqbnstzkbcoo337lmaixmfurya"
)

func TestSuite(t *testing.T) {
	testConfig, err := getTestConfig()
	require.NoError(t, err)

	pool, err := statedb.NewPGXPool(testCtx, testConfig)
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

	require.NoError(t, insertHeaderCID(pool, BlockHash.String(), BlockNumber.Uint64()))
	require.NoError(t, insertHeaderCID(pool, RandomHash.String(), BlockNumber.Uint64()+1))
	require.NoError(t, insertHeaderCID(pool, RandomHash2.String(), BlockNumber.Uint64()+2))
	require.NoError(t, insertHeaderCID(pool, RandomHash3.String(), BlockNumber.Uint64()+3))
	require.NoError(t, insertHeaderCID(pool, RandomHash4.String(), BlockNumber.Uint64()+4))
	require.NoError(t, insertStateCID(pool, stateModel{
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
	require.NoError(t, insertStateCID(pool, stateModel{
		BlockNumber: BlockNumber.Uint64() + 4,
		BlockHash:   RandomHash4.String(),
		LeafKey:     AccountLeafKey.String(),
		CID:         RemovedNodeStorageCID,
		Diff:        true,
		Removed:     true,
	}))
	require.NoError(t, insertStorageCID(pool, storageModel{
		BlockNumber:    BlockNumber.Uint64(),
		BlockHash:      BlockHash.String(),
		LeafKey:        AccountLeafKey.String(),
		StorageLeafKey: StorageLeafKey.String(),
		StorageCID:     StorageCID.String(),
		Diff:           true,
		Value:          StoredValueRLP,
		Removed:        false,
	}))
	require.NoError(t, insertStorageCID(pool, storageModel{
		BlockNumber:    BlockNumber.Uint64() + 1,
		BlockHash:      RandomHash.String(),
		LeafKey:        AccountLeafKey.String(),
		StorageLeafKey: StorageLeafKey.String(),
		StorageCID:     RemovedNodeStateCID,
		Diff:           true,
		Value:          []byte{},
		Removed:        true,
	}))
	require.NoError(t, insertStorageCID(pool, storageModel{
		BlockNumber:    BlockNumber.Uint64() + 2,
		BlockHash:      RandomHash2.String(),
		LeafKey:        AccountLeafKey.String(),
		StorageLeafKey: StorageLeafKey.String(),
		StorageCID:     StorageCID.String(),
		Diff:           true,
		Value:          StoredValueRLP2,
		Removed:        false,
	}))
	require.NoError(t, insertContractCode(pool))

	db, err := statedb.NewStateDatabaseWithPool(pool)
	require.NoError(t, err)

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

		acct2, err := db.StateAccount(AccountLeafKey, RandomHash)
		require.NoError(t, err)
		require.Equal(t, &Account, acct2)

		acct3, err := db.StateAccount(AccountLeafKey, RandomHash2)
		require.NoError(t, err)
		require.Equal(t, &Account, acct3)

		acct4, err := db.StateAccount(AccountLeafKey, RandomHash3)
		require.NoError(t, err)
		require.Equal(t, &Account, acct4)

		acct5, err := db.StateAccount(AccountLeafKey, RandomHash4)
		require.NoError(t, err)
		require.Equal(t, (*types.StateAccount)(nil), acct5)

		val, err := db.StorageValue(AccountLeafKey, StorageLeafKey, BlockHash)
		require.NoError(t, err)
		require.Equal(t, StoredValueRLP, val)

		val2, err := db.StorageValue(AccountLeafKey, StorageLeafKey, RandomHash)
		require.NoError(t, err)
		require.Equal(t, ([]byte)(nil), val2)

		val3, err := db.StorageValue(AccountLeafKey, StorageLeafKey, RandomHash2)
		require.NoError(t, err)
		require.Equal(t, StoredValueRLP2, val3)

		val4, err := db.StorageValue(AccountLeafKey, StorageLeafKey, RandomHash3)
		require.NoError(t, err)
		require.Equal(t, StoredValueRLP2, val4)

		val5, err := db.StorageValue(AccountLeafKey, StorageLeafKey, RandomHash4)
		require.NoError(t, err)
		require.Equal(t, ([]byte)(nil), val5)
	})

	t.Run("StateDB", func(t *testing.T) {
		sdb, err := statedb.New(BlockHash, db)
		require.NoError(t, err)

		checkAccountUnchanged := func() {
			require.Equal(t, Account.Balance, sdb.GetBalance(AccountAddress))
			require.Equal(t, Account.Nonce, sdb.GetNonce(AccountAddress))
			require.Equal(t, StoredValue, sdb.GetState(AccountAddress, StorageLeafKey))
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
		sdb.SetState(AccountAddress, StorageLeafKey, newStorage)
		sdb.SetCode(AccountAddress, newCode)

		require.Equal(t, big.NewInt(400), sdb.GetBalance(AccountAddress))
		require.Equal(t, uint64(42), sdb.GetNonce(AccountAddress))
		require.Equal(t, newStorage, sdb.GetState(AccountAddress, StorageLeafKey))
		require.Equal(t, newCode, sdb.GetCode(AccountAddress))

		sdb.AddSlotToAccessList(AccountAddress, StorageLeafKey)
		require.True(t, sdb.AddressInAccessList(AccountAddress))
		hasAddr, hasSlot := sdb.SlotInAccessList(AccountAddress, StorageLeafKey)
		require.True(t, hasAddr)
		require.True(t, hasSlot)

		sdb.RevertToSnapshot(id)

		checkAccountUnchanged()
		require.False(t, sdb.AddressInAccessList(AccountAddress))
		hasAddr, hasSlot = sdb.SlotInAccessList(AccountAddress, StorageLeafKey)
		require.False(t, hasAddr)
		require.False(t, hasSlot)
	})
}

func insertHeaderCID(db *pgxpool.Pool, blockHash string, blockNumber uint64) error {
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
	_, err := db.Exec(testCtx, sql,
		blockNumber,
		blockHash,
		BlockParentHash.String(),
		HeaderCID.String(),
		0, []string{}, 0,
		Header.Root.String(),
		Header.TxHash.String(),
		Header.ReceiptHash.String(),
		common.HexToHash("0").String(),
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

func insertStateCID(db *pgxpool.Pool, cidModel stateModel) error {
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

func insertStorageCID(db *pgxpool.Pool, cidModel storageModel) error {
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

func insertContractCode(db *pgxpool.Pool) error {
	sql := `INSERT INTO ipld.blocks (block_number, key, data) VALUES ($1, $2, $3)`
	_, err := db.Exec(testCtx, sql, BlockNumber.Uint64(), AccountCodeCID, AccountCode)
	return err
}

func getTestConfig() (conf statedb.Config, err error) {
	port, err := strconv.Atoi(os.Getenv("DATABASE_PORT"))
	if err != nil {
		return
	}
	return statedb.Config{
		Hostname:     os.Getenv("DATABASE_HOSTNAME"),
		DatabaseName: os.Getenv("DATABASE_NAME"),
		Username:     os.Getenv("DATABASE_USER"),
		Password:     os.Getenv("DATABASE_PASSWORD"),
		Port:         port,
	}, nil
}
