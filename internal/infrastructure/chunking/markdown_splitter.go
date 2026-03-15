package chunking

import (
	"regexp"
	"strings"
)

var markdownHeadingRe = regexp.MustCompile(`^\\s{0,3}#{1,6}\\s+`)

type MarkdownSplitter struct {
	fallback *Splitter
}

func NewMarkdownSplitter(chunkSize, overlap int) *MarkdownSplitter {
	return &MarkdownSplitter{fallback: NewSplitter(chunkSize, overlap)}
}

func (s *MarkdownSplitter) Split(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	sections := splitMarkdownSections(text)
	out := make([]string, 0, len(sections))
	for _, section := range sections {
		out = append(out, s.fallback.Split(section)...)
	}
	return out
}

func splitMarkdownSections(text string) []string {
	lines := strings.Split(text, "\n")
	sections := make([]string, 0, 8)
	current := make([]string, 0, 16)
	hasHeading := false

	flush := func() {
		if len(current) == 0 {
			return
		}
		section := strings.TrimSpace(strings.Join(current, "\n"))
		current = current[:0]
		if section != "" {
			sections = append(sections, section)
		}
	}

	for _, line := range lines {
		if markdownHeadingRe.MatchString(line) {
			if len(current) > 0 {
				flush()
			}
			hasHeading = true
		}
		current = append(current, line)
	}
	flush()

	if !hasHeading {
		return []string{strings.TrimSpace(text)}
	}
	return sections
}
