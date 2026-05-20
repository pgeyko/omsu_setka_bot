package schedule

import (
	"testing"
)

func TestDetectAnomalies_Empty(t *testing.T) {
	e := &DiffEngine{}
	anomalies := e.detectAnomalies(nil)

	if len(anomalies) != 0 {
		t.Errorf("expected no anomalies, got %d", len(anomalies))
	}
}

func TestClassifyChange_Building(t *testing.T) {
	e := &DiffEngine{}
	c := Change{
		Date:    "2025-03-18",
		Pair:    3,
		Field:   "building",
		Old:     "4",
		New:     "1",
		Subject: "История",
	}

	a := e.classifyChange(c)
	if a == nil {
		t.Fatal("expected anomaly, got nil")
	}
	if a.Type != AnomalyBuilding {
		t.Errorf("expected ANOMALY_BUILDING, got %s", a.Type)
	}
	if a.Subject != "История" {
		t.Errorf("expected subject 'История', got '%s'", a.Subject)
	}
}

func TestClassifyChange_Room(t *testing.T) {
	e := &DiffEngine{}
	c := Change{
		Date:    "2025-03-18",
		Pair:    2,
		Field:   "room",
		Old:     "101",
		New:     "205",
		Subject: "Математика",
	}

	a := e.classifyChange(c)
	if a == nil {
		t.Fatal("expected anomaly, got nil")
	}
	if a.Type != AnomalyRoom {
		t.Errorf("expected ANOMALY_ROOM, got %s", a.Type)
	}
}

func TestClassifyChange_Subject(t *testing.T) {
	e := &DiffEngine{}
	c := Change{
		Date:    "2025-03-18",
		Pair:    1,
		Field:   "subject",
		Old:     "БЖД",
		New:     "История",
		Subject: "История",
	}

	a := e.classifyChange(c)
	if a == nil {
		t.Fatal("expected anomaly, got nil")
	}
	if a.Type != AnomalySubject {
		t.Errorf("expected ANOMALY_SUBJECT, got %s", a.Type)
	}
}

func TestClassifyChange_Cancel(t *testing.T) {
	e := &DiffEngine{}
	c := Change{
		Date:    "2025-03-18",
		Pair:    4,
		Field:   "full",
		Old:     `{"lesson":"Физика"}`,
		New:     "",
		Subject: "Физика",
	}

	a := e.classifyChange(c)
	if a == nil {
		t.Fatal("expected anomaly, got nil")
	}
	if a.Type != AnomalyCancel {
		t.Errorf("expected ANOMALY_CANCEL, got %s", a.Type)
	}
}

func TestBuildAnnouncement(t *testing.T) {
	e := &DiffEngine{}
	anomalies := []Anomaly{
		{Type: AnomalyBuilding, Date: "2025-03-18", Pair: 3, Old: "4", New: "1", Subject: "История"},
		{Type: AnomalyCancel, Date: "2025-03-19", Pair: 1, Subject: "Физика"},
	}

	msg := e.buildAnnouncement(anomalies)
	if msg == "" {
		t.Fatal("expected non-empty announcement")
	}
}

func TestFormatDate(t *testing.T) {
	e := &DiffEngine{}
	result := e.formatDate("2025-03-18")

	if result == "" {
		t.Fatal("expected formatted date")
	}
}

func TestBuildAnnouncement_Empty(t *testing.T) {
	e := &DiffEngine{}
	msg := e.buildAnnouncement(nil)
	if msg != "" {
		t.Errorf("expected empty message for nil anomalies, got '%s'", msg)
	}
}
