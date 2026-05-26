package db

import (
	"testing"
)

func TestNewInMemory(t *testing.T) {
	t.Parallel()
	d, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create in-memory db: %v", err)
	}
	defer d.Close()

	if d.DB == nil {
		t.Fatal("expected non-nil DB")
	}
}

func TestMigration(t *testing.T) {
	t.Parallel()
	d, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create db: %v", err)
	}
	defer d.Close()

	if err := d.Migrate(); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	// Verify all tables exist
	tables := []string{
		"groups",
		"superadmins",
		"topics",
		"processed_messages",
		"llm_requests",
		"schedule_snapshots",
		"schedule_anomalies",
		"command_permissions",
		"summary_requests",
		"message_buffer",
	}

	for _, table := range tables {
		var count int
		err := d.DB.QueryRow(
			"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?",
			table,
		).Scan(&count)
		if err != nil || count == 0 {
			t.Errorf("table %s not found after migration", table)
		}
	}
}

func TestIdempotentMigration(t *testing.T) {
	t.Parallel()
	d, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create db: %v", err)
	}
	defer d.Close()

	if err := d.Migrate(); err != nil {
		t.Fatalf("first migration failed: %v", err)
	}

	if err := d.Migrate(); err != nil {
		t.Fatalf("second migration failed (not idempotent): %v", err)
	}
}
