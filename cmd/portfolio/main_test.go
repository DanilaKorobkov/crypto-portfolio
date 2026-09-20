package main

import (
	"bytes"
	"encoding/json"
	"github.com/DanilaKorobkov/crypto-portfolio/internal/domain"
	"io"
	"os"
	"testing"

	"filippo.io/age"
)

func TestUnconfiguredCLIHasNoNetworkAndSavesFailure(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("WALLETS_JSON", "")
	t.Setenv("RPC_CONFIG_JSON", "")
	t.Setenv("ZERION_API_KEY", "")
	t.Setenv("REPORT_RECIPIENT", "")
	if err := run(); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("output/diagnostics.json")
	if err != nil {
		t.Fatal(err)
	}
	var r domain.Report
	if json.Unmarshal(raw, &r) != nil || r.Status != "failed" || r.WalletCount != 0 || r.Discovery.Requests != 0 {
		t.Fatal(r)
	}
}

func TestEncryptedCLIProducesDecryptableCheckpoint(t *testing.T) {
	t.Chdir(t.TempDir())
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GITHUB_ACTIONS", "true")
	t.Setenv("WALLETS_JSON", `["0x0000000000000000000000000000000000000001"]`)
	t.Setenv("RPC_CONFIG_JSON", "[]")
	t.Setenv("ZERION_API_KEY", "")
	t.Setenv("REPORT_RECIPIENT", identity.Recipient().String())
	if err := run(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat("output/diagnostics.json"); !os.IsNotExist(err) {
		t.Fatal("plaintext output created")
	}
	ciphertext, err := os.ReadFile("output/diagnostics.json.age")
	if err != nil {
		t.Fatal(err)
	}
	r, err := age.Decrypt(bytes.NewReader(ciphertext), identity)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	var report domain.Report
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatal(err)
	}
	if report.WalletCount != 1 || report.Status != "failed" || report.PortfolioComplete {
		t.Fatal("diagnostic changed")
	}
}

func TestConfiguredActionsRequiresEncryptionBeforeNetworkOrWrite(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("GITHUB_ACTIONS", "true")
	t.Setenv("WALLETS_JSON", `["0x0000000000000000000000000000000000000001"]`)
	t.Setenv("RPC_CONFIG_JSON", "[]")
	t.Setenv("ZERION_API_KEY", "")
	t.Setenv("REPORT_RECIPIENT", "")
	if err := run(); err == nil {
		t.Fatal("plaintext Actions collection allowed")
	}
	if _, err := os.Stat("output"); !os.IsNotExist(err) {
		t.Fatal("wrote output before validating encryption")
	}
}

func TestMalformedRecipientFailsBeforePersistence(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("WALLETS_JSON", "[]")
	t.Setenv("RPC_CONFIG_JSON", "[]")
	t.Setenv("ZERION_API_KEY", "")
	t.Setenv("REPORT_RECIPIENT", "invalid")
	if err := run(); err == nil {
		t.Fatal("bad recipient accepted")
	}
	if _, err := os.Stat("output"); !os.IsNotExist(err) {
		t.Fatal("plaintext fallback used")
	}
}
func TestInvalidConfigurationIsRejected(t *testing.T) {
	t.Setenv("WALLETS_JSON", `["not-an-address"]`)
	t.Setenv("RPC_CONFIG_JSON", "")
	t.Setenv("ZERION_API_KEY", "")
	if err := run(); err == nil {
		t.Fatal("invalid wallet accepted")
	}
}
