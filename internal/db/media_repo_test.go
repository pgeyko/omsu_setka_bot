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
	err = db.InsertMediaItem(ctx, 3, "group2", 12345, "document")
	if err != nil {
		t.Fatalf("InsertMediaItem failed: %v", err)
	}

	items, err := db.GetMediaItemsByGroupID(ctx, "group1")
	if err != nil {
		t.Fatalf("GetMediaItemsByGroupID failed: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("expected 2 items for group1, got %d", len(items))
	}
}

func TestMediaRepo_GetByGroupID_NotFound(t *testing.T) {
	t.Parallel()
	db := setupTopicsTestDB(t)
	ctx := context.Background()

	items, err := db.GetMediaItemsByGroupID(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("GetMediaItemsByGroupID failed: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
}

func TestMediaRepo_DuplicateInsert(t *testing.T) {
	t.Parallel()
	db := setupTopicsTestDB(t)
	ctx := context.Background()

	err := db.InsertMediaItem(ctx, 1, "group1", 12345, "photo")
	if err != nil {
		t.Fatalf("first insert failed: %v", err)
	}

	// Duplicate (same message_id + media_group_id) should be ignored
	err = db.InsertMediaItem(ctx, 1, "group1", 12345, "video")
	if err != nil {
		t.Fatalf("duplicate insert should not error: %v", err)
	}

	items, err := db.GetMediaItemsByGroupID(ctx, "group1")
	if err != nil {
		t.Fatalf("GetMediaItemsByGroupID failed: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("expected 1 item after duplicate insert, got %d", len(items))
	}
	if len(items) > 0 && items[0].MediaType != "photo" {
		t.Errorf("expected original media_type 'photo', got %q", items[0].MediaType)
	}
}

func TestMediaRepo_DeleteExpired_RemovesOld(t *testing.T) {
	t.Parallel()
	db := setupTopicsTestDB(t)
	ctx := context.Background()

	// Insert items at different times by using direct SQL to set past timestamps
	_, err := db.ExecContext(ctx,
		`INSERT INTO media_group_items (message_id, media_group_id, chat_id, media_type, created_at)
		 VALUES (1, 'old', 12345, 'photo', datetime('now', '-2 days'))`)
	if err != nil {
		t.Fatalf("insert old item failed: %v", err)
	}
	_, err = db.ExecContext(ctx,
		`INSERT INTO media_group_items (message_id, media_group_id, chat_id, media_type, created_at)
		 VALUES (2, 'new', 12345, 'video', datetime('now', '-1 hour'))`)
	if err != nil {
		t.Fatalf("insert new item failed: %v", err)
	}

	// Delete items older than 1 day
	deleted, err := db.DeleteExpiredMediaItems(ctx, 24*60)
	if err != nil {
		t.Fatalf("DeleteExpiredMediaItems failed: %v", err)
	}
	if deleted != 1 {
		t.Errorf("expected 1 deleted item, got %d", deleted)
	}

	// Verify only the new item remains
	remaining, _ := db.GetMediaItemsByGroupID(ctx, "new")
	if len(remaining) != 1 {
		t.Errorf("expected 1 remaining item, got %d", len(remaining))
	}
}

func TestMediaRepo_DeleteExpired_ZeroTTL(t *testing.T) {
	t.Parallel()
	db := setupTopicsTestDB(t)
	ctx := context.Background()

	db.InsertMediaItem(ctx, 1, "g1", 12345, "photo")

	// Delete with zero TTL — should not delete anything (negative minutes logic)
	deleted, err := db.DeleteExpiredMediaItems(ctx, 0)
	if err != nil {
		t.Fatalf("DeleteExpiredMediaItems failed: %v", err)
	}
	if deleted != 0 {
		t.Errorf("expected 0 deleted with zero TTL, got %d", deleted)
	}
}

func TestMediaRepo_Insert_MultipleMediaTypes(t *testing.T) {
	t.Parallel()
	db := setupTopicsTestDB(t)
	ctx := context.Background()

	types := []string{"photo", "video", "document", "audio", ""}
	for i, mt := range types {
		err := db.InsertMediaItem(ctx, i+1, "multi", 12345, mt)
		if err != nil {
			t.Fatalf("InsertMediaItem type=%q failed: %v", mt, err)
		}
	}

	items, err := db.GetMediaItemsByGroupID(ctx, "multi")
	if err != nil {
		t.Fatalf("GetMediaItemsByGroupID failed: %v", err)
	}
	if len(items) != len(types) {
		t.Errorf("expected %d items, got %d", len(types), len(items))
	}

	// Verify all types stored correctly
	mediaTypes := make(map[string]bool)
	for _, item := range items {
		mediaTypes[item.MediaType] = true
	}
	for _, mt := range types {
		if !mediaTypes[mt] {
			t.Errorf("expected media_type %q in results", mt)
		}
	}
}
