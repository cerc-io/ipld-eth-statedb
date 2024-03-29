package sql

import (
	"context"

	"github.com/jmoiron/sqlx"
)

var _ Driver = &SQLXDriver{}

// SQLXDriver driver, implements Driver
type SQLXDriver struct {
	ctx context.Context
	db  *sqlx.DB
}

// NewSQLXDriverFromPool returns a new sqlx driver for Postgres
func NewSQLXDriverFromPool(ctx context.Context, db *sqlx.DB) *SQLXDriver {
	return &SQLXDriver{ctx: ctx, db: db}
}

// QueryRow satisfies sql.Database
func (driver *SQLXDriver) QueryRow(_ context.Context, sql string, args ...interface{}) ScannableRow {
	return driver.db.QueryRowx(sql, args...)
}

// Exec satisfies sql.Database
func (driver *SQLXDriver) Exec(_ context.Context, sql string, args ...interface{}) (Result, error) {
	return driver.db.Exec(sql, args...)
}
