package domain

import "testing"

func TestDocumentMetadataDefaults(t *testing.T) {
	meta := DocumentMetadata{}
	if meta.SourceType != "" {
		t.Fatalf("expected empty SourceType, got %q", meta.SourceType)
	}
	if meta.Tags == nil {
		// Tags should be nil by default (zero value for slice)
	}
}
