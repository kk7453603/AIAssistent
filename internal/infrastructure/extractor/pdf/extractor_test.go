package pdf

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

type storageFake struct {
	data []byte
}

func (f *storageFake) Save(context.Context, string, io.Reader) error { return nil }
func (f *storageFake) Open(_ context.Context, _ string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader(string(f.data))), nil
}

func TestPDFExtractor_EmptyFile(t *testing.T) {
	storage := &storageFake{data: []byte{}}
	ext := NewExtractor(storage)

	doc := &domain.Document{StoragePath: "empty.pdf", Filename: "empty.pdf"}
	_, err := ext.Extract(context.Background(), doc)
	if err == nil {
		t.Error("expected error for empty PDF")
	}
}

func TestPDFExtractor_InvalidData(t *testing.T) {
	storage := &storageFake{data: []byte("not a pdf")}
	ext := NewExtractor(storage)

	doc := &domain.Document{StoragePath: "bad.pdf", Filename: "bad.pdf"}
	_, err := ext.Extract(context.Background(), doc)
	if err == nil {
		t.Error("expected error for invalid PDF data")
	}
}
