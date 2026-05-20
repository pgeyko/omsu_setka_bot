package api

import (
	"github.com/gofiber/fiber/v2"
)

type snapshotResponse struct {
	ID        int    `json:"id"`
	CreatedAt string `json:"created_at"`
}

type anomalyResponse struct {
	ID         int    `json:"id"`
	SnapshotID int    `json:"snapshot_id"`
	Type       string `json:"type"`
	Details    string `json:"details"`
	Notified   bool   `json:"notified"`
	CreatedAt  string `json:"created_at"`
}

func (s *Server) handleScheduleSnapshots(c *fiber.Ctx) error {
	limit, offset := parsePagination(c)

	var total int
	s.DB.QueryRowContext(c.Context(), `SELECT COUNT(*) FROM schedule_snapshots`).Scan(&total)

	rows, err := s.DB.QueryContext(c.Context(),
		`SELECT id, created_at FROM schedule_snapshots ORDER BY created_at DESC LIMIT ? OFFSET ?`,
		limit, offset)
	if err != nil {
		return respondError(c, fiber.StatusInternalServerError, ErrInternal, "failed to query snapshots")
	}
	defer rows.Close()

	snapshots := make([]snapshotResponse, 0)
	for rows.Next() {
		var snap snapshotResponse
		if err := rows.Scan(&snap.ID, &snap.CreatedAt); err != nil {
			continue
		}
		snapshots = append(snapshots, snap)
	}

	return respondPaginated(c, snapshots, total, limit, offset)
}

func (s *Server) handleScheduleAnomalies(c *fiber.Ctx) error {
	limit, offset := parsePagination(c)

	var total int
	s.DB.QueryRowContext(c.Context(), `SELECT COUNT(*) FROM schedule_anomalies`).Scan(&total)

	rows, err := s.DB.QueryContext(c.Context(),
		`SELECT id, snapshot_id, type, details, notified, created_at
		 FROM schedule_anomalies ORDER BY created_at DESC LIMIT ? OFFSET ?`,
		limit, offset)
	if err != nil {
		return respondError(c, fiber.StatusInternalServerError, ErrInternal, "failed to query anomalies")
	}
	defer rows.Close()

	anomalies := make([]anomalyResponse, 0)
	for rows.Next() {
		var a anomalyResponse
		var notified int
		if err := rows.Scan(&a.ID, &a.SnapshotID, &a.Type, &a.Details, &notified, &a.CreatedAt); err != nil {
			continue
		}
		a.Notified = notified == 1
		anomalies = append(anomalies, a)
	}

	return respondPaginated(c, anomalies, total, limit, offset)
}
