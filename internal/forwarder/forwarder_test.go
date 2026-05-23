package forwarder

import (
	"testing"
)

type mockPersona struct {
	name string
}

func (m *mockPersona) Name() string {
	return m.name
}

func TestBuildHeader(t *testing.T) {
	f := &Forwarder{}
	header := f.buildHeader("общий_чат", "testuser", []string{"сессия", "дедлайны"})

	expected := "#сессия #дедлайны"
	if header != expected {
		t.Errorf("header mismatch:\n  got:  %s\n  want: %s", header, expected)
	}
}

func TestBuildHeader_NoHashtags(t *testing.T) {
	f := &Forwarder{}
	header := f.buildHeader("чат", "user", nil)

	expected := ""
	if header != expected {
		t.Errorf("header mismatch:\n  got:  %s\n  want: %s", header, expected)
	}
}

func TestBuildHeader_EmptyUsername(t *testing.T) {
	f := &Forwarder{}
	header := f.buildHeader("чат", "", nil)

	expected := ""
	if header != expected {
		t.Errorf("header mismatch:\n  got:  %s\n  want: %s", header, expected)
	}
}
