package db

import (
	"context"
	"testing"
)

func TestMediaRepo_InsertAndGetByGroupID(t *testing.T) {
	t.Parallel()
	db := setupTopicsTestDB(t)
	ctx := context.Background()

	err := db.InsertMediaItem(ctx, 1, "group1", 12345, "photo")
	if err != nil {
		t.Fatalf("InsertMediaItem failed: %v", err)
	}
	err = db.InsertMediaItem(ctx, 2, "group1", 12345, "video")
	if err != nil {
		t.Fatalf("InsertMediaItem failed: %v", err)
	}
	// Insert a third with different group
	err = db.InsertMediaItem(ctx, 3, "group2", 12345, "document")
	if err != nil {
		t.Fatalf("InsertMediaItem failed: %v", err)
	}

	items, err := db.GetMediaItemsByGroupID(ctx, "group1")
	if err != nil {
		t.Fatalf("GetMediaItemsByGroupID failed: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("expected 2 items, got %d", len(items))
	}
}

func TestMediaRepo_DeleteExpired(t *testing.T) {
	t.Parallel()
	db := setupTopicsTestDB(t)
	ctx := context.Background()

	db.InsertMediaItem(ctx, 1, "old-group", 12345, "photo")
	db.InsertMediaItem(ctx, 2, "new-group", 12345, "photo")

	// Delete expired items (negative minutes means "in the future" — nothing deleted)
	deleted, err := db.DeleteExpiredMediaItems(ctx, -1)
	if err != nil {
		t.Fatalf("DeleteExpiredMediaItems failed: %v", err)
	}
	if deleted != 0 {
		t.Errorf("expected 0 deleted, got %d", deleted)
	}
}
