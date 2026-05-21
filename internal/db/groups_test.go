package db

import (
	"context"
	"testing"
)

func TestGroupsCRUD(t *testing.T) {
	d, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create db: %v", err)
	}
	defer d.Close()

	if err := d.Migrate(); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	ctx := context.Background()

	// Test Create
	g := &Group{
		ChatID:           -1001234567,
		Title:            "Test Group",
		APIToken:         "test-token",
		OmsuGroupID:      42,
		AnnounceThreadID: 100,
		IsActive:         true,
		IsVIP:            false,
	}

	if err := d.CreateGroup(ctx, g); err != nil {
		t.Fatalf("failed to create group: %v", err)
	}

	// Test Get
	retrieved, err := d.GetGroup(ctx, -1001234567)
	if err != nil {
		t.Fatalf("failed to get group: %v", err)
	}
	if retrieved.Title != "Test Group" || retrieved.APIToken != "test-token" || retrieved.OmsuGroupID != 42 || retrieved.AnnounceThreadID != 100 {
		t.Errorf("mismatched group fields: %+v", retrieved)
	}

	// Test GetByToken
	retrieved2, err := d.GetGroupByToken(ctx, "test-token")
	if err != nil {
		t.Fatalf("failed to get group by token: %v", err)
	}
	if retrieved2.ChatID != -1001234567 {
		t.Errorf("mismatched chat_id on token lookup: %d", retrieved2.ChatID)
	}

	// Test Update
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

	// Test List
	list, err := d.ListGroups(ctx)
	if err != nil {
		t.Fatalf("failed to list groups: %v", err)
	}
	if len(list) != 1 || list[0].ChatID != -1001234567 {
		t.Errorf("expected list of length 1, got %d", len(list))
	}

	// Test Superadmin
	isSA, err := d.IsSuperadmin(ctx, 999)
	if err != nil {
		t.Fatalf("failed to check superadmin: %v", err)
	}
	if isSA {
		t.Fatal("expected false for unregistered superadmin")
	}

	if err := d.AddSuperadmin(ctx, 999, "my note"); err != nil {
		t.Fatalf("failed to add superadmin: %v", err)
	}

	isSA2, err := d.IsSuperadmin(ctx, 999)
	if err != nil {
		t.Fatalf("failed to check superadmin: %v", err)
	}
	if !isSA2 {
		t.Fatal("expected true for registered superadmin")
	}

	// Test ListSuperadmins
	saList, err := d.ListSuperadmins(ctx)
	if err != nil {
		t.Fatalf("failed to list superadmins: %v", err)
	}
	if len(saList) != 1 || saList[0].UserID != 999 || saList[0].Note != "my note" {
		t.Errorf("expected 1 superadmin, got: %+v", saList)
	}

	if err := d.RemoveSuperadmin(ctx, 999); err != nil {
		t.Fatalf("failed to remove superadmin: %v", err)
	}

	isSA3, err := d.IsSuperadmin(ctx, 999)
	if err != nil {
		t.Fatalf("failed to check superadmin: %v", err)
	}
	if isSA3 {
		t.Fatal("expected false after removal")
	}

	// Test UpdateGroupOmsuID
	if err := d.UpdateGroupOmsuID(ctx, -1001234567, 9999); err != nil {
		t.Fatalf("failed to update group omsu id: %v", err)
	}
	updatedOmsu, err := d.GetGroup(ctx, -1001234567)
	if err != nil {
		t.Fatalf("failed to get group after update: %v", err)
	}
	if updatedOmsu.OmsuGroupID != 9999 {
		t.Errorf("expected OmsuGroupID to be 9999, got %d", updatedOmsu.OmsuGroupID)
	}

	// Test Delete
	if err := d.DeleteGroup(ctx, -1001234567); err != nil {
		t.Fatalf("failed to delete group: %v", err)
	}

	_, err = d.GetGroup(ctx, -1001234567)
	if err == nil {
		t.Fatal("expected error getting deleted group")
	}
}
