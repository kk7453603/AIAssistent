package domain

import "io"

// SourceRequest carries input from any ingest initiator.
type SourceRequest struct {
	SourceType string            // "upload", "obsidian", "web"
	Filename   string            // original filename or path
	MimeType   string            // MIME type if known
	Body       io.Reader         // content (for upload/file sources)
	URL        string            // URL (for web scraping)
	VaultID    string            // vault ID (for Obsidian)
	Path       string            // path in vault/fs
	Meta       map[string]string // arbitrary source metadata
}

// IngestResult is the normalized output from a SourceAdapter.
type IngestResult struct {
	Filename   string
	MimeType   string
	Body       io.Reader
	SourceType string
	Path       string
	ExtraMeta  map[string]string
}
