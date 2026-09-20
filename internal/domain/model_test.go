package domain

import (
	"strings"
	"testing"
	"time"
)

func TestAddressValidation(t *testing.T) {
	for _, input := range []string{"", "0x123", "0x" + strings.Repeat("g", 40)} {
		if _, err := ParseAddress(input); err == nil {
			t.Fatalf("accepted malformed address")
		}
	}
	got, err := ParseAddress("0x" + strings.Repeat("A", 40))
	if err != nil || string(got) != "0x"+strings.Repeat("a", 40) {
		t.Fatal("address normalization")
	}
}
func TestPartialNeverImpliesComplete(t *testing.T) {
	r := Report{Chains: []ChainSnapshot{{Balances: []NativeBalance{{Status: OK, Atomic: "0"}}}}}
	r.Finalize(time.Now())
	if r.Status != "partial" || r.PortfolioComplete {
		t.Fatal("zero balance is data, not full coverage")
	}
	r = Report{}
	r.Finalize(time.Now())
	if r.Status != "failed" {
		t.Fatal("missing data is not success")
	}
}

func intPointer(value int) *int { return &value }

func TestNormalizeDiscoveryPreservesExactQuantityAndWalletOwnership(t *testing.T) {
	discovery := Discovery{Wallets: []WalletDiscovery{
		{Wallet: "wallet-1", Candidates: []Candidate{{ID: " token ", Chain: "ETHEREUM", Protocol: " Aave-V3 ", Kind: "DEBT", Atomic: "000123456789012345678901234567890", Decimals: intPointer(18)}}},
		{Wallet: "wallet-2", Candidates: []Candidate{{ID: "token", Chain: "ethereum", Atomic: "-42", Decimals: intPointer(6)}}},
	}}
	got := NormalizeDiscovery(discovery)
	if len(got.Candidates) != 2 || len(got.Failures) != 0 || got.CoverageComplete {
		t.Fatal(got)
	}
	if got.Candidates[0].Atomic != "123456789012345678901234567890" || got.Candidates[0].Protocol != "aave-v3" || got.Candidates[0].Kind != "debt" {
		t.Fatal(got.Candidates[0])
	}
	if got.Candidates[1].Wallet != "wallet-2" || got.Candidates[1].Atomic != "-42" {
		t.Fatal(got.Candidates[1])
	}
}

func TestNormalizeDiscoveryRejectsMissingValuesWithoutInventingZero(t *testing.T) {
	discovery := Discovery{Wallets: []WalletDiscovery{{Wallet: "wallet", Candidates: []Candidate{
		{ID: "missing-amount", Chain: "ethereum", Atomic: "", Decimals: intPointer(18)},
		{ID: "missing-decimals", Chain: "ethereum", Atomic: "0"},
		{ID: "fractional", Chain: "ethereum", Atomic: "1.2", Decimals: intPointer(18)},
	}}}}
	got := NormalizeDiscovery(discovery)
	if len(got.Candidates) != 0 || len(got.Failures) != 3 {
		t.Fatal(got)
	}
	for _, failure := range got.Failures {
		if failure.Status != InvalidResponse {
			t.Fatal(failure)
		}
	}
}

func TestNormalizeDiscoveryDeduplicatesOnlyIdenticalEvidence(t *testing.T) {
	decimals := intPointer(18)
	base := Candidate{ID: "token", Chain: "ethereum", Atomic: "10", Decimals: decimals}
	conflict := base
	conflict.Atomic = "11"
	discovery := Discovery{Wallets: []WalletDiscovery{{Wallet: "wallet", Candidates: []Candidate{base, base, conflict, conflict, base}}}}
	got := NormalizeDiscovery(discovery)
	if len(got.Candidates) != 2 || len(got.Failures) != 1 {
		t.Fatal(got)
	}
	if got.Candidates[0].Status != InconsistentPagination || got.Candidates[1].Status != InconsistentPagination || got.Failures[0].Status != InconsistentPagination {
		t.Fatal(got)
	}
}
