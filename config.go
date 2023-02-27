package ipld_eth_statedb

import (
	"context"
	"time"

	"github.com/jackc/pgx/pgxpool"
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
}

// NewPGXPool returns a new pgx conn pool
func (c Config) NewPGXPool(ctx context.Context, config Config) (*pgxpool.Pool, error) {
	pgConf, err := makePGXConfig(config)
	if err != nil {
		return nil, err
	}
	return pgxpool.ConnectConfig(ctx, pgConf)
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
