package sqlite

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"strings"
	"time"

	"sanitation-operations/internal/identity"
	"sanitation-operations/internal/repository"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaFS embed.FS

type executor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type queryStore struct {
	e   executor
	ids identity.Generator
}

type DB struct {
	queryStore
	db *sql.DB
}

type txStore struct {
	queryStore
	tx *sql.Tx
}

func Open(ctx context.Context, dsn string) (*DB, error) {
	if !strings.Contains(dsn, "_") && dsn == "" {
		return nil, fmt.Errorf("database URL is required")
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	store := &DB{db: db}
	store.queryStore = queryStore{e: db, ids: identity.Random{}}
	if err := store.configure(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (d *DB) configure(ctx context.Context) error {
	for _, statement := range []string{"PRAGMA foreign_keys = ON", "PRAGMA busy_timeout = 5000"} {
		if _, err := d.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("sqlite %s: %w", statement, err)
		}
	}
	return d.Migrate(ctx)
}

func (d *DB) Migrate(ctx context.Context) error {
	schema, err := schemaFS.ReadFile("schema.sql")
	if err != nil {
		return fmt.Errorf("read schema: %w", err)
	}
	if _, err := d.db.ExecContext(ctx, string(schema)); err != nil {
		return fmt.Errorf("apply schema: %w", err)
	}
	var version int
	if err := d.db.QueryRowContext(ctx, "SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&version); err != nil {
		return fmt.Errorf("read migration version: %w", err)
	}
	if version < 1 {
		if _, err := d.db.ExecContext(ctx, "INSERT INTO schema_migrations(version, applied_at) VALUES(1, ?)", time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
			return fmt.Errorf("record migration: %w", err)
		}
		version = 1
	}
	if version < 2 {
		tx, err := d.db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin migration 2: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `UPDATE vehicles SET plate_number = CASE plate_number WHEN '沪环-001' THEN '沪A00001' WHEN '沪环-002' THEN '沪A00002' END WHERE plate_number IN ('沪环-001', '沪环-002')`); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("normalize demo plates: %w", err)
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO schema_migrations(version, applied_at) VALUES(2, ?)", time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration 2: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration 2: %w", err)
		}
	}
	return nil
}

func (d *DB) Ping(ctx context.Context) error { return d.db.PingContext(ctx) }
func (d *DB) Close() error                   { return d.db.Close() }

func (d *DB) WithTx(ctx context.Context, fn func(context.Context, repository.Tx) error) error {
	tx, err := d.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return err
	}
	store := &txStore{tx: tx}
	store.queryStore = queryStore{e: tx, ids: d.ids}
	if err := fn(ctx, store); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}
