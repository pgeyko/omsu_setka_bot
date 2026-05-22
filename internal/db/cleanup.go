package db

import (
	"context"
	"log/slog"
	"time"
)

func (d *DB) StartCleanup(ctx context.Context) {
	ticker := time.NewTicker(24 * time.Hour)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				d.runCleanup(ctx)
			case <-ctx.Done():
				return
			}
		}
	}()
	slog.Info("db cleanup scheduler started")
}

func (d *DB) runCleanup(ctx context.Context) {
	queries := []string{
		`DELETE FROM message_buffer WHERE created_at < datetime('now', '-7 days')`,
		`DELETE FROM processed_messages WHERE processed_at < datetime('now', '-30 days')`,
		`DELETE FROM llm_requests WHERE created_at < datetime('now', '-90 days')`,
		`DELETE FROM summary_requests WHERE requested_at < datetime('now', '-1 day')`,
	}
	for _, q := range queries {
		res, err := d.ExecContext(ctx, q)
		if err != nil {
			slog.Error("db cleanup query failed", "error", err, "query", q)
			continue
		}
		if n, _ := res.RowsAffected(); n > 0 {
			slog.Debug("db cleanup deleted rows", "count", n)
		}
	}

	if time.Now().Weekday() == time.Sunday {
		if _, err := d.ExecContext(ctx, "VACUUM"); err != nil {
			slog.Error("db VACUUM failed", "error", err)
		} else {
			slog.Info("db VACUUM completed")
		}
	}
}
