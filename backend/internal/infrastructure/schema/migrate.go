// Package schema runs explicit operator-invoked migrations, never API startup DDL.
package schema

import (
	"context"
	"database/sql"
	"errors"
	"github.com/golang-migrate/migrate/v4"
	migratepgx "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/ranakdinesh/epcERP/sql/migrations"
	"net/url"
	"time"
)

const LatestVersion = 91

var ErrUnowned = errors.New("database is not an empty or epcERP-owned database")

// Open accepts only empty or epcERP-owned databases before migration DDL.
// Errors must be sanitized by the CLI; driver errors can contain connection secrets.
func Open(ctx context.Context, dsn string) (*migrate.Migrate, error) {
	parsed, err := url.Parse(dsn)
	if err != nil || (parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") {
		return nil, errors.New("use an epcERP PostgreSQL URL")
	}
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	cfg.ConnectTimeout = 10 * time.Second
	db := stdlib.OpenDB(*cfg)
	db.SetMaxOpenConns(2)
	if err = guard(ctx, db); err != nil {
		db.Close()
		return nil, err
	}
	driver, err := migratepgx.WithInstance(db, &migratepgx.Config{StatementTimeout: 30 * time.Second})
	if err != nil {
		db.Close()
		return nil, err
	}
	source, err := iofs.New(migrations.Files, ".")
	if err != nil {
		driver.Close()
		return nil, err
	}
	result, err := migrate.NewWithInstance("iofs", source, "pgx5", driver)
	if err != nil {
		source.Close()
		driver.Close()
		return nil, err
	}
	result.LockTimeout = 15 * time.Second
	return result, nil
}
func guard(ctx context.Context, db *sql.DB) error {
	var marker bool
	if err := db.QueryRowContext(ctx, `SELECT to_regclass('public.epcerp_schema_identity') IS NOT NULL`).Scan(&marker); err != nil {
		return err
	}
	if marker {
		var project string
		if err := db.QueryRowContext(ctx, "SELECT project FROM public.epcerp_schema_identity WHERE singleton").Scan(&project); err != nil || project != "epcERP" {
			return ErrUnowned
		}
		return nil
	}
	var count int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM information_schema.tables WHERE table_schema NOT IN ('pg_catalog','information_schema') AND left(table_schema,3) <> 'pg_' AND NOT (table_schema='public' AND table_name='schema_migrations')`).Scan(&count); err != nil {
		return err
	}
	if count != 0 {
		return ErrUnowned
	}
	var versionTable bool
	if err := db.QueryRowContext(ctx, "SELECT to_regclass('public.schema_migrations') IS NOT NULL").Scan(&versionTable); err != nil {
		return err
	}
	if versionTable {
		if err := db.QueryRowContext(ctx, "SELECT count(*) FROM schema_migrations WHERE version > 0 OR dirty").Scan(&count); err != nil {
			return ErrUnowned
		}
		if count != 0 {
			return ErrUnowned
		}
	}
	return nil
}
