package telegram

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

var defaultHTTPClient = &http.Client{Timeout: 30 * time.Second}

const maxRegistrationRetries = 3

// RegisterWebhooksWithSetka queries all active groups with a valid omsu_group_id,
// registers their webhooks with the Setka schedule microservice, and returns the group IDs.
func RegisterWebhooksWithSetka(ctx context.Context, db *sql.DB, setkaBaseURL, setkaAdminKey, webhookSecret, setkaPublicURL string) ([]int, error) {
	if setkaBaseURL == "" || setkaAdminKey == "" {
		return nil, fmt.Errorf("setka config is empty")
	}

	rows, err := db.QueryContext(ctx, "SELECT omsu_group_id FROM groups WHERE is_active = 1 AND omsu_group_id > 0")
	if err != nil {
		return nil, fmt.Errorf("failed to query active groups: %w", err)
	}
	defer rows.Close()

	var groupIDs []int
	for rows.Next() {
		var gid int
		if err := rows.Scan(&gid); err != nil {
			return nil, fmt.Errorf("failed to scan group id: %w", err)
		}
		groupIDs = append(groupIDs, gid)
	}

	if len(groupIDs) == 0 {
		slog.Info("no active groups with omsu_group_id to register with Setka")
		return nil, nil
	}

	publicURL := setkaPublicURL
	if publicURL == "" {
		publicURL = "http://localhost:8081"
	}

	body := map[string]interface{}{
		"url":       publicURL + "/webhook/schedule",
		"secret":    webhookSecret,
		"group_ids": groupIDs,
		"enabled":   true,
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal webhook registration body: %w", err)
	}

	// Retry with exponential backoff: 2s, 4s, 8s
	var lastErr error
	for attempt := 0; attempt < maxRegistrationRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(1<<(attempt+1)) * time.Second):
			}
		}

		req, err := http.NewRequestWithContext(ctx, "PUT",
			fmt.Sprintf("%s/api/v1/admin/webhooks/by-url", setkaBaseURL),
			bytes.NewReader(data))
		if err != nil {
			lastErr = fmt.Errorf("failed to create http request: %w", err)
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Admin-Key", setkaAdminKey)

		resp, err := defaultHTTPClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("failed to execute webhook registration request: %w", err)
			slog.Warn("registerWithSetka attempt failed, retrying...", "attempt", attempt+1, "error", err)
			continue
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			resp.Body.Close()
			lastErr = fmt.Errorf("setka returned status %d", resp.StatusCode)
			slog.Warn("registerWithSetka attempt failed, retrying...", "attempt", attempt+1, "status", resp.StatusCode)
			continue
		}
		resp.Body.Close()

		slog.Info("successfully registered webhooks with Setka", "groups", groupIDs)
		return groupIDs, nil
	}

	slog.Error("failed to register webhooks with Setka after all retries", "error", lastErr)
	return nil, fmt.Errorf("registerWithSetka failed after %d attempts: %w", maxRegistrationRetries, lastErr)
}
