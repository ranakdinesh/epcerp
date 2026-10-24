package infrastructure

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ranakdinesh/epcERP/internal/config"
	"time"
)

func poolConfig(settings config.Config) (*pgxpool.Config, error) {
	parsed, err := pgxpool.ParseConfig(settings.DatabaseURL())
	if err != nil {
		return nil, errors.New("invalid database configuration")
	}
	parsed.MinConns = int32(settings.DatabasePoolSize)
	parsed.MaxConns = int32(settings.DatabasePoolSize + settings.DatabaseMaxOverflow)
	parsed.MaxConnLifetime = 30 * time.Minute
	parsed.ConnConfig.ConnectTimeout = 5 * time.Second
	return parsed, nil
}
func OpenDatabase(ctx context.Context, settings config.Config) (*pgxpool.Pool, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	parsed, err := poolConfig(settings)
	if err != nil {
		return nil, err
	}
	pool, err := pgxpool.NewWithConfig(ctx, parsed)
	if err != nil {
		return nil, errors.New("database pool could not be created")
	}
	// Verify connectivity before handing the pool to the application.
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, errors.New("database connection failed")
	}
	return pool, nil
}
