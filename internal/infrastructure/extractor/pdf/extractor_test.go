package pdf

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

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

// minimalPDFNoText is a valid PDF with one empty page (no text content).
var minimalPDFNoText = []byte(
	"%PDF-1.0\n" +
		"1 0 obj<</Type/Catalog/Pages 2 0 R>>endobj\n" +
		"2 0 obj<</Type/Pages/Kids[3 0 R]/Count 1>>endobj\n" +
		"3 0 obj<</Type/Page/MediaBox[0 0 612 792]/Parent 2 0 R>>endobj\n" +
		"xref\n0 4\n" +
		"0000000000 65535 f \n" +
		"0000000009 00000 n \n" +
		"0000000058 00000 n \n" +
		"0000000115 00000 n \n" +
		"trailer<</Size 4/Root 1 0 R>>\nstartxref\n190\n%%EOF",
)

// validPDFWithText is a hand-crafted minimal valid PDF containing the text "Hello World".
// It has proper cross-reference table and a text stream on page 1.
var validPDFWithText = buildValidPDF()

func buildValidPDF() []byte {
	// Minimal PDF 1.4 with one page containing "Hello World" text.
	var b strings.Builder
	offsets := make([]int, 6) // objects 1-5, index 0 unused

	b.WriteString("%PDF-1.4\n")

	// Object 1: Catalog
	offsets[1] = b.Len()
	b.WriteString("1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n")

	// Object 2: Pages
	offsets[2] = b.Len()
	b.WriteString("2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n")

	// Object 3: Page
	offsets[3] = b.Len()
	b.WriteString("3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 4 0 R /Resources << /Font << /F1 5 0 R >> >> >>\nendobj\n")

	// Object 4: Content stream
	stream := "BT /F1 12 Tf 100 700 Td (Hello World) Tj ET"
	offsets[4] = b.Len()
	fmt.Fprintf(&b, "4 0 obj\n<< /Length %d >>\nstream\n%s\nendstream\nendobj\n", len(stream), stream)

	// Object 5: Font
	offsets[5] = b.Len()
	b.WriteString("5 0 obj\n<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>\nendobj\n")

	// Cross-reference table
	xrefOffset := b.Len()
	b.WriteString("xref\n")
	fmt.Fprintf(&b, "0 %d\n", 6)
	b.WriteString("0000000000 65535 f \n")
	for i := 1; i <= 5; i++ {
		fmt.Fprintf(&b, "%010d 00000 n \n", offsets[i])
	}

	// Trailer
	fmt.Fprintf(&b, "trailer\n<< /Size 6 /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", xrefOffset)

	return []byte(b.String())
}

func TestExtractPDF_ValidWithText(t *testing.T) {
	storage := &storageFake{data: validPDFWithText}
	ext := NewExtractor(storage)

	doc := &domain.Document{StoragePath: "hello.pdf", Filename: "hello.pdf"}
	text, err := ext.Extract(context.Background(), doc)
	if err != nil {
		t.Fatalf("Extract() error = %v", err)
	}
	if !strings.Contains(text, "Hello World") {
		t.Errorf("expected 'Hello World' in extracted text, got %q", text)
	}
}

func TestExtractPDF_EmptyFile(t *testing.T) {
	storage := &storageFake{data: minimalPDFNoText}
	ext := NewExtractor(storage)

	doc := &domain.Document{StoragePath: "notext.pdf", Filename: "notext.pdf"}
	_, err := ext.Extract(context.Background(), doc)
	if err == nil {
		t.Fatal("expected error for PDF with no text content")
	}
	if !strings.Contains(err.Error(), "no text content") && !strings.Contains(err.Error(), "parse pdf") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestExtractPDF_StorageError(t *testing.T) {
	storageErr := errors.New("disk failure")
	storage := &storageFake{openErr: storageErr}
	ext := NewExtractor(storage)

	doc := &domain.Document{StoragePath: "any.pdf", Filename: "any.pdf"}
	_, err := ext.Extract(context.Background(), doc)
	if err == nil {
		t.Fatal("expected error when storage.Open fails")
	}
	if !errors.Is(err, storageErr) {
		t.Errorf("expected wrapped storage error, got: %v", err)
	}
}
