package telegram

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
)

// RegisterWebhooksWithSetka queries all active groups with a valid omsu_group_id,
// registers their webhooks with the Setka schedule microservice, and returns the group IDs.
func RegisterWebhooksWithSetka(ctx context.Context, db *sql.DB, setkaBaseURL, setkaAdminKey, webhookSecret, listenAddr string) ([]int, error) {
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

	body := map[string]interface{}{
		"url":       fmt.Sprintf("http://localhost%s/webhook/schedule", listenAddr),
		"secret":    webhookSecret,
		"group_ids": groupIDs,
		"enabled":   true,
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal webhook registration body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		fmt.Sprintf("%s/api/v1/admin/webhooks", setkaBaseURL),
		bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Admin-Key", setkaAdminKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute webhook registration request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("setka returned status %d", resp.StatusCode)
	}

	slog.Info("successfully registered webhooks with Setka", "groups", groupIDs)
	return groupIDs, nil
}
