package spreadsheet

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

type storageFake struct {
	data    []byte
	openErr error
}

func (f *storageFake) Save(context.Context, string, io.Reader) error { return nil }
func (f *storageFake) Open(_ context.Context, _ string) (io.ReadCloser, error) {
	if f.openErr != nil {
		return nil, f.openErr
	}
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

// makeXLSX creates an in-memory XLSX file using excelize and returns the bytes.
// sheetData maps sheet name -> rows (each row is a slice of cell values).
func makeXLSX(t *testing.T, sheetData map[string][][]string) []byte {
	t.Helper()
	f := excelize.NewFile()
	first := true
	for name, rows := range sheetData {
		if first {
			// Rename the default "Sheet1"
			f.SetSheetName("Sheet1", name)
			first = false
		} else {
			_, err := f.NewSheet(name)
			if err != nil {
				t.Fatalf("create sheet %q: %v", name, err)
			}
		}
		for r, row := range rows {
			for c, val := range row {
				cell, _ := excelize.CoordinatesToCellName(c+1, r+1)
				_ = f.SetCellValue(name, cell, val)
			}
		}
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		t.Fatalf("write xlsx to buffer: %v", err)
	}
	return buf.Bytes()
}

func TestExtractSpreadsheet_EmptySheet(t *testing.T) {
	// XLSX with one sheet that has no data at all.
	data := makeXLSX(t, map[string][][]string{
		"Empty": {},
	})
	storage := &storageFake{data: data}
	ext := NewExtractor(storage)

	doc := &domain.Document{
		StoragePath: "empty_sheet.xlsx",
		Filename:    "empty_sheet.xlsx",
		MimeType:    "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	}
	_, err := ext.Extract(context.Background(), doc)
	if err == nil {
		t.Fatal("expected error for XLSX with empty sheet")
	}
	if !strings.Contains(err.Error(), "no data") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestExtractSpreadsheet_MultipleSheets(t *testing.T) {
	data := makeXLSX(t, map[string][][]string{
		"Users": {
			{"Name", "Age"},
			{"Alice", "30"},
		},
		"Orders": {
			{"Item", "Qty"},
			{"Widget", "5"},
		},
	})
	storage := &storageFake{data: data}
	ext := NewExtractor(storage)

	doc := &domain.Document{
		StoragePath: "multi.xlsx",
		Filename:    "multi.xlsx",
		MimeType:    "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	}
	text, err := ext.Extract(context.Background(), doc)
	if err != nil {
		t.Fatalf("Extract() error = %v", err)
	}
	// Both sheet headers should appear.
	if !strings.Contains(text, "Sheet: Users") {
		t.Errorf("expected 'Sheet: Users' in output, got %q", text)
	}
	if !strings.Contains(text, "Sheet: Orders") {
		t.Errorf("expected 'Sheet: Orders' in output, got %q", text)
	}
	// Data from both sheets should appear.
	if !strings.Contains(text, "Alice") {
		t.Errorf("expected 'Alice' in output, got %q", text)
	}
	if !strings.Contains(text, "Widget") {
		t.Errorf("expected 'Widget' in output, got %q", text)
	}
}

func TestExtractCSV_EmptyContent(t *testing.T) {
	// CSV with only whitespace / newlines -- no actual records.
	storage := &storageFake{data: []byte("\n\n")}
	ext := NewExtractor(storage)

	doc := &domain.Document{
		StoragePath: "blank.csv",
		Filename:    "blank.csv",
		MimeType:    "text/csv",
	}
	// Either returns error or returns only whitespace; both are acceptable.
	text, err := ext.Extract(context.Background(), doc)
	if err != nil {
		return // error is fine for empty content
	}
	trimmed := strings.TrimSpace(text)
	if trimmed != "" {
		t.Errorf("expected empty or whitespace-only result for blank CSV, got %q", text)
	}
}

func TestExtractSpreadsheet_StorageError(t *testing.T) {
	storageErr := errors.New("permission denied")
	storage := &storageFake{openErr: storageErr}
	ext := NewExtractor(storage)

	doc := &domain.Document{
		StoragePath: "any.xlsx",
		Filename:    "any.xlsx",
		MimeType:    "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	}
	_, err := ext.Extract(context.Background(), doc)
	if err == nil {
		t.Fatal("expected error when storage.Open fails")
	}
	if !errors.Is(err, storageErr) {
		t.Errorf("expected wrapped storage error, got: %v", err)
	}
}
