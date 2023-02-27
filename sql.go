package ipld_eth_statedb

const (
	GetBalancePgStr = `SELECT balance FROM eth.state_cids
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
	GetContractCodePgStr = `SELECT data FROM public.blocks WHERE key = $1`
)
