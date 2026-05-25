package permissions

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	_ "modernc.org/sqlite"
)

func setupDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS command_permissions (
		group_id     INTEGER NOT NULL,
		command      TEXT NOT NULL,
		allowed_role TEXT NOT NULL DEFAULT 'everyone',
		PRIMARY KEY (group_id, command)
	)`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}
	return db
}

func TestService_AllowedRole(t *testing.T) {
	db := setupDB(t)
	s := NewService(db)
	ctx := context.Background()

	_, err := db.ExecContext(ctx,
		`INSERT INTO command_permissions (group_id, command, allowed_role) VALUES (?, ?, ?)`,
		-1001, "ban", "admin")
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	_, err = db.ExecContext(ctx,
		`INSERT INTO command_permissions (group_id, command, allowed_role) VALUES (?, ?, ?)`,
		-1001, "ping", "everyone")
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	tests := []struct {
		groupID int64
		command string
		want    string
	}{
		{-1001, "ban", "admin"},
		{-1001, "ping", "everyone"},
		{-1001, "unknown", ""},
		{-9999, "ban", ""},
	}
	for _, tt := range tests {
		name := fmt.Sprintf("%d/%s", tt.groupID, tt.command)
		t.Run(name, func(t *testing.T) {
			if got := s.AllowedRole(ctx, tt.groupID, tt.command); got != tt.want {
				t.Errorf("AllowedRole(%d, %q) = %q, want %q", tt.groupID, tt.command, got, tt.want)
			}
		})
	}
}

func TestService_IsAdminOnly(t *testing.T) {
	db := setupDB(t)
	s := NewService(db)
	ctx := context.Background()

	_, err := db.ExecContext(ctx,
		`INSERT INTO command_permissions (group_id, command, allowed_role) VALUES (?, ?, ?)`,
		-1001, "ban", "admin")
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	if !s.IsAdminOnly(ctx, -1001, "ban") {
		t.Error("expected IsAdminOnly = true for ban")
	}
	if s.IsAdminOnly(ctx, -1001, "ping") {
		t.Error("expected IsAdminOnly = false for unconfigured command")
	}
}
