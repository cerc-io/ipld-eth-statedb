package ipld_eth_statedb

const (
	GetContractCodePgStr = `SELECT data FROM ipld.blocks WHERE key = $1`
	GetStateAccount      = `SELECT balance, nonce, code_hash, storage_root, removed FROM eth.state_cids
						INNER JOIN eth.header_cids ON (
							state_cids.header_id = header_cids.block_hash
							AND state_cids.block_number = header_cids.block_number
						)
						WHERE state_leaf_key = $1
						AND header_cids.block_number <= (SELECT block_number
															FROM eth.header_cids
															WHERE block_hash = $2)
						AND header_cids.block_hash = (SELECT canonical_header_hash(header_cids.block_number))
						ORDER BY header_cids.block_number DESC
						LIMIT 1`
	GetStorageSlot = `SELECT val, removed FROM eth.storage_cids
						INNER JOIN eth.header_cids ON (
							storage_cids.header_id = header_cids.block_hash
							AND storage_cids.block_number = header_cids.block_number
						)
						WHERE state_leaf_key = $1
						AND storage_leaf_key = $2
						AND header_cids.block_number <= (SELECT block_number
															FROM eth.header_cids
															WHERE block_hash = $3)
						AND header_cids.block_hash = (SELECT canonical_header_hash(header_cids.block_number))
						ORDER BY header_cids.block_number DESC
						LIMIT 1`
)

// StorageSlotResult struct for unpacking GetStorageSlot result
type StorageSlotResult struct {
	Value   []byte `db:"val"`
	Removed bool   `db:"removed"`
}

// StateAccountResult struct for unpacking GetStateAccount result
type StateAccountResult struct {
	Balance     string `db:"balance"`
	Nonce       uint64 `db:"nonce"`
	CodeHash    string `db:"code_hash"`
	StorageRoot string `db:"storage_root"`
	Removed     bool   `db:"removed"`
}
