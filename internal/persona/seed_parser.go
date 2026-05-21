package persona

import (
	"fmt"
	"os"
	"strings"
)

func ParseSeedFile(path string) (Persona, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Persona{}, fmt.Errorf("failed to read seed file: %w", err)
	}

	return ParseSeed(string(data)), nil
}

func ParseSeed(content string) Persona {
	p := Persona{}
	sections := strings.Split(content, "\n# ")
	for _, section := range sections {
		section = strings.TrimSpace(section)
		if section == "" {
			continue
		}

		var key, value string
		if idx := strings.Index(section, "\n"); idx != -1 {
			key = strings.TrimSpace(section[:idx])
			value = strings.TrimSpace(section[idx+1:])
		} else {
			key = section
			value = ""
		}

		key = strings.TrimLeft(key, "#")
		key = strings.TrimSpace(key)

		switch key {
		case "name":
			p.Name = value
		case "system_prompt":
			p.SystemPrompt = value
		case "signature":
			p.Signature = value
		case "aliases":
			normalized := strings.ReplaceAll(value, "\n", ",")
			parts := strings.Split(normalized, ",")
			var aliases []string
			for _, part := range parts {
				part = strings.TrimSpace(part)
				if part != "" {
					aliases = append(aliases, part)
				}
			}
			p.Aliases = aliases
		}
	}
	return p
}
