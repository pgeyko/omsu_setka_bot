package handlers

import (
	"context"
	"testing"
	"time"

	"omsu_bot/internal/db"
)

func TestSummaryHandler_RateLimit(t *testing.T) {
	// 1. Setup in-memory DB and Migrate
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create in-memory db: %v", err)
	}
	defer database.Close()
	if err := database.Migrate(); err != nil {
		t.Fatalf("failed to migrate db: %v", err)
	}

	// Insert the group first to satisfy the foreign key constraint:
	// chat_id REFERENCES groups(chat_id)
	_, err = database.DB.Exec(`
		INSERT INTO groups (chat_id, title, api_token, created_at)
		VALUES (123, 'Test Group', 'test_token', CURRENT_TIMESTAMP)
	`)
	if err != nil {
		t.Fatalf("failed to seed group: %v", err)
	}

	// 2. Create SummaryHandler
	h := &SummaryHandler{
		db: database.DB,
	}

	ctx := context.Background()
	userID := int64(999)
	chatID := int64(123)

	// First call should succeed (no requests in DB)
	if !h.checkRateLimit(ctx, userID, chatID) {
		t.Error("expected first rate limit check to pass")
	}

	// Verify that a row was inserted
	var count int
	err = database.DB.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM summary_requests WHERE user_id = ? AND chat_id = ?",
		userID, chatID,
	).Scan(&count)
	if err != nil {
		t.Fatalf("failed to query DB: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 row in summary_requests, got %d", count)
	}

	// Second call should fail (within 30 minutes)
	if h.checkRateLimit(ctx, userID, chatID) {
		t.Error("expected second rate limit check to fail within 30 minutes")
	}

	// Manually update requested_at to 31 minutes ago
	_, err = database.DB.ExecContext(ctx,
		"UPDATE summary_requests SET requested_at = datetime('now', '-31 minutes') WHERE user_id = ? AND chat_id = ?",
		userID, chatID,
	)
	if err != nil {
		t.Fatalf("failed to update requested_at: %v", err)
	}

	// Third call should pass (greater than 30 minutes ago)
	if !h.checkRateLimit(ctx, userID, chatID) {
		t.Error("expected rate limit check to pass after 31 minutes")
	}

	// Verify that there is still only 1 row (due to INSERT OR REPLACE)
	err = database.DB.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM summary_requests WHERE user_id = ? AND chat_id = ?",
		userID, chatID,
	).Scan(&count)
	if err != nil {
		t.Fatalf("failed to query DB: %v", err)
	}
	if count != 1 {
		t.Errorf("expected exactly 1 row in summary_requests after replacement, got %d", count)
	}

	// Check that the requested_at has been updated to approximately now (not 31 minutes ago)
	var requestedAtStr string
	err = database.DB.QueryRowContext(ctx,
		"SELECT requested_at FROM summary_requests WHERE user_id = ? AND chat_id = ?",
		userID, chatID,
	).Scan(&requestedAtStr)
	if err != nil {
		t.Fatalf("failed to get requested_at: %v", err)
	}

	// Parse the timestamp and ensure it is fresh
	// SQLite's CURRENT_TIMESTAMP is UTC
	tFormat := "2006-01-02 15:04:05"
	requestedAt, err := time.Parse(tFormat, requestedAtStr)
	if err != nil {
		requestedAt, err = time.Parse(time.RFC3339, requestedAtStr)
		if err != nil {
			t.Fatalf("failed to parse DB timestamp '%s': %v", requestedAtStr, err)
		}
	}

	if time.Since(requestedAt.UTC()) > 10*time.Second {
		t.Errorf("expected requested_at to be updated to current time, but it is %v", requestedAt)
	}
}
