package metadata

import (
	"context"
	"testing"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

func TestExtractMetadata_MarkdownWithFrontmatter(t *testing.T) {
	ext := New()
	doc := &domain.Document{
		Filename: "notes/programming/go-patterns.md",
		MimeType: "text/markdown",
	}
	text := "---\ncategory: programming\ntags:\n  - go\ntitle: Go Patterns\n---\n\n# Go Patterns\n\nSome content about Go patterns and best practices."

	meta, err := ext.ExtractMetadata(context.Background(), doc, text)
	if err != nil {
		t.Fatalf("ExtractMetadata() error = %v", err)
	}
	if meta.SourceType != "markdown" {
		t.Errorf("SourceType = %q, want %q", meta.SourceType, "markdown")
	}
	if meta.Category != "programming" {
		t.Errorf("Category = %q, want %q", meta.Category, "programming")
	}
	if meta.Title != "Go Patterns" {
		t.Errorf("Title = %q, want %q", meta.Title, "Go Patterns")
	}
	if len(meta.Tags) != 1 || meta.Tags[0] != "go" {
		t.Errorf("Tags = %v, want [go]", meta.Tags)
	}
	if len(meta.Headers) == 0 || meta.Headers[0] != "Go Patterns" {
		t.Errorf("Headers = %v, want [Go Patterns]", meta.Headers)
	}
}

func TestExtractMetadata_PlainText(t *testing.T) {
	ext := New()
	doc := &domain.Document{
		Filename: "report.txt",
		MimeType: "text/plain",
	}
	text := "This is a plain text report about something important.\n\nSecond paragraph."

	meta, err := ext.ExtractMetadata(context.Background(), doc, text)
	if err != nil {
		t.Fatalf("ExtractMetadata() error = %v", err)
	}
	if meta.SourceType != "text" {
		t.Errorf("SourceType = %q, want %q", meta.SourceType, "text")
	}
	if meta.Title != "report" {
		t.Errorf("Title = %q, want %q", meta.Title, "report")
	}
	if meta.Summary == "" {
		t.Error("Summary should not be empty")
	}
}

func TestExtractMetadata_MarkdownNoFrontmatter_UsesH1(t *testing.T) {
	ext := New()
	doc := &domain.Document{
		Filename: "notes/ideas.md",
		MimeType: "text/markdown",
	}
	text := "# My Great Idea\n\nSome content here.\n\n## Section 2\n\nMore content."

	meta, err := ext.ExtractMetadata(context.Background(), doc, text)
	if err != nil {
		t.Fatalf("ExtractMetadata() error = %v", err)
	}
	if meta.Title != "My Great Idea" {
		t.Errorf("Title = %q, want %q", meta.Title, "My Great Idea")
	}
	if meta.Category != "notes" {
		t.Errorf("Category = %q, want %q", meta.Category, "notes")
	}
	if len(meta.Headers) != 2 {
		t.Errorf("Headers count = %d, want 2", len(meta.Headers))
	}
}

func TestExtractMetadata_CategoryFromPath(t *testing.T) {
	ext := New()
	doc := &domain.Document{
		Filename: "vault/science/physics/quantum.md",
		MimeType: "text/markdown",
	}
	text := "# Quantum Mechanics\n\nContent."

	meta, err := ext.ExtractMetadata(context.Background(), doc, text)
	if err != nil {
		t.Fatalf("ExtractMetadata() error = %v", err)
	}
	if meta.Category != "science" {
		t.Errorf("Category = %q, want %q", meta.Category, "science")
	}
	if meta.Path != "vault/science/physics/quantum.md" {
		t.Errorf("Path = %q, want %q", meta.Path, "vault/science/physics/quantum.md")
	}
}
