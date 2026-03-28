package metadata

import (
	"strings"

	"gopkg.in/yaml.v3"
)

type frontmatterData struct {
	Category string   `yaml:"category"`
	Tags     []string `yaml:"tags"`
	Title    string   `yaml:"title"`
}

// parseFrontmatter extracts YAML frontmatter (between --- delimiters) and returns
// parsed data + remaining body text. If no frontmatter found, returns zero data and original text.
func parseFrontmatter(text string) (frontmatterData, string) {
	if !strings.HasPrefix(text, "---\n") {
		return frontmatterData{}, text
	}

	rest := text[4:]

	// Handle closing delimiter with or without preceding newline.
	// Prefer "\n---" (normal case), but also accept "---" at start of rest (empty frontmatter).
	var yamlBlock, body string
	if strings.HasPrefix(rest, "---") {
		// Empty frontmatter: "---\n---\n..."
		yamlBlock = ""
		body = strings.TrimSpace(rest[3:])
		if strings.HasPrefix(body, "\n") {
			body = strings.TrimSpace(body[1:])
		}
	} else {
		end := strings.Index(rest, "\n---")
		if end < 0 {
			return frontmatterData{}, text
		}
		yamlBlock = rest[:end]
		body = strings.TrimSpace(rest[end+4:])
	}

	var fm frontmatterData
	if err := yaml.Unmarshal([]byte(yamlBlock), &fm); err != nil {
		return frontmatterData{}, text
	}

	return fm, body
}
