package domain

import "time"

type DocumentStatus string

const (
	StatusUploaded   DocumentStatus = "uploaded"
	StatusProcessing DocumentStatus = "processing"
	StatusReady      DocumentStatus = "ready"
	StatusFailed     DocumentStatus = "failed"
)

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
