package storage

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"filippo.io/age"
	"github.com/DanilaKorobkov/crypto-portfolio/internal/domain"
)

func TestAgeCheckpointRoundTripAndNoPlaintextFile(t *testing.T) {
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "report.age")
	store, err := NewAgeFile(path, identity.Recipient().String())
	if err != nil {
		t.Fatal(err)
	}
	report := domain.Report{SchemaVersion: 3, Status: "partial", Kind: "synthetic-private-marker"}
	if err := store.Save(report); err != nil {
		t.Fatal(err)
	}
	ciphertext, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(ciphertext, []byte(report.Kind)) || json.Valid(ciphertext) {
		t.Fatal("plaintext leaked")
	}
	reader, err := age.Decrypt(bytes.NewReader(ciphertext), identity)
	if err != nil {
		t.Fatal(err)
	}
	plaintext, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	var decoded domain.Report
	if err := json.Unmarshal(plaintext, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Kind != report.Kind || decoded.Status != report.Status {
		t.Fatal("report changed")
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0600 {
		t.Fatalf("permissions %o", info.Mode().Perm())
	}
	files, _ := os.ReadDir(dir)
	if len(files) != 1 {
		t.Fatal("temporary or plaintext file remained")
	}
}

func TestAgeRejectsWrongKeyAndTampering(t *testing.T) {
	identity, _ := age.GenerateX25519Identity()
	other, _ := age.GenerateX25519Identity()
	path := filepath.Join(t.TempDir(), "report.age")
	store, _ := NewAgeFile(path, identity.Recipient().String())
	if err := store.Save(domain.Report{Status: "partial"}); err != nil {
		t.Fatal(err)
	}
	ciphertext, _ := os.ReadFile(path)
	if _, err := age.Decrypt(bytes.NewReader(ciphertext), other); err == nil {
		t.Fatal("wrong key accepted")
	}
	for _, variant := range []string{"tampered", "truncated"} {
		t.Run(variant, func(t *testing.T) {
			changed := bytes.Clone(ciphertext)
			if variant == "tampered" {
				changed[len(changed)-1] ^= 1
			} else {
				changed = changed[:len(changed)-1]
			}
			r, err := age.Decrypt(bytes.NewReader(changed), identity)
			if err == nil {
				_, err = io.ReadAll(r)
			}
			if err == nil {
				t.Fatal("unauthenticated ciphertext accepted")
			}
		})
	}
}

func TestAgeInvalidRecipientHasNoFallback(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.age")
	_, err := NewAgeFile(path, "secret-invalid-material")
	if err == nil || bytes.Contains([]byte(err.Error()), []byte("secret-invalid-material")) {
		t.Fatal("invalid recipient handling")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("file created")
	}
}

func TestFailedAtomicWritePreservesCheckpoint(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report.age")
	if err := os.WriteFile(path, []byte("previous-ciphertext"), 0600); err != nil {
		t.Fatal(err)
	}
	err := writeAtomic(path, func(w io.Writer) error {
		if _, err := w.Write([]byte("partial-ciphertext")); err != nil {
			return err
		}
		return errors.New("failed encryption")
	})
	if err == nil {
		t.Fatal("failure lost")
	}
	data, _ := os.ReadFile(path)
	if string(data) != "previous-ciphertext" {
		t.Fatal("checkpoint overwritten")
	}
	files, _ := os.ReadDir(dir)
	if len(files) != 1 {
		t.Fatal("temporary file remained")
	}
}
