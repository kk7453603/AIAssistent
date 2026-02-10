package localfs

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type Storage struct {
	basePath string
}

func New(basePath string) (*Storage, error) {
	if basePath == "" {
		basePath = "./data/storage"
	}
	if err := os.MkdirAll(basePath, 0o755); err != nil {
		return nil, fmt.Errorf("create storage dir: %w", err)
	}
	return &Storage{basePath: basePath}, nil
}

func (s *Storage) Save(_ context.Context, key string, data io.Reader) error {
	path := filepath.Join(s.basePath, key)
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, data); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	return nil
}

func (s *Storage) Open(_ context.Context, key string) (io.ReadCloser, error) {
	path := filepath.Join(s.basePath, key)
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	return f, nil
}
