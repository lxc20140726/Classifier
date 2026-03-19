package db

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"
)

func TestOpenRunsMigrations(t *testing.T) {
	t.Parallel()

	dsn := filepath.Join(t.TempDir(), "classifier.db")
	database, err := Open(dsn)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer database.Close()

	tables := []string{"folders", "snapshots", "audit_logs", "config"}
	for _, table := range tables {
		t.Run(table, func(t *testing.T) {
			if !sqliteObjectExists(t, database, "table", table) {
				t.Fatalf("expected table %q to exist", table)
			}
		})
	}

	indexes := []string{
		"idx_folders_status",
		"idx_folders_category",
		"idx_snapshots_job",
		"idx_snapshots_folder",
		"idx_audit_folder",
		"idx_audit_action",
		"idx_audit_created",
	}

	for _, index := range indexes {
		t.Run(index, func(t *testing.T) {
			if !sqliteObjectExists(t, database, "index", index) {
				t.Fatalf("expected index %q to exist", index)
			}
		})
	}
}

func TestOpenEnablesPragmas(t *testing.T) {
	t.Parallel()

	dsn := filepath.Join(t.TempDir(), "classifier_pragmas.db")
	database, err := Open(dsn)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer database.Close()

	var foreignKeys int
	if err := database.QueryRow("PRAGMA foreign_keys;").Scan(&foreignKeys); err != nil {
		t.Fatalf("query PRAGMA foreign_keys error = %v", err)
	}

	if foreignKeys != 1 {
		t.Fatalf("foreign_keys = %d, want 1", foreignKeys)
	}

	var journalMode string
	if err := database.QueryRow("PRAGMA journal_mode;").Scan(&journalMode); err != nil {
		t.Fatalf("query PRAGMA journal_mode error = %v", err)
	}

	if journalMode != "wal" {
		t.Fatalf("journal_mode = %q, want %q", journalMode, "wal")
	}
}

func sqliteObjectExists(t *testing.T, db *sql.DB, objectType, name string) bool {
	t.Helper()

	var count int
	err := db.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type = ? AND name = ?",
		objectType,
		name,
	).Scan(&count)
	if err != nil {
		t.Fatalf("query sqlite_master for %s %s failed: %v", objectType, name, err)
	}

	return count > 0
}

func TestOpenInvalidPath(t *testing.T) {
	t.Parallel()

	_, err := Open("/path/that/does/not/exist/classifier.db")
	if err == nil {
		t.Fatalf("expected Open() to fail for invalid path")
	}

	if got := err.Error(); got == "" {
		t.Fatalf("expected error text, got empty string")
	}

	_ = fmt.Sprintf("%v", err)
}
