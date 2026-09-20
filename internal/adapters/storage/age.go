package storage

import (
	"encoding/json"
	"errors"
	"io"

	"filippo.io/age"
	"github.com/DanilaKorobkov/crypto-portfolio/internal/domain"
)

// AgeFile implements ReportRepository without writing plaintext to disk.
// Only the public recipient is needed by the collector; decryption happens elsewhere.
type AgeFile struct {
	path      string
	recipient *age.X25519Recipient
}

func NewAgeFile(path, recipient string) (*AgeFile, error) {
	parsed, err := age.ParseX25519Recipient(recipient)
	if err != nil || path == "" {
		// Do not echo malformed key material from configuration into public logs.
		return nil, errors.New("invalid report encryption configuration")
	}
	return &AgeFile{path: path, recipient: parsed}, nil
}

func (s *AgeFile) Save(report domain.Report) error {
	if s == nil || s.recipient == nil {
		return errors.New("report encryption is not configured")
	}
	return writeAtomic(s.path, func(w io.Writer) error {
		encrypted, err := age.Encrypt(w, s.recipient)
		if err != nil {
			return err
		}
		if err := json.NewEncoder(encrypted).Encode(report); err != nil {
			return err
		}
		// Closing authenticates the final chunk before the checkpoint is committed.
		return encrypted.Close()
	})
}
