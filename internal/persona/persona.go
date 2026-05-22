package persona

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
)

type Persona struct {
	Name         string   `json:"name"`
	SystemPrompt string   `json:"system_prompt"`
	Signature    string   `json:"signature"`
	Aliases      []string `json:"aliases"`
}

type Store struct {
	mu      sync.RWMutex
	current Persona
}

func NewStore(db *sql.DB) *Store {
	return &Store{}
}

func (s *Store) Load(ctx context.Context, seedPath string) error {
	if seedPath == "" {
		seedPath = "persona.md"
	}
	seed, err := ParseSeedFile(seedPath)
	if err != nil {
		return fmt.Errorf("failed to parse seed file: %w", err)
	}

	s.mu.Lock()
	s.current = seed
	s.mu.Unlock()

	slog.Info("bot_persona loaded from file", "name", s.current.Name, "path", seedPath)
	return nil
}

func (s *Store) Get() Persona {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.current
}

func (s *Store) SystemPrompt() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.current.SystemPrompt
}

func (s *Store) Name() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.current.Name
}

func (s *Store) Update(ctx context.Context, p Persona) error {
	s.mu.Lock()
	s.current = p
	s.mu.Unlock()

	slog.Info("bot_persona updated in memory", "name", p.Name)
	return nil
}

func (s *Store) SaveToFile(path string) error {
	s.mu.RLock()
	p := s.current
	s.mu.RUnlock()

	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("name: %s\n", p.Name))
	sb.WriteString(fmt.Sprintf("signature: %s\n", p.Signature))
	if len(p.Aliases) > 0 {
		sb.WriteString(fmt.Sprintf("aliases: [%s]\n", strings.Join(p.Aliases, ", ")))
	}
	sb.WriteString("---\n\n")
	sb.WriteString(p.SystemPrompt)

	return os.WriteFile(path, []byte(sb.String()), 0644)
}

func (s *Store) Reset(ctx context.Context, seedPath string) error {
	if seedPath == "" {
		seedPath = "persona.md"
	}
	seed, err := ParseSeedFile(seedPath)
	if err != nil {
		return fmt.Errorf("failed to parse seed file: %w", err)
	}

	s.mu.Lock()
	s.current = seed
	s.mu.Unlock()

	slog.Info("bot_persona reset to seed values from file")
	return nil
}

// GetGroupPersona returns a group-specific persona, falling back to defaultPersona if not found or fields are empty.
func GetGroupPersona(chatID int64, defaultPersona Persona) Persona {
	if chatID == 0 {
		return defaultPersona
	}
	filePath := fmt.Sprintf("data/groups/%d/persona.md", chatID)
	p, err := ParseSeedFile(filePath)
	if err != nil {
		return defaultPersona
	}
	if p.Name == "" {
		p.Name = defaultPersona.Name
	}
	if p.SystemPrompt == "" {
		p.SystemPrompt = defaultPersona.SystemPrompt
	}
	if p.Signature == "" {
		p.Signature = defaultPersona.Signature
	}
	if len(p.Aliases) == 0 {
		p.Aliases = defaultPersona.Aliases
	}
	return p
}

// GetGroupSystemPrompt returns the resolved system prompt and persona for a group.
func GetGroupSystemPrompt(chatID int64, defaultPersona Persona) (string, Persona) {
	p := GetGroupPersona(chatID, defaultPersona)
	if chatID == 0 {
		return p.SystemPrompt, p
	}

	spPath := fmt.Sprintf("data/groups/%d/system_prompt.txt", chatID)
	if spBytes, err := os.ReadFile(spPath); err == nil {
		p.SystemPrompt = string(spBytes)
	}

	systemPrompt := p.SystemPrompt

	kbPath := fmt.Sprintf("data/groups/%d/knowledge_base.txt", chatID)
	if kbBytes, err := os.ReadFile(kbPath); err == nil {
		kbStr := string(kbBytes)
		if strings.TrimSpace(kbStr) != "" {
			systemPrompt += "\n\n=== ГРУППОВАЯ БАЗА ЗНАНИЙ ===\n" + kbStr
		}
	}

	return systemPrompt, p
}
