package persona

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

type Persona struct {
	Name         string `json:"name"`
	SystemPrompt string `json:"system_prompt"`
	Signature    string `json:"signature"`
}

type Store struct {
	mu      sync.RWMutex
	current Persona
	db      *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) Load(ctx context.Context, seedPath string) error {
	var p Persona
	err := s.db.QueryRowContext(ctx,
		`SELECT name, system_prompt, signature FROM bot_persona WHERE id = 1`,
	).Scan(&p.Name, &p.SystemPrompt, &p.Signature)

	if err == sql.ErrNoRows {
		slog.Info("bot_persona table empty, loading from seed file", "path", seedPath)
		seed, err := ParseSeedFile(seedPath)
		if err != nil {
			return fmt.Errorf("failed to parse seed file: %w", err)
		}

		if _, err := s.db.ExecContext(ctx,
			`INSERT INTO bot_persona (id, name, system_prompt, signature, updated_at)
			 VALUES (1, ?, ?, ?, CURRENT_TIMESTAMP)`,
			seed.Name, seed.SystemPrompt, seed.Signature,
		); err != nil {
			return fmt.Errorf("failed to seed bot_persona: %w", err)
		}

		p = seed
		slog.Info("seeded bot_persona from file", "name", p.Name)
	} else if err != nil {
		return fmt.Errorf("failed to load bot_persona: %w", err)
	}

	s.mu.Lock()
	s.current = p
	s.mu.Unlock()

	slog.Info("bot_persona loaded", "name", p.Name)
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
	now := time.Now()
	result, err := s.db.ExecContext(ctx,
		`UPDATE bot_persona SET name = ?, system_prompt = ?, signature = ?, updated_at = ? WHERE id = 1`,
		p.Name, p.SystemPrompt, p.Signature, now,
	)
	if err != nil {
		return fmt.Errorf("failed to update bot_persona: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("bot_persona record not found")
	}

	s.mu.Lock()
	s.current = p
	s.mu.Unlock()

	slog.Info("bot_persona updated", "name", p.Name)
	return nil
}

func (s *Store) Reset(ctx context.Context, seedPath string) error {
	seed, err := ParseSeedFile(seedPath)
	if err != nil {
		return fmt.Errorf("failed to parse seed file: %w", err)
	}

	if err := s.Update(ctx, seed); err != nil {
		return fmt.Errorf("failed to reset bot_persona: %w", err)
	}

	slog.Info("bot_persona reset to seed values")
	return nil
}
