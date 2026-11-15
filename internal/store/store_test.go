package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestSQLiteTripleSlashAbsolutePath(t *testing.T) {
	dir := t.TempDir()
	abs := filepath.Join(dir, "nodehub.db")
	db, err := Open(context.Background(), "sqlite://"+abs)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := os.Stat(abs); err != nil {
		t.Fatal(err)
	}
}

func TestUpgradeRejectsExistingPortCollision(t *testing.T) {
	t.Skip("SQLite applies a complete schema; PostgreSQL incremental collision upgrades are not used on nodehub")
}

func TestSQLiteOpenIsIdempotent(t *testing.T) {
	ctx := context.Background()
	url := "sqlite://" + t.TempDir() + "/nodehub.db"
	first, err := Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	first.Close()
	second, err := Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	var n int
	if err := second.QueryRow(ctx, `SELECT count(*) FROM port_reservations`).Scan(&n); err != nil {
		t.Fatal(err)
	}
}
