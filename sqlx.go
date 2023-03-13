package ipld_eth_statedb

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

// NewSQLXDriver returns a new sqlx driver for Postgres
func NewSQLXDriver(ctx context.Context, config Config) (*SQLXDriver, error) {
	db, err := NewSQLXPool(ctx, config)
	if err != nil {
		return nil, err
	}
	if config.MaxConns > 0 {
		db.SetMaxOpenConns(config.MaxConns)
	}
	if config.MaxConnLifetime > 0 {
		db.SetConnMaxLifetime(config.MaxConnLifetime)
	}
	db.SetMaxIdleConns(config.MaxIdle)
	return &SQLXDriver{ctx: ctx, db: db}, nil
}

// NewSQLXDriverFromPool returns a new sqlx driver for Postgres
func NewSQLXDriverFromPool(ctx context.Context, db *sqlx.DB) (*SQLXDriver, error) {
	return &SQLXDriver{ctx: ctx, db: db}, nil
}

// QueryRow satisfies sql.Database
func (driver *SQLXDriver) QueryRow(_ context.Context, sql string, args ...interface{}) ScannableRow {
	return driver.db.QueryRowx(sql, args...)
}

// Exec satisfies sql.Database
func (driver *SQLXDriver) Exec(_ context.Context, sql string, args ...interface{}) (Result, error) {
	return driver.db.Exec(sql, args...)
}
