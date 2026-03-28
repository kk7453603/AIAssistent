package spreadsheet

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

func TestCSVExtract(t *testing.T) {
	csv := "Name,Age,City\nAlice,30,Moscow\nBob,25,London\n"
	storage := &storageFake{data: []byte(csv)}
	ext := NewExtractor(storage)

	doc := &domain.Document{
		StoragePath: "data.csv",
		Filename:    "data.csv",
		MimeType:    "text/csv",
	}

	text, err := ext.Extract(context.Background(), doc)
	if err != nil {
		t.Fatalf("Extract() error = %v", err)
	}
	if !strings.Contains(text, "Alice") {
		t.Errorf("expected 'Alice' in text, got %q", text)
	}
	if !strings.Contains(text, "Moscow") {
		t.Errorf("expected 'Moscow' in text, got %q", text)
	}
}

func TestCSVExtract_Empty(t *testing.T) {
	storage := &storageFake{data: []byte("")}
	ext := NewExtractor(storage)

	doc := &domain.Document{StoragePath: "empty.csv", Filename: "empty.csv", MimeType: "text/csv"}
	_, err := ext.Extract(context.Background(), doc)
	if err == nil {
		t.Error("expected error for empty CSV")
	}
}

func TestXLSXExtract_InvalidData(t *testing.T) {
	storage := &storageFake{data: []byte("not xlsx")}
	ext := NewExtractor(storage)

	doc := &domain.Document{
		StoragePath: "bad.xlsx",
		Filename:    "bad.xlsx",
		MimeType:    "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	}
	_, err := ext.Extract(context.Background(), doc)
	if err == nil {
		t.Error("expected error for invalid XLSX data")
	}
}
