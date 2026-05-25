package messages

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultMessages(t *testing.T) {
	if Default == nil {
		t.Fatal("Default is nil")
	}
	if Default.RegisterOK == "" {
		t.Error("Default.RegisterOK is empty")
	}
	if Default.HelpHeader == "" {
		t.Error("Default.HelpHeader is empty")
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	m := Load("/nonexistent/path/messages.yaml")
	if m == nil {
		t.Fatal("Load returned nil")
	}
	if m.RegisterOK == "" {
		t.Error("fallback Default has empty RegisterOK")
	}
}

func TestLoad_ValidFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "messages.yaml")
	content := []byte(`register_ok: "✅ Зарегистрировано"
help_header: "📖 Помощь"`)
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	m := Load(path)
	if m.RegisterOK != "✅ Зарегистрировано" {
		t.Errorf("RegisterOK = %q, want %q", m.RegisterOK, "✅ Зарегистрировано")
	}
	if m.HelpHeader != "📖 Помощь" {
		t.Errorf("HelpHeader = %q, want %q", m.HelpHeader, "📖 Помощь")
	}
}

func TestLoad_PartialOverlay(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "messages.yaml")
	content := []byte(`register_ok: "✅ Зарегистрировано"`)
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	m := Load(path)
	if m.RegisterOK != "✅ Зарегистрировано" {
		t.Errorf("RegisterOK should be overridden")
	}
	if m.HelpHeader == "" {
		t.Error("HelpHeader should fall back to Default")
	}
}

func TestFormat(t *testing.T) {
	m := &Messages{
		RegisterOK: "Hello {name}!",
	}

	tests := []struct {
		template string
		vars     map[string]string
		want     string
	}{
		{"Hello {name}!", map[string]string{"name": "Alice"}, "Hello Alice!"},
		{"{greeting}, {name}!", map[string]string{"greeting": "Hi", "name": "Bob"}, "Hi, Bob!"},
		{"no placeholders", map[string]string{}, "no placeholders"},
		{"{missing}", map[string]string{}, "{missing}"},
	}
	for _, tt := range tests {
		t.Run(tt.template, func(t *testing.T) {
			if got := m.Format(tt.template, tt.vars); got != tt.want {
				t.Errorf("Format(%q, %v) = %q, want %q", tt.template, tt.vars, got, tt.want)
			}
		})
	}
}
