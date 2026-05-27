package db

import (
	"context"
	"testing"
)

func TestProcessedRepo_InsertAndUpdate(t *testing.T) {
	t.Parallel()
	db := setupTopicsTestDB(t)
	ctx := context.Background()

	inserted, err := db.InsertProcessedMessage(ctx, 100, 12345, "classified")
	if err != nil {
		t.Fatalf("InsertProcessedMessage failed: %v", err)
	}
	if !inserted {
		t.Error("expected first insert to return true")
	}

	err = db.UpdateProcessedMessage(ctx, 100, 12345, 1, "forwarded", 2)
	if err != nil {
		t.Fatalf("UpdateProcessedMessage failed: %v", err)
	}
}

func TestProcessedRepo_InsertDuplicate(t *testing.T) {
	t.Parallel()
	db := setupTopicsTestDB(t)
	ctx := context.Background()

	first, err := db.InsertProcessedMessage(ctx, 100, 12345, "classified")
	if err != nil {
		t.Fatalf("first insert failed: %v", err)
	}
	if !first {
		t.Error("expected first insert to return true")
	}

	// Second insert with same keys should be ignored (INSERT OR IGNORE)
	second, err := db.InsertProcessedMessage(ctx, 100, 12345, "classified")
	if err != nil {
		t.Fatalf("duplicate insert should not error: %v", err)
	}
	if second {
		t.Error("expected duplicate insert to return false")
	}
}
