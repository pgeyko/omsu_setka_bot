package handlers

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type CommandRegistry struct {
	TopicCommands   []string          `json:"topic_commands"`
	SummaryCmds     []string          `json:"summary_commands"`
	IDQueries       []string          `json:"id_queries"`
	ScheduleQueries []string          `json:"schedule_queries"`
	PersonaAliases  []string          `json:"persona_aliases"`
	Responses       map[string]string `json:"responses"`
}

func LoadCommands(path string) (*CommandRegistry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read commands file: %w", err)
	}
	var reg CommandRegistry
	if err := json.Unmarshal(data, &reg); err != nil {
		return nil, fmt.Errorf("failed to parse commands file: %w", err)
	}
	if reg.Responses == nil {
		reg.Responses = make(map[string]string)
	}
	return &reg, nil
}

func (r *CommandRegistry) IsPersonaMention(text, personaName string) bool {
	lower := strings.ToLower(text)
	if personaName != "" && strings.Contains(lower, strings.ToLower(personaName)) {
		return true
	}
	for _, alias := range r.PersonaAliases {
		if strings.Contains(lower, alias) {
			return true
		}
	}
	return false
}

func (r *CommandRegistry) IsTopicCommand(text string) bool {
	lower := strings.ToLower(text)
	for _, kw := range r.TopicCommands {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

func (r *CommandRegistry) IsSummaryCommand(text string) bool {
	lower := strings.ToLower(text)
	for _, kw := range r.SummaryCmds {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

func (r *CommandRegistry) IsTopicIDQuery(text string) bool {
	lower := strings.ToLower(text)
	for _, kw := range r.IDQueries {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

func (r *CommandRegistry) IsScheduleQuery(text string) bool {
	lower := strings.ToLower(text)
	for _, kw := range r.ScheduleQueries {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

func (r *CommandRegistry) Response(key string, params map[string]string) string {
	tpl, ok := r.Responses[key]
	if !ok {
		return key
	}
	for k, v := range params {
		tpl = strings.ReplaceAll(tpl, "{"+k+"}", v)
	}
	return tpl
}
