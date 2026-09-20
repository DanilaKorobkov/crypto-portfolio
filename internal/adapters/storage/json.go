// Package storage implements the private local report repository.
package storage

import (
	"encoding/json"
	"github.com/DanilaKorobkov/crypto-portfolio/internal/domain"
	"io"
	"os"
	"path/filepath"
)

type JSONFile struct{ Path string }

func (s JSONFile) Save(report domain.Report) error {
	return writeAtomic(s.Path, func(w io.Writer) error {
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(report)
	})
}

// Commit only a fully written file. On failure the previous checkpoint survives.
func writeAtomic(path string, write func(io.Writer) error) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".report-*")
	if err != nil {
		return err
	}
	name := file.Name()
	defer os.Remove(name)
	if err := write(file); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}
