package metadata

import (
	"context"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

var markdownHeaderRe = regexp.MustCompile(`(?m)^#{1,3}\s+(.+)$`)

const maxSummaryLen = 200

// Extractor implements ports.MetadataExtractor using deterministic rules.
type Extractor struct{}

func New() *Extractor {
	return &Extractor{}
}

func (e *Extractor) ExtractMetadata(_ context.Context, doc *domain.Document, text string) (domain.DocumentMetadata, error) {
	meta := domain.DocumentMetadata{
		SourceType: detectSourceType(doc.Filename, doc.MimeType),
		Path:       doc.Filename,
	}

	bodyText := text

	// Parse frontmatter for markdown files.
	if meta.SourceType == "markdown" {
		fm, body := parseFrontmatter(text)
		bodyText = body
		if fm.Category != "" {
			meta.Category = fm.Category
		}
		if len(fm.Tags) > 0 {
			meta.Tags = fm.Tags
		}
		if fm.Title != "" {
			meta.Title = fm.Title
		}
	}

	// Extract headers from markdown.
	if meta.SourceType == "markdown" {
		meta.Headers = extractHeaders(bodyText)
	}

	// Title fallback: first H1 header, then filename without extension.
	if meta.Title == "" && len(meta.Headers) > 0 {
		meta.Title = meta.Headers[0]
	}
	if meta.Title == "" {
		meta.Title = filenameWithoutExt(doc.Filename)
	}

	// Category fallback: first meaningful directory from path.
	if meta.Category == "" {
		meta.Category = categoryFromPath(doc.Filename)
	}

	// Summary: first N characters of body text up to double newline.
	meta.Summary = truncateSummary(bodyText, maxSummaryLen)

	return meta, nil
}

func detectSourceType(filename, mimeType string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".md", ".markdown":
		return "markdown"
	case ".txt":
		return "text"
	}
	if strings.Contains(mimeType, "markdown") {
		return "markdown"
	}
	if strings.HasPrefix(mimeType, "text/") {
		return "text"
	}
	return "unknown"
}

func extractHeaders(text string) []string {
	matches := markdownHeaderRe.FindAllStringSubmatch(text, -1)
	headers := make([]string, 0, len(matches))
	for _, m := range matches {
		if h := strings.TrimSpace(m[1]); h != "" {
			headers = append(headers, h)
		}
	}
	return headers
}

func filenameWithoutExt(filename string) string {
	base := filepath.Base(filename)
	ext := filepath.Ext(base)
	if ext != "" {
		base = base[:len(base)-len(ext)]
	}
	return base
}

// genericRoots are top-level directory names that are skipped only when a more specific
// subdirectory exists deeper in the path.
var genericRoots = map[string]bool{
	"vault": true,
	"docs":  true,
}

func categoryFromPath(filename string) string {
	dir := filepath.Dir(filename)
	if dir == "." || dir == "/" || dir == "" {
		return ""
	}
	parts := strings.Split(filepath.ToSlash(dir), "/")

	// Filter out empty and "." segments.
	var meaningful []string
	for _, p := range parts {
		if p != "" && p != "." {
			meaningful = append(meaningful, strings.ToLower(p))
		}
	}
	if len(meaningful) == 0 {
		return ""
	}

	// Skip generic root names (vault, docs) only when there is a more specific segment after them.
	for i, p := range meaningful {
		if genericRoots[p] && i < len(meaningful)-1 {
			continue
		}
		return p
	}
	return meaningful[len(meaningful)-1]
}

func truncateSummary(text string, maxLen int) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	if idx := strings.Index(text, "\n\n"); idx > 0 && idx < maxLen {
		text = text[:idx]
	}
	runes := []rune(text)
	if len(runes) > maxLen {
		runes = runes[:maxLen]
	}
	return strings.TrimSpace(string(runes))
}
