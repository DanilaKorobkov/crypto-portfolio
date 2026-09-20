package main

import (
	"encoding/json"
	"github.com/DanilaKorobkov/crypto-portfolio/internal/domain"
	"os"
	"testing"
)

func TestUnconfiguredCLIHasNoNetworkAndSavesFailure(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("WALLETS_JSON", "")
	t.Setenv("RPC_CONFIG_JSON", "")
	t.Setenv("ZERION_API_KEY", "")
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
func TestInvalidConfigurationIsRejected(t *testing.T) {
	t.Setenv("WALLETS_JSON", `["not-an-address"]`)
	t.Setenv("RPC_CONFIG_JSON", "")
	t.Setenv("ZERION_API_KEY", "")
	if err := run(); err == nil {
		t.Fatal("invalid wallet accepted")
	}
}
