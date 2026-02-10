package domain

type SearchFilter struct {
	Category string
}

type RetrievedChunk struct {
	DocumentID string  `json:"document_id"`
	Filename   string  `json:"filename"`
	Category   string  `json:"category"`
	Text       string  `json:"text"`
	Score      float64 `json:"score"`
}

type Answer struct {
	Text    string           `json:"text"`
	Sources []RetrievedChunk `json:"sources"`
}
