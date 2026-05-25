package util

import (
	"testing"
)

func TestGroupDir(t *testing.T) {
	if got := GroupDir(-1001234567890); got != "data/groups/-1001234567890" {
		t.Errorf("GroupDir() = %q, want %q", got, "data/groups/-1001234567890")
	}
}

func TestGroupFilePath(t *testing.T) {
	if got := GroupFilePath(-100123, "kb.txt"); got != "data/groups/-100123/kb.txt" {
		t.Errorf("GroupFilePath() = %q, want %q", got, "data/groups/-100123/kb.txt")
	}
}
