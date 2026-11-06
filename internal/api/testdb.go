package api

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"vps-control/internal/store"
)

func openTestDB(t *testing.T) *store.DB {
	t.Helper()
	ctx := context.Background()
	if url := os.Getenv("TEST_DATABASE_URL"); url != "" && (len(url) >= 7 && (url[:7] == "sqlite:" || url[:5] == "file:")) {
		db, err := store.Open(ctx, url)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(db.Close)
		return db
	}
	dir := t.TempDir()
	db, err := store.Open(ctx, "sqlite://"+filepath.Join(dir, "nodehub.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	return db
}
