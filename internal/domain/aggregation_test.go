package domain

import (
	"testing"
	"time"
)

func valuedPosition(wallet, instrument, unit, exposure string, class PositionClass, role AccountingRole, value string) (VerifiedPosition, PositionValuation) {
	decimal, _ := ParseDecimal(value)
	position := VerifiedPosition{
		Wallet: Address(wallet), Chain: "ethereum", Protocol: "synthetic", InstrumentID: instrument,
		ValuationUnitID: unit, ExposureID: exposure, AccountingRole: role, Class: class,
		Atomic: "1", Decimals: 0,
		Snapshot: SnapshotReference{Chain: "ethereum", ChainID: 1, BlockNumber: "0x1", BlockHash: "block", BlockTime: time.Unix(1, 0), Finality: UnknownFinality}, Status: OK,
	}
	valuation := PositionValuation{Wallet: position.Wallet, Chain: position.Chain, Protocol: position.Protocol, InstrumentID: position.InstrumentID,
		ExposureID: position.ExposureID, Class: position.Class, AccountingRole: position.AccountingRole, Atomic: position.Atomic,
		Decimals: position.Decimals, Snapshot: position.Snapshot, Value: &decimal, Status: OK}
	return position, valuation
}

func TestAggregatePortfolioSeparatesGrossAssetsAndDebt(t *testing.T) {
	assetOne, valueOne := valuedPosition("wallet-1", "free", "usdc", "free-1", AssetPosition, EconomicExposure, "10.25")
	assetTwo, valueTwo := valuedPosition("wallet-2", "collateral", "usdc", "deposit-2", AssetPosition, EconomicExposure, "20.75")
	debt, debtValue := valuedPosition("wallet-1", "loan", "usdc", "loan-1", DebtPosition, EconomicExposure, "5")
	got := AggregatePortfolio([]VerifiedPosition{assetOne, assetTwo, debt}, Valuation{Items: []PositionValuation{valueOne, valueTwo, debtValue}})
	if got.AvailableGrossAssetsUSD == nil || got.AvailableGrossAssetsUSD.String() != "31" || got.AvailableGrossDebtUSD == nil || got.AvailableGrossDebtUSD.String() != "5" {
		t.Fatal(got)
	}
	if len(got.Lines) != 2 || got.Lines[0].PositionCount != 2 || got.AssetCoverageComplete || got.DebtCoverageComplete {
		t.Fatal(got)
	}
	if got.AssetCoverage.EconomicPositions != 2 || got.AssetCoverage.ValuedPositions != 2 || got.DebtCoverage.EconomicPositions != 1 || got.DebtCoverage.ValuedPositions != 1 {
		t.Fatal(got)
	}
}

func TestAggregatePortfolioExcludesRepresentationWithoutDoubleCounting(t *testing.T) {
	deposit, depositValue := valuedPosition("wallet", "deposit", "vault-share", "vault-1", VaultPosition, EconomicExposure, "100")
	receipt, receiptValue := valuedPosition("wallet", "receipt", "vault-share", "vault-1", AssetPosition, Representation, "100")
	got := AggregatePortfolio([]VerifiedPosition{deposit, receipt}, Valuation{Items: []PositionValuation{depositValue, receiptValue}})
	if got.AvailableGrossAssetsUSD == nil || got.AvailableGrossAssetsUSD.String() != "100" || len(got.Excluded) != 1 || got.Excluded[0].Reason != Represented || len(got.Failures) != 0 {
		t.Fatal(got)
	}
}

func TestAggregatePortfolioRejectsAmbiguousEconomicExposure(t *testing.T) {
	first, firstValue := valuedPosition("wallet", "deposit", "unit", "same", AssetPosition, EconomicExposure, "100")
	second, secondValue := valuedPosition("wallet", "receipt", "unit", "same", AssetPosition, EconomicExposure, "100")
	got := AggregatePortfolio([]VerifiedPosition{first, second}, Valuation{Items: []PositionValuation{firstValue, secondValue}})
	if got.AvailableGrossAssetsUSD != nil || len(got.Excluded) != 2 || len(got.Failures) != 1 || got.Failures[0].Status != DoubleCountRisk {
		t.Fatal(got)
	}
	if got.AssetCoverage.EconomicPositions != 2 || got.AssetCoverage.ExcludedPositions != 2 {
		t.Fatal(got.AssetCoverage)
	}
}

func TestAggregatePortfolioKeepsPricedSubsetExplicit(t *testing.T) {
	priced, pricedValue := valuedPosition("wallet", "priced", "one", "one", AssetPosition, EconomicExposure, "12")
	unpriced, _ := valuedPosition("wallet", "unpriced", "two", "two", AssetPosition, EconomicExposure, "99")
	unpricedValue := ValuePosition(unpriced, PriceQuote{Status: MissingPrice})
	got := AggregatePortfolio([]VerifiedPosition{priced, unpriced}, Valuation{Items: []PositionValuation{pricedValue, unpricedValue}})
	if got.AvailableGrossAssetsUSD == nil || got.AvailableGrossAssetsUSD.String() != "12" || len(got.Failures) != 1 || got.AssetCoverageComplete {
		t.Fatal(got)
	}
	if got.AssetCoverage.EconomicPositions != 2 || got.AssetCoverage.ValuedPositions != 1 || got.AssetCoverage.UnpricedPositions != 1 {
		t.Fatal(got.AssetCoverage)
	}
}

func TestAggregatePortfolioScopesExposureByWallet(t *testing.T) {
	first, firstValue := valuedPosition("wallet-1", "deposit", "unit", "market", AssetPosition, EconomicExposure, "10")
	second, secondValue := valuedPosition("wallet-2", "deposit", "unit", "market", AssetPosition, EconomicExposure, "20")
	got := AggregatePortfolio([]VerifiedPosition{first, second}, Valuation{Items: []PositionValuation{firstValue, secondValue}})
	if got.AvailableGrossAssetsUSD == nil || got.AvailableGrossAssetsUSD.String() != "30" || len(got.Failures) != 0 {
		t.Fatal(got)
	}
}

func TestAggregatePortfolioDoesNotOverwriteSameInstrumentPositions(t *testing.T) {
	first, firstValue := valuedPosition("wallet", "token", "unit", "free", AssetPosition, EconomicExposure, "10")
	second, secondValue := valuedPosition("wallet", "token", "unit", "collateral", AssetPosition, EconomicExposure, "20")
	got := AggregatePortfolio([]VerifiedPosition{first, second}, Valuation{Items: []PositionValuation{firstValue, secondValue}})
	if got.AvailableGrossAssetsUSD == nil || got.AvailableGrossAssetsUSD.String() != "30" || len(got.Lines) != 1 || got.Lines[0].PositionCount != 2 {
		t.Fatal(got)
	}
}

func TestAggregatePortfolioRejectsDuplicateValuationIdentity(t *testing.T) {
	position, value := valuedPosition("wallet", "token", "unit", "free", AssetPosition, EconomicExposure, "10")
	got := AggregatePortfolio([]VerifiedPosition{position}, Valuation{Items: []PositionValuation{value, value}})
	if got.AvailableGrossAssetsUSD != nil || len(got.Excluded) != 1 || got.Excluded[0].Reason != InvalidResponse || len(got.Failures) == 0 {
		t.Fatal(got)
	}
}

func TestAggregatePortfolioDoesNotMaskInvalidPosition(t *testing.T) {
	position, value := valuedPosition("wallet", "token", "unit", "free", AssetPosition, Representation, "10")
	position.Status = InconsistentSnapshot
	got := AggregatePortfolio([]VerifiedPosition{position}, Valuation{Items: []PositionValuation{value}})
	if got.AvailableGrossAssetsUSD != nil || len(got.Excluded) != 1 || got.Excluded[0].Reason != InconsistentSnapshot || len(got.Failures) != 1 {
		t.Fatal(got)
	}
}
