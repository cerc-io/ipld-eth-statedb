package ipld_eth_statedb

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

type Config struct {
	Hostname     string
	Port         int
	DatabaseName string
	Username     string
	Password     string

	ConnTimeout     time.Duration
	MaxConns        int
	MinConns        int
	MaxConnLifetime time.Duration
	MaxConnIdleTime time.Duration
	MaxIdle         int
}

// DbConnectionString constructs and returns the connection string from the config (for sqlx driver)
func (c Config) DbConnectionString() string {
	if len(c.Username) > 0 && len(c.Password) > 0 {
		return fmt.Sprintf("postgresql://%s:%s@%s:%d/%s?sslmode=disable",
			c.Username, c.Password, c.Hostname, c.Port, c.DatabaseName)
	}
	if len(c.Username) > 0 && len(c.Password) == 0 {
		return fmt.Sprintf("postgresql://%s@%s:%d/%s?sslmode=disable",
			c.Username, c.Hostname, c.Port, c.DatabaseName)
	}
	return fmt.Sprintf("postgresql://%s:%d/%s?sslmode=disable", c.Hostname, c.Port, c.DatabaseName)
}

// NewPGXPool returns a new pgx conn pool
func NewPGXPool(ctx context.Context, config Config) (*pgxpool.Pool, error) {
	pgConf, err := makePGXConfig(config)
	if err != nil {
		return nil, err
	}
	return pgxpool.ConnectConfig(ctx, pgConf)
}

// NewSQLXPool returns a new sqlx conn pool
func NewSQLXPool(ctx context.Context, config Config) (*sqlx.DB, error) {
	db, err := sqlx.ConnectContext(ctx, "postgres", config.DbConnectionString())
	if err != nil {
		return nil, err
	}
	return db, nil
}

// makePGXConfig creates a pgxpool.Config from the provided Config
func makePGXConfig(config Config) (*pgxpool.Config, error) {
	conf, err := pgxpool.ParseConfig("")
	if err != nil {
		return nil, err
	}

	conf.ConnConfig.Config.Host = config.Hostname
	conf.ConnConfig.Config.Port = uint16(config.Port)
	conf.ConnConfig.Config.Database = config.DatabaseName
	conf.ConnConfig.Config.User = config.Username
	conf.ConnConfig.Config.Password = config.Password

	if config.ConnTimeout != 0 {
		conf.ConnConfig.Config.ConnectTimeout = config.ConnTimeout
	}
	if config.MaxConns != 0 {
		conf.MaxConns = int32(config.MaxConns)
	}
	if config.MinConns != 0 {
		conf.MinConns = int32(config.MinConns)
	}
	if config.MaxConnLifetime != 0 {
		conf.MaxConnLifetime = config.MaxConnLifetime
	}
	if config.MaxConnIdleTime != 0 {
		conf.MaxConnIdleTime = config.MaxConnIdleTime
	}

	return conf, nil
}
