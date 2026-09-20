package domain

import (
	"math/big"
	"testing"
	"time"
)

func riskFixture(t *testing.T, debtAtomic string) (RiskUnitInput, []VerifiedPosition) {
	t.Helper()
	snapshot := SnapshotReference{Chain: "ethereum", ChainID: 1, BlockNumber: "0x10", BlockHash: "0xabc", BlockTime: time.Unix(10, 0).UTC(), Finality: UnknownFinality}
	priceTwo, priceOne, threshold := mustDecimal(t, "2"), mustDecimal(t, "1"), mustDecimal(t, "0.5")
	collateral := VerifiedPosition{Wallet: "wallet", Chain: "ethereum", Protocol: "lending", InstrumentID: "collateral", ValuationUnitID: "eth", ExposureID: "collateral",
		AccountingRole: EconomicExposure, Class: AssetPosition, Atomic: "100", Decimals: 0, Source: "synthetic", Snapshot: snapshot, Status: OK}
	debt := VerifiedPosition{Wallet: "wallet", Chain: "ethereum", Protocol: "lending", InstrumentID: "debt", ValuationUnitID: "usd", ExposureID: "debt",
		AccountingRole: EconomicExposure, Class: DebtPosition, Atomic: debtAtomic, Decimals: 0, Source: "synthetic", Snapshot: snapshot, Status: OK}
	oracle := func(unit string, price *Decimal) OracleQuote {
		return OracleQuote{UnitID: unit, USD: price, Source: "protocol-oracle", AsOf: snapshot.BlockTime, Snapshot: snapshot, Status: OK}
	}
	input := RiskUnitInput{RiskUnitID: "market/wallet", Wallet: "wallet", Protocol: "lending", MarketID: "market", Model: LinearWeightedCollateral,
		Snapshot: snapshot, Source: "synthetic-adapter",
		Collateral: []CollateralRiskInput{{Amount: RiskAmount{InstrumentID: "collateral", UnitID: "eth", Atomic: "100", Decimals: 0, OraclePrice: oracle("eth", &priceTwo)}, LiquidationThreshold: &threshold}},
		Debts:      []RiskAmount{{InstrumentID: "debt", UnitID: "usd", Atomic: debtAtomic, Decimals: 0, OraclePrice: oracle("usd", &priceOne), PriceRelation: IndependentPrice}},
	}
	return input, []VerifiedPosition{collateral, debt}
}

func assertRatio(t *testing.T, value *Rational, numerator, denominator string) {
	t.Helper()
	if value == nil || value.Numerator != numerator || value.Denominator != denominator {
		t.Fatalf("got %#v; want %s/%s", value, numerator, denominator)
	}
}

func TestAssessDebtRiskHFAndLiquidationScenarioExact(t *testing.T) {
	input, positions := riskFixture(t, "50")
	got := AssessDebtRisk([]RiskUnitInput{input}, positions)
	if got.Status != OK || len(got.Units) != 1 {
		t.Fatal(got)
	}
	unit := got.Units[0]
	assertRatio(t, unit.DerivedHF, "2", "1")
	assertRatio(t, unit.LiquidationPriceUSD, "1", "1")
	assertRatio(t, unit.DropToLiquidationPercent, "50", "1")
	if unit.RedZone == nil || *unit.RedZone || unit.EffectiveHFSource != "derived" || !unit.LiquidationScenarioEligible {
		t.Fatal(unit)
	}
}

func TestAssessDebtRiskRedZoneBoundary(t *testing.T) {
	input, positions := riskFixture(t, "80")
	unit := AssessDebtRisk([]RiskUnitInput{input}, positions).Units[0]
	assertRatio(t, unit.DerivedHF, "5", "4")
	assertRatio(t, unit.DropToLiquidationPercent, "20", "1")
	if unit.RedZone == nil || !*unit.RedZone {
		t.Fatal(unit)
	}
	input, positions = riskFixture(t, "50")
	// 100*2*0.7 / 100 = 1.4 exactly.
	threshold := mustDecimal(t, "0.7")
	input.Collateral[0].LiquidationThreshold = &threshold
	input.Debts[0].Atomic, positions[1].Atomic = "100", "100"
	unit = AssessDebtRisk([]RiskUnitInput{input}, positions).Units[0]
	assertRatio(t, unit.DerivedHF, "7", "5")
	if unit.RedZone == nil || *unit.RedZone {
		t.Fatal(unit)
	}
}

func TestAssessDebtRiskThresholdReachedHasNoDrop(t *testing.T) {
	input, positions := riskFixture(t, "100")
	unit := AssessDebtRisk([]RiskUnitInput{input}, positions).Units[0]
	assertRatio(t, unit.DerivedHF, "1", "1")
	if unit.ScenarioStatus != ThresholdReached || unit.DropToLiquidationPercent != nil || unit.RedZone == nil || !*unit.RedZone {
		t.Fatal(unit)
	}
}

func TestAssessDebtRiskNoDebtKeepsMetricsNull(t *testing.T) {
	input, positions := riskFixture(t, "0")
	input.Debts = nil
	positions = positions[:1]
	unit := AssessDebtRisk([]RiskUnitInput{input}, positions).Units[0]
	if unit.Status != NoDebt || unit.DerivedHF != nil || unit.EffectiveHF != nil || unit.LiquidationPriceUSD != nil || unit.RedZone != nil {
		t.Fatal(unit)
	}
}

func TestAssessDebtRiskLinkedDebtDisablesScenario(t *testing.T) {
	input, positions := riskFixture(t, "50")
	input.Debts[0].PriceRelation = LinkedPrice
	unit := AssessDebtRisk([]RiskUnitInput{input}, positions).Units[0]
	assertRatio(t, unit.DerivedHF, "2", "1")
	if unit.LiquidationScenarioEligible || unit.LiquidationPriceUSD != nil || unit.DropToLiquidationPercent != nil || unit.ScenarioStatus != ScenarioUnavailable {
		t.Fatal(unit)
	}
}

func TestAssessDebtRiskPreservesNativeAndDerived(t *testing.T) {
	input, positions := riskFixture(t, "50")
	native, _ := NewRational(big.NewInt(19), big.NewInt(10))
	input.NativeHF = &native
	unit := AssessDebtRisk([]RiskUnitInput{input}, positions).Units[0]
	assertRatio(t, unit.NativeHF, "19", "10")
	assertRatio(t, unit.DerivedHF, "2", "1")
	assertRatio(t, unit.EffectiveHF, "19", "10")
	if unit.HFComparison != "different" || unit.EffectiveHFSource != "native" {
		t.Fatal(unit)
	}
}

func TestAssessDebtRiskMissingOracleDoesNotBecomeZero(t *testing.T) {
	input, positions := riskFixture(t, "50")
	input.Collateral[0].Amount.OraclePrice.USD = nil
	unit := AssessDebtRisk([]RiskUnitInput{input}, positions).Units[0]
	if unit.DerivedHF != nil || unit.CollateralValueUSD != nil || len(unit.Failures) == 0 || unit.Status != InvalidRiskModel {
		t.Fatal(unit)
	}
}

func TestAssessDebtRiskKeepsRepeatingHFExact(t *testing.T) {
	input, positions := riskFixture(t, "300")
	unit := AssessDebtRisk([]RiskUnitInput{input}, positions).Units[0]
	assertRatio(t, unit.DerivedHF, "1", "3")
	if unit.DropToLiquidationPercent != nil || unit.ScenarioStatus != ThresholdReached {
		t.Fatal(unit)
	}
}
