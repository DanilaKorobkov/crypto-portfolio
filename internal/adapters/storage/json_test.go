package storage

import (
	"encoding/json"
	"github.com/DanilaKorobkov/crypto-portfolio/internal/domain"
	"os"
	"path/filepath"
	"testing"
)

func TestReportReplacementIsPrivateAndReadable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "private", "report.json")
	s := JSONFile{Path: path}
	for _, status := range []string{"running", "partial"} {
		if err := s.Save(domain.Report{Status: status}); err != nil {
			t.Fatal(err)
		}
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var r domain.Report
	if json.Unmarshal(raw, &r) != nil || r.Status != "partial" {
		t.Fatal("invalid saved report")
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm()&0077 != 0 {
		t.Fatal("report readable by other local users")
	}
}
