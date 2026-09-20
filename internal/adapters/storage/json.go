// Package storage implements the private local report repository.
package storage

import (
	"encoding/json"
	"github.com/DanilaKorobkov/crypto-portfolio/internal/domain"
	"os"
	"path/filepath"
)

type JSONFile struct{ Path string }

func (s JSONFile) Save(report domain.Report) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.Path), 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(s.Path), ".report-*")
	if err != nil {
		return err
	}
	name := file.Name()
	defer os.Remove(name)
	if _, err := file.Write(data); err != nil {
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
	return os.Rename(name, s.Path)
}
