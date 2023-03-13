package ipld_eth_statedb

import (
	"context"
)

// Database interfaces to support multiple Postgres drivers
type Database interface {
	Driver
	Statements
}

// Driver interface has all the methods required by a driver implementation to support the sql indexer
type Driver interface {
	QueryRow(ctx context.Context, sql string, args ...interface{}) ScannableRow
	Exec(ctx context.Context, sql string, args ...interface{}) (Result, error)
}

// ScannableRow interface to accommodate different concrete row types
type ScannableRow interface {
	Scan(dest ...interface{}) error
}

// Result interface to accommodate different concrete result types
type Result interface {
	RowsAffected() (int64, error)
}

// Statements interface to accommodate different SQL query syntax
type Statements interface {
	GetContractCodeStmt() string
	GetStateAccountStmt() string
	GetStorageSlotStmt() string
}
