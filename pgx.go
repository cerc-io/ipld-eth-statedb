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

// NewPGXDriverFromPool returns a new pgx driver for Postgres
func NewPGXDriverFromPool(ctx context.Context, db *pgxpool.Pool) *PGXDriver {
	return &PGXDriver{ctx: ctx, db: db}
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
