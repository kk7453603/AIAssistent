package usecase

import "regexp"

var (
	wikilinkRe = regexp.MustCompile(`\[\[([^\]]+)\]\]`)
	mdLinkRe   = regexp.MustCompile(`\[([^\]]*)\]\(([^)]+\.md)\)`)
)

// extractWikilinks returns all [[Page Name]] targets from text.
func extractWikilinks(text string) []string {
	matches := wikilinkRe.FindAllStringSubmatch(text, -1)
	result := make([]string, 0, len(matches))
	for _, m := range matches {
		if m[1] != "" {
			result = append(result, m[1])
		}
	}
	return result
}

// extractMarkdownLinks returns all [text](path.md) targets from text.
func extractMarkdownLinks(text string) []string {
	matches := mdLinkRe.FindAllStringSubmatch(text, -1)
	result := make([]string, 0, len(matches))
	for _, m := range matches {
		if m[2] != "" {
			result = append(result, m[2])
		}
	}
	return result
}
