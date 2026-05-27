package db

import (
	"context"
	"testing"
)

func setupTopicsTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatalf("migration failed: %v", err)
	}
	return db
}

func TestTopicsRepo_CreateAndGetBySlug(t *testing.T) {
	t.Parallel()
	db := setupTopicsTestDB(t)
	ctx := context.Background()

	err := db.CreateTopic(ctx, 12345, 1, "Test Topic", "test-topic")
	if err != nil {
		t.Fatalf("CreateTopic failed: %v", err)
	}

	threadID, err := db.GetBySlug(ctx, 12345, "test-topic")
	if err != nil {
		t.Fatalf("GetBySlug failed: %v", err)
	}
	if threadID != 1 {
		t.Errorf("expected threadID 1, got %d", threadID)
	}
}

func TestTopicsRepo_GetBySlug_NotFound(t *testing.T) {
	t.Parallel()
	db := setupTopicsTestDB(t)
	ctx := context.Background()

	_, err := db.GetBySlug(ctx, 12345, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent slug, got nil")
	}
}

func TestTopicsRepo_CountActiveByGroup(t *testing.T) {
	t.Parallel()
	db := setupTopicsTestDB(t)
	ctx := context.Background()

	db.CreateTopic(ctx, 12345, 1, "Topic A", "topic-a")
	db.CreateTopic(ctx, 12345, 2, "Topic B", "topic-b")
	db.SetTopicActive(ctx, 12345, 2, false)

	count, err := db.CountActiveByGroup(ctx, 12345)
	if err != nil {
		t.Fatalf("CountActiveByGroup failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 active topic, got %d", count)
	}
}

func TestTopicsRepo_SearchBySlugPrefix(t *testing.T) {
	t.Parallel()
	db := setupTopicsTestDB(t)
	ctx := context.Background()

	db.CreateTopic(ctx, 12345, 1, "Session", "session")
	db.CreateTopic(ctx, 12345, 2, "Session 2", "session-2")
	db.CreateTopic(ctx, 12345, 3, "Labs", "labs")

	results, err := db.SearchBySlugPrefix(ctx, 12345, "session")
	if err != nil {
		t.Fatalf("SearchBySlugPrefix failed: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}

func TestTopicsRepo_UpdateName(t *testing.T) {
	t.Parallel()
	db := setupTopicsTestDB(t)
	ctx := context.Background()

	db.CreateTopic(ctx, 12345, 1, "Old Name", "old-name")
	err := db.UpdateTopicName(ctx, 12345, 1, "New Name")
	if err != nil {
		t.Fatalf("UpdateTopicName failed: %v", err)
	}

	name, err := db.GetByThreadID(ctx, 12345, 1)
	if err != nil {
		t.Fatalf("GetByThreadID failed: %v", err)
	}
	if name != "New Name" {
		t.Errorf("expected 'New Name', got '%s'", name)
	}
}

func TestTopicsRepo_GetActiveByGroup(t *testing.T) {
	t.Parallel()
	db := setupTopicsTestDB(t)
	ctx := context.Background()

	db.CreateTopic(ctx, 12345, 1, "Active", "active")
	db.CreateTopic(ctx, 12345, 2, "Inactive", "inactive")
	db.SetTopicActive(ctx, 12345, 2, false)

	topics, err := db.GetActiveByGroup(ctx, 12345)
	if err != nil {
		t.Fatalf("GetActiveByGroup failed: %v", err)
	}
	if len(topics) != 1 {
		t.Errorf("expected 1 active topic, got %d", len(topics))
	}
	if topics[0].Name != "active" {
		t.Errorf("expected 'active' slug, got '%s'", topics[0].Name)
	}
}

func TestTopicsRepo_FindByName(t *testing.T) {
	t.Parallel()
	db := setupTopicsTestDB(t)
	ctx := context.Background()

	db.CreateTopic(ctx, 12345, 1, "Exam Schedule", "exam-schedule")
	db.CreateTopic(ctx, 12345, 2, "Exam Schedule", "exam-schedule-2") // duplicate name should not happen normally

	threadID, err := db.FindByName(ctx, 12345, "exam schedule")
	if err != nil {
		t.Fatalf("FindByName failed: %v", err)
	}
	if threadID == 0 {
		t.Error("expected non-zero threadID")
	}
}
