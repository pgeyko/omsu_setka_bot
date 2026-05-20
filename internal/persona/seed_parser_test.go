package persona

import (
	"testing"
)

func TestParseSeed(t *testing.T) {
	content := `# name
Борис

# system_prompt
Ты — Борис, помощник

# signature
Б.`

	p := ParseSeed(content)

	if p.Name != "Борис" {
		t.Errorf("expected name 'Борис', got '%s'", p.Name)
	}
	if p.SystemPrompt != "Ты — Борис, помощник" {
		t.Errorf("expected system_prompt 'Ты — Борис, помощник', got '%s'", p.SystemPrompt)
	}
	if p.Signature != "Б." {
		t.Errorf("expected signature 'Б.', got '%s'", p.Signature)
	}
}

func TestParseSeed_Empty(t *testing.T) {
	p := ParseSeed("")
	if p.Name != "" {
		t.Errorf("expected empty name, got '%s'", p.Name)
	}
}

func TestParseSeed_OnlyName(t *testing.T) {
	content := `# name
Тест`
	p := ParseSeed(content)
	if p.Name != "Тест" {
		t.Errorf("expected name 'Тест', got '%s'", p.Name)
	}
	if p.SystemPrompt != "" {
		t.Errorf("expected empty system_prompt, got '%s'", p.SystemPrompt)
	}
}
