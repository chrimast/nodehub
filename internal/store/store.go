package store

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"vps-control/internal/authconfig"

	_ "modernc.org/sqlite"
)

//go:embed schema.sqlite.sql
var sqliteSchema string

func Open(ctx context.Context, raw string) (*DB, error) {
	path, err := sqlitePath(raw)
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(path, "file:") {
		dir := filepath.Dir(path)
		if dir != "." && dir != "" {
			if err := os.MkdirAll(dir, 0o700); err != nil {
				return nil, err
			}
		}
		path = "file:" + path + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(15000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)"
	}
	sqlDB, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(1)
	if err := sqlDB.PingContext(ctx); err != nil {
		sqlDB.Close()
		return nil, err
	}
	if _, err := sqlDB.ExecContext(ctx, "PRAGMA foreign_keys=ON"); err != nil {
		sqlDB.Close()
		return nil, err
	}
	if _, err := sqlDB.ExecContext(ctx, sqliteSchema); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("sqlite schema: %w", err)
	}
	initial, err := authconfig.HashPassword("admin")
	if err != nil { sqlDB.Close(); return nil, err }
	if _, err := sqlDB.ExecContext(ctx, `INSERT OR IGNORE INTO admin_credentials(tenant_id,password_hash,must_change) VALUES('self',$1,1)`, initial); err != nil {
		sqlDB.Close()
		return nil, err
	}
	if _, err := sqlDB.ExecContext(ctx, `INSERT OR IGNORE INTO schema_migrations(version) VALUES('sqlite-nodehub')`); err != nil {
		sqlDB.Close()
		return nil, err
	}
	return &DB{sql: sqlDB}, nil
}
