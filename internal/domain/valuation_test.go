package domain

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func mustDecimal(t *testing.T, value string) Decimal {
	t.Helper()
	decimal, err := ParseDecimal(value)
	if err != nil {
		t.Fatal(err)
	}
	return decimal
}

func TestDecimalCanonicalJSONRoundTrip(t *testing.T) {
	for input, canonical := range map[string]string{"0": "0", "000.1200": "0.12", "-001.2300": "-1.23", "12345678901234567890.0000001": "12345678901234567890.0000001"} {
		decimal := mustDecimal(t, input)
		if decimal.String() != canonical {
			t.Fatalf("%s: %s", input, decimal.String())
		}
		raw, err := json.Marshal(decimal)
		if err != nil {
			t.Fatal(err)
		}
		var restored Decimal
		if err := json.Unmarshal(raw, &restored); err != nil || restored.String() != canonical {
			t.Fatal(string(raw), restored.String(), err)
		}
	}
}

func TestValuePositionIsExact(t *testing.T) {
	price := mustDecimal(t, "1234.56789")
	position := VerifiedPosition{Wallet: "wallet", Chain: "ethereum", InstrumentID: "token", ValuationUnitID: "token-unit", Atomic: "123456789012345678901234567890", Decimals: 18, Snapshot: SnapshotReference{ChainID: 1}, Status: OK}
	got := ValuePosition(position, PriceQuote{ChainID: 1, UnitID: "token-unit", USD: &price, Source: "synthetic", AsOf: time.Unix(1, 0), Status: OK})
	if got.Status != OK || got.Value == nil || got.Value.String() != "152415787517146.7887517146788750190521" {
		t.Fatal(got)
	}
}

func TestMissingPriceRemainsAbsent(t *testing.T) {
	position := VerifiedPosition{Wallet: "wallet", Chain: "ethereum", InstrumentID: "token", Atomic: "0", Decimals: 18, Status: OK}
	got := ValuePosition(position, PriceQuote{Status: MissingPrice})
	if got.Status != MissingPrice || got.Price != nil || got.Value != nil {
		t.Fatal(got)
	}
}

func TestAvailableTotalDoesNotImplyCoverage(t *testing.T) {
	one, two := mustDecimal(t, "1.25"), mustDecimal(t, "-0.25")
	items := []PositionValuation{{Value: &one, Status: OK}, {Status: MissingPrice}, {Value: &two, Status: OK}}
	total := SumAvailableValues(items)
	if total == nil || total.String() != "1" {
		t.Fatal(total)
	}
}

func TestDecimalRejectsAmbiguousValues(t *testing.T) {
	for _, value := range []string{"", ".1", "1.", "+1", " 1", "1e3", "NaN"} {
		if _, err := ParseDecimal(value); err == nil {
			t.Fatalf("accepted %q", value)
		}
	}
}

func TestDecimalRoundTripsMaximumDerivedScale(t *testing.T) {
	input := "0." + strings.Repeat("0", maxDecimalScale-1) + "1"
	decimal := mustDecimal(t, input)
	raw, err := json.Marshal(decimal)
	if err != nil {
		t.Fatal(err)
	}
	var restored Decimal
	if err := json.Unmarshal(raw, &restored); err != nil || restored.String() != input {
		t.Fatal(err, restored.String())
	}
}

func TestValuePositionsRejectsConflictingQuotesAndKeepsNull(t *testing.T) {
	one, two := mustDecimal(t, "1"), mustDecimal(t, "2")
	position := VerifiedPosition{Wallet: "wallet", Chain: "ethereum", InstrumentID: "token", ValuationUnitID: "unit", Atomic: "10", Decimals: 0,
		Snapshot: SnapshotReference{ChainID: 1}, Status: OK}
	got := ValuePositions([]VerifiedPosition{position}, PriceResolution{Status: OK, Quotes: []PriceQuote{
		{ChainID: 1, UnitID: "unit", USD: &one, Source: "one", AsOf: time.Unix(1, 0), Status: OK},
		{ChainID: 1, UnitID: "unit", USD: &two, Source: "two", AsOf: time.Unix(1, 0), Status: OK},
	}})
	if got.CoverageComplete || len(got.Failures) != 1 || len(got.Items) != 1 || got.Items[0].Status != MissingPrice || got.Items[0].Value != nil || got.AvailableTotalUSD != nil {
		t.Fatal(got)
	}
	raw, err := json.Marshal(got.Items[0])
	if err != nil || !strings.Contains(string(raw), `"value_usd":null`) {
		t.Fatal(string(raw), err)
	}
}

func TestValuePositionRejectsZeroAndMismatchedPrice(t *testing.T) {
	zero := mustDecimal(t, "0")
	position := VerifiedPosition{ValuationUnitID: "unit", Atomic: "1", Decimals: 0, Snapshot: SnapshotReference{ChainID: 1}, Status: OK}
	for _, quote := range []PriceQuote{
		{ChainID: 1, UnitID: "unit", USD: &zero, Source: "source", AsOf: time.Unix(1, 0), Status: OK},
		{ChainID: 2, UnitID: "unit", USD: func() *Decimal { value := mustDecimal(t, "1"); return &value }(), Source: "source", AsOf: time.Unix(1, 0), Status: OK},
	} {
		if got := ValuePosition(position, quote); got.Status != InvalidResponse || got.Value != nil {
			t.Fatal(got)
		}
	}
}
