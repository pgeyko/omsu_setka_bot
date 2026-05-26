package db

import (
	"context"
	"testing"
)

func TestGroupCRUD_Create(t *testing.T) {
	t.Parallel()
	d, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create db: %v", err)
	}
	defer d.Close()
	if err := d.Migrate(); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	ctx := context.Background()
	g := &Group{
		ChatID:      -1001234567,
		Title:       "Test Group",
		APIToken:    "test-token",
		OmsuGroupID: 42,
		IsActive:    true,
		IsVIP:       false,
	}
	if err := d.CreateGroup(ctx, g); err != nil {
		t.Fatalf("failed to create group: %v", err)
	}

	retrieved, err := d.GetGroup(ctx, -1001234567)
	if err != nil {
		t.Fatalf("failed to get group: %v", err)
	}
	if retrieved.Title != "Test Group" || retrieved.APIToken != "test-token" || retrieved.OmsuGroupID != 42 {
		t.Errorf("mismatched group fields: %+v", retrieved)
	}

	retrieved2, err := d.GetGroupByToken(ctx, "test-token")
	if err != nil {
		t.Fatalf("failed to get group by token: %v", err)
	}
	if retrieved2.ChatID != -1001234567 {
		t.Errorf("mismatched chat_id on token lookup: %d", retrieved2.ChatID)
	}
}

func TestGroupCRUD_Update(t *testing.T) {
	t.Parallel()
	d, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create db: %v", err)
	}
	defer d.Close()
	if err := d.Migrate(); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	ctx := context.Background()
	g := &Group{
		ChatID:      -1001234567,
		Title:       "Test Group",
		APIToken:    "test-token",
		OmsuGroupID: 42,
		IsActive:    true,
		IsVIP:       false,
	}
	if err := d.CreateGroup(ctx, g); err != nil {
		t.Fatalf("failed to create group: %v", err)
	}

	retrieved, err := d.GetGroup(ctx, -1001234567)
	if err != nil {
		t.Fatalf("failed to get group: %v", err)
	}

	retrieved.Title = "Updated Title"
	retrieved.IsVIP = true
	if err := d.UpdateGroup(ctx, retrieved); err != nil {
		t.Fatalf("failed to update group: %v", err)
	}

	updated, err := d.GetGroup(ctx, -1001234567)
	if err != nil {
		t.Fatalf("failed to get group: %v", err)
	}
	if updated.Title != "Updated Title" || !updated.IsVIP {
		t.Errorf("mismatched updated fields: %+v", updated)
	}
}

func TestGroupCRUD_Delete(t *testing.T) {
	t.Parallel()
	d, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create db: %v", err)
	}
	defer d.Close()
	if err := d.Migrate(); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	ctx := context.Background()
	g := &Group{
		ChatID:      -1001234567,
		Title:       "Test Group",
		APIToken:    "test-token",
		OmsuGroupID: 42,
		IsActive:    true,
		IsVIP:       false,
	}
	if err := d.CreateGroup(ctx, g); err != nil {
		t.Fatalf("failed to create group: %v", err)
	}

	if err := d.DeleteGroup(ctx, -1001234567); err != nil {
		t.Fatalf("failed to delete group: %v", err)
	}

	_, err = d.GetGroup(ctx, -1001234567)
	if err == nil {
		t.Fatal("expected error getting deleted group")
	}
}

func TestGroupCRUD_SoftDelete(t *testing.T) {
	t.Parallel()
	d, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create db: %v", err)
	}
	defer d.Close()
	if err := d.Migrate(); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	ctx := context.Background()
	g := &Group{
		ChatID:      -1001234567,
		Title:       "Test Group",
		APIToken:    "test-token",
		OmsuGroupID: 42,
		IsActive:    true,
		IsVIP:       false,
	}
	if err := d.CreateGroup(ctx, g); err != nil {
		t.Fatalf("failed to create group: %v", err)
	}

	if err := d.SoftDeleteGroup(ctx, -1001234567); err != nil {
		t.Fatalf("failed to soft-delete group: %v", err)
	}

	retrieved, err := d.GetGroup(ctx, -1001234567)
	if err != nil {
		t.Fatalf("failed to get soft-deleted group: %v", err)
	}
	if retrieved.IsActive {
		t.Error("expected is_active to be false after soft delete")
	}
}

func TestGroupCRUD_List(t *testing.T) {
	t.Parallel()
	d, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create db: %v", err)
	}
	defer d.Close()
	if err := d.Migrate(); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	ctx := context.Background()
	g1 := &Group{
		ChatID:      -1001234567,
		Title:       "Group A",
		APIToken:    "token-a",
		OmsuGroupID: 1,
		IsActive:    true,
		IsVIP:       false,
	}
	g2 := &Group{
		ChatID:      -1001234568,
		Title:       "Group B",
		APIToken:    "token-b",
		OmsuGroupID: 2,
		IsActive:    false,
		IsVIP:       true,
	}
	if err := d.CreateGroup(ctx, g1); err != nil {
		t.Fatalf("failed to create group 1: %v", err)
	}
	if err := d.CreateGroup(ctx, g2); err != nil {
		t.Fatalf("failed to create group 2: %v", err)
	}

	list, err := d.ListGroups(ctx)
	if err != nil {
		t.Fatalf("failed to list groups: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(list))
	}

	found := make(map[int64]bool)
	for _, g := range list {
		found[g.ChatID] = true
	}
	if !found[-1001234567] || !found[-1001234568] {
		t.Error("ListGroups did not return both groups")
	}
}
