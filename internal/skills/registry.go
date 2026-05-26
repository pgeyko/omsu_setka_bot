package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"omsu_bot/internal/llm"

	"gopkg.in/yaml.v3"
)

type SkillType string

const (
	SkillTypeTool SkillType = "tool"
)

type SkillParam struct {
	Type        string   `yaml:"type"`
	Description string   `yaml:"description"`
	Required    bool     `yaml:"required"`
	Enum        []string `yaml:"enum,omitempty"`
}

type SkillTool struct {
	Name       string                `yaml:"name"`
	Parameters map[string]SkillParam `yaml:"parameters"`
}

type SkillDefinition struct {
	Name        string    `yaml:"name"`
	DisplayName string    `yaml:"display_name"`
	Type        SkillType `yaml:"type"`
	Category    string    `yaml:"category"`
	AdminOnly   bool      `yaml:"admin_only"`
	Enabled     bool      `yaml:"enabled_by_default"`

	Description  string     `yaml:"description"`
	Tool         SkillTool  `yaml:"tool"`
	Triggers     []string   `yaml:"triggers"`
	Instructions string     `yaml:"instructions"`

	// Handler maps to ToolExecutor method (set from manifest)
	Handler string `yaml:"-"`
}

type SkillManifestEntry struct {
	File    string `yaml:"file"`
	Handler string `yaml:"handler"`
}

type SkillManifest struct {
	Version int                  `yaml:"version"`
	Skills  []SkillManifestEntry `yaml:"skills"`
}

type Registry struct {
	mu     sync.RWMutex
	dir    string
	skills map[string]*SkillDefinition
}

func NewRegistry(dir string) (*Registry, error) {
	r := &Registry{
		dir:    dir,
		skills: make(map[string]*SkillDefinition),
	}
	if err := r.Load(dir); err != nil {
		return nil, err
	}
	return r, nil
}

func Load(dir string) (*Registry, error) {
	return NewRegistry(dir)
}

func (r *Registry) Load(dir string) error {
	manifestPath := filepath.Join(dir, "manifest.yaml")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to read skills manifest: %w", err)
	}

	var manifest SkillManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return fmt.Errorf("failed to parse skills manifest: %w", err)
	}

	skills := make(map[string]*SkillDefinition, len(manifest.Skills))
	for _, entry := range manifest.Skills {
		skillPath := filepath.Join(dir, entry.File)
		skillData, err := os.ReadFile(skillPath)
		if err != nil {
			return fmt.Errorf("failed to read skill %s: %w", entry.File, err)
		}

		var skill SkillDefinition
		if err := yaml.Unmarshal(skillData, &skill); err != nil {
			return fmt.Errorf("failed to parse skill %s: %w", entry.File, err)
		}
		skill.Handler = entry.Handler
		skills[skill.Name] = &skill
	}

	r.mu.Lock()
	r.skills = skills
	r.mu.Unlock()
	return nil
}

func (r *Registry) Get(name string) *SkillDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.skills[name]
}

func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.skills))
	for name := range r.skills {
		names = append(names, name)
	}
	return names
}

func (r *Registry) All() []*SkillDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	skills := make([]*SkillDefinition, 0, len(r.skills))
	for _, s := range r.skills {
		skills = append(skills, s)
	}
	return skills
}

func (r *Registry) GetTools(chatID int64, isAdmin bool, features map[string]bool) []llm.Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var tools []llm.Tool
	for _, s := range r.skills {
		if s.AdminOnly && !isAdmin {
			continue
		}
		if features != nil {
			if enabled, exists := features[s.Name]; exists && !enabled {
				continue
			}
		}
		tools = append(tools, skillToLLMTool(s))
	}
	return tools
}

func skillToLLMTool(s *SkillDefinition) llm.Tool {
	params := make(map[string]interface{})
	params["type"] = "object"

	properties := make(map[string]interface{})
	required := make([]string, 0)
	for name, p := range s.Tool.Parameters {
		prop := map[string]interface{}{
			"type":        p.Type,
			"description": p.Description,
		}
		if len(p.Enum) > 0 {
			prop["enum"] = p.Enum
		}
		properties[name] = prop
		if p.Required {
			required = append(required, name)
		}
	}
	params["properties"] = properties
	if len(required) > 0 {
		params["required"] = required
	}

	return llm.Tool{
		Name:        s.Tool.Name,
		Description: s.Description + "\n\n" + s.Instructions,
		Parameters:  params,
	}
}
