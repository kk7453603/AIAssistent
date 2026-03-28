package domain

import "time"

type DocumentStatus string

const (
	StatusUploaded   DocumentStatus = "uploaded"
	StatusProcessing DocumentStatus = "processing"
	StatusReady      DocumentStatus = "ready"
	StatusFailed     DocumentStatus = "failed"
)

type DocumentMetadata struct {
	SourceType string   `json:"source_type"`
	Category   string   `json:"category"`
	Tags       []string `json:"tags"`
	Title      string   `json:"title"`
	Summary    string   `json:"summary"`
	Headers    []string `json:"headers"`
	Path       string   `json:"path"`
}

type Document struct {
	ID          string         `json:"id"`
	Filename    string         `json:"filename"`
	MimeType    string         `json:"mime_type"`
	StoragePath string         `json:"storage_path"`
	Category    string         `json:"category,omitempty"`
	Subcategory string         `json:"subcategory,omitempty"`
	Tags        []string       `json:"tags,omitempty"`
	Confidence  float64        `json:"confidence,omitempty"`
	Summary     string         `json:"summary,omitempty"`
	SourceType  string         `json:"source_type"`
	Title       string         `json:"title"`
	Headers     []string       `json:"headers,omitempty"`
	Path        string         `json:"path"`
	Status      DocumentStatus `json:"status"`
	Error       string         `json:"error,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

type Classification struct {
	Category    string   `json:"category"`
	Subcategory string   `json:"subcategory"`
	Tags        []string `json:"tags"`
	Confidence  float64  `json:"confidence"`
	Summary     string   `json:"summary"`
}
