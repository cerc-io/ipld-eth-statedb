package ipld_eth_statedb

import (
	"context"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4/pgxpool"
)

var _ Driver = &PGXDriver{}

// PGXDriver driver, implements Driver
type PGXDriver struct {
	ctx context.Context
	db  *pgxpool.Pool
}

// NewPGXDriver returns a new pgx driver for Postgres
func NewPGXDriver(ctx context.Context, config Config) (*PGXDriver, error) {
	db, err := NewPGXPool(ctx, config)
	if err != nil {
		return nil, err
	}
	return &PGXDriver{ctx: ctx, db: db}, nil
}

// NewPGXDriverFromPool returns a new pgx driver for Postgres
func NewPGXDriverFromPool(ctx context.Context, db *pgxpool.Pool) (*PGXDriver, error) {
	return &PGXDriver{ctx: ctx, db: db}, nil
}

// QueryRow satisfies sql.Database
func (driver *PGXDriver) QueryRow(ctx context.Context, sql string, args ...interface{}) ScannableRow {
	return driver.db.QueryRow(ctx, sql, args...)
}

// Exec satisfies sql.Database
func (pgx *PGXDriver) Exec(ctx context.Context, sql string, args ...interface{}) (Result, error) {
	res, err := pgx.db.Exec(ctx, sql, args...)
	return resultWrapper{ct: res}, err
}

type resultWrapper struct {
	ct pgconn.CommandTag
}

// RowsAffected satisfies sql.Result
func (r resultWrapper) RowsAffected() (int64, error) {
	return r.ct.RowsAffected(), nil
}
