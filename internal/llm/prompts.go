package llm

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

var safePromptName = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

type PromptRegistry struct {
	mu         sync.RWMutex
	promptsDir string
	prompts    map[string]string
	extensions map[string]string
}

func NewPromptRegistry(promptsDir string) (*PromptRegistry, error) {
	reg := &PromptRegistry{
		promptsDir: promptsDir,
		prompts:    make(map[string]string),
		extensions: make(map[string]string),
	}
	if err := reg.load(); err != nil {
		return nil, err
	}
	return reg, nil
}

func (r *PromptRegistry) load() error {
	entries, err := os.ReadDir(r.promptsDir)
	if err != nil {
		return fmt.Errorf("failed to read prompts dir: %w", err)
	}

	pm := make(map[string]string)
	exts := make(map[string]string)
	for _, entry := range entries {
		ext := filepath.Ext(entry.Name())
		if entry.IsDir() || (ext != ".txt" && ext != ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(r.promptsDir, entry.Name()))
		if err != nil {
			return fmt.Errorf("failed to read prompt %s: %w", entry.Name(), err)
		}
		name := entry.Name()[:len(entry.Name())-len(ext)]
		pm[name] = string(data)
		exts[name] = ext
	}

	r.mu.Lock()
	r.prompts = pm
	r.extensions = exts
	r.mu.Unlock()
	return nil
}

func (r *PromptRegistry) Reload() error {
	return r.load()
}

func (r *PromptRegistry) Get(name string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.prompts[name]
}

func (r *PromptRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.prompts))
	for name := range r.prompts {
		names = append(names, name)
	}
	return names
}

func (r *PromptRegistry) Dir() string {
	return r.promptsDir
}

func (r *PromptRegistry) Update(name, content string) error {
	if !safePromptName.MatchString(name) {
		return fmt.Errorf("invalid prompt name: %s", name)
	}
	r.mu.RLock()
	ext := r.extensions[name]
	r.mu.RUnlock()
	if ext == "" {
		ext = ".txt"
	}
	path := filepath.Join(r.promptsDir, name+ext)
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}
	absDir, err := filepath.Abs(r.promptsDir)
	if err != nil {
		return fmt.Errorf("failed to resolve prompts dir: %w", err)
	}
	if !strings.HasPrefix(absPath, absDir) {
		return fmt.Errorf("path traversal detected for prompt %s", name)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write prompt %s: %w", name, err)
	}
	return r.load()
}

func (r *PromptRegistry) Delete(name string) error {
	if !safePromptName.MatchString(name) {
		return fmt.Errorf("invalid prompt name: %s", name)
	}
	r.mu.RLock()
	ext := r.extensions[name]
	r.mu.RUnlock()
	if ext == "" {
		ext = ".txt"
	}
	path := filepath.Join(r.promptsDir, name+ext)
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}
	absDir, err := filepath.Abs(r.promptsDir)
	if err != nil {
		return fmt.Errorf("failed to resolve prompts dir: %w", err)
	}
	if !strings.HasPrefix(absPath, absDir) {
		return fmt.Errorf("path traversal detected for prompt %s", name)
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("failed to delete prompt %s: %w", name, err)
	}
	r.mu.Lock()
	delete(r.prompts, name)
	delete(r.extensions, name)
	r.mu.Unlock()
	return nil
}
