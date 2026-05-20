package llm

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewPromptRegistry_NoDir(t *testing.T) {
	_, err := NewPromptRegistry("/nonexistent/path")
	if err == nil {
		t.Error("expected error for nonexistent directory")
	}
}

func TestNewPromptRegistry_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()

	reg, err := NewPromptRegistry(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if reg.Get("anything") != "" {
		t.Error("expected empty string for missing prompt")
	}
}

func TestNewPromptRegistry_WithFiles(t *testing.T) {
	tmpDir := t.TempDir()

	os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("hello world"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "other.txt"), []byte("foo bar"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "not_a_prompt.md"), []byte("markdown"), 0644)

	reg, err := NewPromptRegistry(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if reg.Get("test") != "hello world" {
		t.Errorf("expected 'hello world', got '%s'", reg.Get("test"))
	}
	if reg.Get("other") != "foo bar" {
		t.Errorf("expected 'foo bar', got '%s'", reg.Get("other"))
	}
	if reg.Get("not_a_prompt") != "" {
		t.Error("expected empty for non-txt file")
	}
}
