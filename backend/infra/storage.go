package infra

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/khiemnd777/legal_api/core/ingest/extractor"
)

type Storage struct {
	Root string
}

func NewStorage(root string) *Storage {
	return &Storage{Root: root}
}

func (s *Storage) Write(path string, content []byte) error {
	full := filepath.Join(s.Root, path)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	return os.WriteFile(full, content, 0o644)
}

func (s *Storage) Read(path string) (string, error) {
	full := filepath.Join(s.Root, path)
	text, err := extractor.ExtractText(full)
	if err != nil {
		return "", fmt.Errorf("extract text from %s: %w", full, err)
	}
	return text, nil
}

func (s *Storage) Remove(path string) error {
	full := filepath.Join(s.Root, path)
	if err := os.Remove(full); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
