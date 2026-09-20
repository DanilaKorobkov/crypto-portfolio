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
