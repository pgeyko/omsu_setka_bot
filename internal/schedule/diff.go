package schedule

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"omsu_bot/internal/llm"
)

type Change struct {
	Date    string `json:"date"`
	Pair    int    `json:"pair"`
	Field   string `json:"field"`
	Old     string `json:"old"`
	New     string `json:"new"`
	Subject string `json:"subject"`
}

type WebhookPayload struct {
	Type    string   `json:"type"`
	GroupID int      `json:"group_id"`
	Changes []Change `json:"changes"`
}

type AnomalyType string

const (
	AnomalyBuilding AnomalyType = "ANOMALY_BUILDING"
	AnomalyRoom     AnomalyType = "ANOMALY_ROOM"
	AnomalySubject  AnomalyType = "ANOMALY_SUBJECT"
	AnomalyCancel   AnomalyType = "ANOMALY_CANCEL"
)

type Anomaly struct {
	Type    AnomalyType `json:"type"`
	Field   string      `json:"field"`
	Date    string      `json:"date"`
	Pair    int         `json:"pair"`
	Old     string      `json:"old"`
	New     string      `json:"new"`
	Subject string      `json:"subject"`
}

type Snapshot struct {
	ID        int       `json:"id"`
	Data      string    `json:"data"`
	CreatedAt time.Time `json:"created_at"`
}

type AnomalyRecord struct {
	ID         int         `json:"id"`
	SnapshotID int         `json:"snapshot_id"`
	Type       AnomalyType `json:"type"`
	Details    string      `json:"details"`
	Notified   bool        `json:"notified"`
	CreatedAt  time.Time   `json:"created_at"`
}

type DiffEngine struct {
	db     *sql.DB
	bot    TelegramPoster
	client LLMClient
}

type TelegramPoster interface {
	PostToThread(ctx context.Context, threadID int, text string) error
}

type LLMClient interface {
	Call(ctx context.Context, reqType, systemExtra, userPrompt string, requiresVision bool) (*llm.Response, error)
}

func NewDiffEngine(db *sql.DB, bot TelegramPoster, client LLMClient) *DiffEngine {
	return &DiffEngine{db: db, bot: bot, client: client}
}

func (e *DiffEngine) ProcessWebhook(ctx context.Context, payload *WebhookPayload, announceThreadID int) error {
	anomalies := e.detectAnomalies(payload.Changes)
	slog.Debug("anomalies detected", "count", len(anomalies))

	if len(anomalies) == 0 {
		slog.Info("no anomalies detected in webhook payload")
		return nil
	}

	snapshotData, _ := json.Marshal(payload)
	snapshotID, err := e.saveSnapshot(ctx, string(snapshotData))
	if err != nil {
		return fmt.Errorf("failed to save snapshot: %w", err)
	}

	for _, a := range anomalies {
		details, _ := json.Marshal(a)
		if err := e.saveAnomaly(ctx, snapshotID, a.Type, string(details)); err != nil {
			slog.Error("failed to save anomaly", "error", err, "type", a.Type)
		}
	}

	msg := e.buildAnnouncement(anomalies)
	if err := e.bot.PostToThread(ctx, announceThreadID, msg); err != nil {
		slog.Error("failed to post announcement", "error", err)
	}

	return nil
}

func (e *DiffEngine) detectAnomalies(changes []Change) []Anomaly {
	var anomalies []Anomaly

	for _, c := range changes {
		anomaly := e.classifyChange(c)
		if anomaly != nil {
			anomalies = append(anomalies, *anomaly)
		}
	}

	return anomalies
}

func (e *DiffEngine) classifyChange(c Change) *Anomaly {
	switch c.Field {
	case "building":
		return &Anomaly{
			Type:    AnomalyBuilding,
			Field:   c.Field,
			Date:    c.Date,
			Pair:    c.Pair,
			Old:     c.Old,
			New:     c.New,
			Subject: c.Subject,
		}
	case "room", "auditorium":
		return &Anomaly{
			Type:    AnomalyRoom,
			Field:   c.Field,
			Date:    c.Date,
			Pair:    c.Pair,
			Old:     c.Old,
			New:     c.New,
			Subject: c.Subject,
		}
	case "subject":
		return &Anomaly{
			Type:    AnomalySubject,
			Field:   c.Field,
			Date:    c.Date,
			Pair:    c.Pair,
			Old:     c.Old,
			New:     c.New,
			Subject: c.New,
		}
	case "full":
		if c.Old != "" && c.New == "" {
			return &Anomaly{
				Type:    AnomalyCancel,
				Field:   c.Field,
				Date:    c.Date,
				Pair:    c.Pair,
				Old:     c.Old,
				New:     "",
				Subject: c.Subject,
			}
		}
		return nil
	default:
		return nil
	}
}

func (e *DiffEngine) buildAnnouncement(anomalies []Anomaly) string {
	if len(anomalies) == 0 {
		return ""
	}

	text := "📅 Изменения в расписании\n\n"
	for _, a := range anomalies {
		dateStr := e.formatDate(a.Date)
		switch a.Type {
		case AnomalyBuilding:
			text += fmt.Sprintf("⚠️ %s, %d-я пара — %s. Переезд: корпус %s → корпус %s\n", dateStr, a.Pair, a.Subject, a.Old, a.New)
		case AnomalyRoom:
			text += fmt.Sprintf("⚠️ %s, %d-я пара — %s. Аудитория: %s → %s\n", dateStr, a.Pair, a.Subject, a.Old, a.New)
		case AnomalySubject:
			text += fmt.Sprintf("🔄 %s, %d-я пара — замена: %s → %s\n", dateStr, a.Pair, a.Old, a.New)
		case AnomalyCancel:
			text += fmt.Sprintf("❌ %s, %d-я пара — %s ОТМЕНЕНА\n", dateStr, a.Pair, a.Subject)
		}
	}
	text += "\nНе перепутайте 👆"

	return text
}

func (e *DiffEngine) formatDate(dateStr string) string {
	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return dateStr
	}

	weekdays := map[string]string{
		"Monday": "Понедельник", "Tuesday": "Вторник", "Wednesday": "Среда",
		"Thursday": "Четверг", "Friday": "Пятница", "Saturday": "Суббота", "Sunday": "Воскресенье",
	}

	dayName := weekdays[t.Weekday().String()]
	months := map[time.Month]string{
		time.January: "января", time.February: "февраля", time.March: "марта",
		time.April: "апреля", time.May: "мая", time.June: "июня",
		time.July: "июля", time.August: "августа", time.September: "сентября",
		time.October: "октября", time.November: "ноября", time.December: "декабря",
	}

	return fmt.Sprintf("%s %d %s", dayName, t.Day(), months[t.Month()])
}

func (e *DiffEngine) saveSnapshot(ctx context.Context, data string) (int, error) {
	result, err := e.db.ExecContext(ctx,
		`INSERT INTO schedule_snapshots (data, created_at) VALUES (?, CURRENT_TIMESTAMP)`, data)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	return int(id), err
}

func (e *DiffEngine) saveAnomaly(ctx context.Context, snapshotID int, anomalyType AnomalyType, details string) error {
	_, err := e.db.ExecContext(ctx,
		`INSERT INTO schedule_anomalies (snapshot_id, type, details, notified, created_at)
		 VALUES (?, ?, ?, 0, CURRENT_TIMESTAMP)`,
		snapshotID, string(anomalyType), details)
	return err
}
