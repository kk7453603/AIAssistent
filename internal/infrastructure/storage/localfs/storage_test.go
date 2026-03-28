package localfs

import (
	"context"
	"io"
	"path/filepath"
	"strings"
	"testing"
)

func TestNew_CreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "dir")
	s, err := New(dir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if s == nil {
		t.Fatal("expected non-nil storage")
	}
}

func TestNew_DefaultPath(t *testing.T) {
	// With empty basePath, uses default ./data/storage
	// This may create dirs in the current directory, so we skip in short mode
	if testing.Short() {
		t.Skip("skipping default path test in short mode")
	}
}

func TestSaveAndOpen_RoundTrip(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	content := "hello world"
	ctx := context.Background()

	if err := s.Save(ctx, "test.txt", strings.NewReader(content)); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	reader, err := s.Open(ctx, "test.txt")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if string(data) != content {
		t.Fatalf("got %q, want %q", string(data), content)
	}
}

func TestOpen_NotFound(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	_, err = s.Open(context.Background(), "nonexistent.txt")
	if err == nil {
		t.Fatal("expected error for non-existent file")
	}
}

func TestSave_Overwrite(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	ctx := context.Background()

	_ = s.Save(ctx, "file.txt", strings.NewReader("first"))
	_ = s.Save(ctx, "file.txt", strings.NewReader("second"))

	reader, err := s.Open(ctx, "file.txt")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer reader.Close()

	data, _ := io.ReadAll(reader)
	if string(data) != "second" {
		t.Fatalf("got %q, want %q", string(data), "second")
	}
}

func TestSave_EmptyContent(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	ctx := context.Background()

	if err := s.Save(ctx, "empty.txt", strings.NewReader("")); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	reader, err := s.Open(ctx, "empty.txt")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer reader.Close()

	data, _ := io.ReadAll(reader)
	if len(data) != 0 {
		t.Fatalf("expected empty content, got %d bytes", len(data))
	}
}
