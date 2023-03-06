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
	BlockParentHash = common.HexToHash("0123456701234567012345670123456701234567012345670123456701234567")

	AccountPK, _   = crypto.HexToECDSA("8a1f9a8f95be41cd7ccb6168179afb4504aefe388d1e14474d32c45c72ce7b7a")
	AccountAddress = crypto.PubkeyToAddress(AccountPK.PublicKey) //0x703c4b2bD70c169f5717101CaeE543299Fc946C7
	AccountLeafKey = crypto.Keccak256Hash(AccountAddress.Bytes())
	AccountPath    = []byte{AccountLeafKey[0] & 0xf0} // first nibble of path

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

	StoredValueRLP, _ = rlp.EncodeToBytes(StoredValue)
	StorageRLP, _     = rlp.EncodeToBytes(&[]interface{}{StoragePartialPath, StoredValueRLP})
	StorageCID, _     = ipld.RawdataToCid(ipld.MEthStorageTrie, StorageRLP, multihash.KECCAK_256)
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

	require.NoError(t, insertHeaderCID(pool))
	require.NoError(t, insertStateCID(pool))
	require.NoError(t, insertStorageCID(pool))
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

		val, err := db.StorageValue(AccountLeafKey, StorageLeafKey, BlockHash)
		require.NoError(t, err)
		require.Equal(t, StoredValueRLP, val)
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

func insertHeaderCID(db *pgxpool.Pool) error {
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
		BlockNumber.Uint64(),
		BlockHash.String(),
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

func insertStateCID(db *pgxpool.Pool) error {
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
		BlockNumber.Uint64(),
		BlockHash.String(),
		AccountLeafKey.String(),
		AccountCID.String(),
		false,
		Account.Balance.Uint64(),
		Account.Nonce,
		AccountCodeHash.String(),
		Account.Root.String(),
		false,
	)
	return err
}

func insertStorageCID(db *pgxpool.Pool) error {
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
		BlockNumber.Uint64(),
		BlockHash.String(),
		AccountLeafKey.String(),
		StorageLeafKey.String(),
		StorageCID.String(),
		false,
		StoredValueRLP,
		false,
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
