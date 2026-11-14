package database

import (
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestNodeTypeMigrationDefaultsLegacyRowsToPanel(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "x-ui.db")
	if err := InitDB(dbPath); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	defer CloseDB()

	// Simulate a pre-node-type schema after startup AutoMigrate, then exercise the
	// explicit upgrade migration against an existing nodes table.
	if err := GetDB().Migrator().DropColumn(&model.Node{}, "NodeType"); err != nil {
		t.Fatalf("drop node_type from legacy fixture: %v", err)
	}
	if err := GetDB().Exec("INSERT INTO nodes (name, address, port) VALUES (?, ?, ?)", "legacy", "127.0.0.1", 1).Error; err != nil {
		t.Fatalf("insert legacy node: %v", err)
	}
	if err := migrateNodeTypeColumn(); err != nil {
		t.Fatalf("migrate node_type: %v", err)
	}

	var got model.Node
	if err := GetDB().Where("name = ?", "legacy").First(&got).Error; err != nil {
		t.Fatalf("reload node: %v", err)
	}
	if got.NodeType != model.NodeTypePanel {
		t.Fatalf("NodeType = %q, want %q", got.NodeType, model.NodeTypePanel)
	}
}

func TestNodeTypeMigrationIsIdempotent(t *testing.T) {
	t.Setenv("XUI_DB_FOLDER", t.TempDir())
	if err := InitDB(config.GetDBPath()); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	defer CloseDB()

	if err := migrateNodeTypeColumn(); err != nil {
		t.Fatalf("first migration: %v", err)
	}
	if err := migrateNodeTypeColumn(); err != nil {
		t.Fatalf("second migration: %v", err)
	}
}
