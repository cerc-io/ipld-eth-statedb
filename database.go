package ipld_eth_statedb

var _ Database = &DB{}

// NewPostgresDB returns a postgres.DB using the provided driver
func NewPostgresDB(driver Driver) *DB {
	return &DB{driver}
}

// DB implements sql.Database using a configured driver and Postgres statement syntax
type DB struct {
	Driver
}

// GetContractCodeStmt satisfies the Statements interface
func (db *DB) GetContractCodeStmt() string {
	return GetContractCodePgStr
}

// GetStateAccountStmt satisfies the Statements interface
func (db *DB) GetStateAccountStmt() string {
	return GetStateAccount
}

// GetStorageSlotStmt satisfies the Statements interface
func (db *DB) GetStorageSlotStmt() string {
	return GetStorageSlot
}
