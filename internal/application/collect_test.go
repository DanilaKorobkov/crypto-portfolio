package application

import (
	"context"
	"errors"
	"github.com/DanilaKorobkov/crypto-portfolio/internal/domain"
	"testing"
	"time"
)

type memoryReports struct {
	reports []domain.Report
	err     error
}

func (m *memoryReports) Save(r domain.Report) error { m.reports = append(m.reports, r); return m.err }

type reader struct{ status domain.Status }

func (r reader) Read(context.Context, []domain.Address) domain.ChainSnapshot {
	return domain.ChainSnapshot{Status: r.status, Balances: []domain.NativeBalance{{Status: r.status, Atomic: "0"}}}
}
func TestNetworkFailureKeepsOtherData(t *testing.T) {
	store := &memoryReports{}
	c := Collector{Reports: store, Readers: []ChainReader{reader{domain.OK}, reader{domain.AccessDenied}}}
	r, err := c.Run(context.Background(), []domain.Address{"fixture"})
	if err != nil || r.Status != "partial" || len(r.Chains) != 2 || r.PortfolioComplete {
		t.Fatal(r, err)
	}
	if len(store.reports) < 4 {
		t.Fatal("checkpoints missing")
	}
}
func TestUnconfiguredReport(t *testing.T) {
	r, err := (Collector{Reports: &memoryReports{}}).Run(context.Background(), nil)
	if err != nil || r.Status != "failed" || len(r.Failures) != 2 || r.Discovery.CatalogStatus != domain.MissingAPIKey {
		t.Fatal(r, err)
	}
}
func TestPersistenceFailureStopsCollection(t *testing.T) {
	expected := errors.New("disk full")
	_, err := (Collector{Reports: &memoryReports{err: expected}}).Run(context.Background(), nil)
	if !errors.Is(err, expected) {
		t.Fatal(err)
	}
}

func TestCollectorNormalizesDiscoveryWithoutClaimingCoverage(t *testing.T) {
	discoverer := discoveryStub{result: domain.Discovery{Wallets: []domain.WalletDiscovery{{
		Wallet: "wallet", Candidates: []domain.Candidate{{ID: "token", Chain: "ETHEREUM", Atomic: "001", Decimals: pointer(18)}},
	}}}}
	r, err := (Collector{Reports: &memoryReports{}, Discoverer: discoverer}).Run(context.Background(), []domain.Address{"wallet"})
	if err != nil || r.SchemaVersion != 9 || len(r.Normalization.Candidates) != 1 || r.Normalization.Candidates[0].Atomic != "1" || r.Normalization.CoverageComplete {
		t.Fatal(r, err)
	}
	if r.Verification.Status != domain.MissingConfiguration || len(r.Verification.Positions) != 0 || len(r.Verification.Failures) != 1 {
		t.Fatal(r.Verification)
	}
}

type discoveryStub struct{ result domain.Discovery }

func (d discoveryStub) Discover(_ context.Context, _ []domain.Address, checkpoint func(domain.Discovery) error) (domain.Discovery, error) {
	return d.result, checkpoint(d.result)
}

func pointer(value int) *int { return &value }

type verifierStub struct {
	result domain.Verification
	err    error
}

func (v verifierStub) Verify(context.Context, []domain.NormalizedCandidate) (domain.Verification, error) {
	return v.result, v.err
}

func TestCollectorPreservesPartialProtocolVerification(t *testing.T) {
	now := time.Unix(1, 0).UTC()
	snapshot := domain.ChainSnapshot{Chain: "ethereum", ChainID: 1, Status: domain.OK, BlockNumber: "0x123", BlockHash: "0xabc", BlockTime: now}
	reference, _ := domain.ReferenceSnapshot(snapshot)
	discoverer := discoveryStub{result: domain.Discovery{Wallets: []domain.WalletDiscovery{{Wallet: "wallet", Candidates: []domain.Candidate{{ID: "token", Chain: "ethereum", Atomic: "1", Decimals: pointer(18)}}}}}}
	verification := domain.Verification{
		Status: domain.AccessDenied,
		Positions: []domain.VerifiedPosition{{Wallet: "wallet", Chain: "ethereum", InstrumentID: "token", Class: domain.AssetPosition,
			ValuationUnitID: "token-unit", ExposureID: "asset-1", AccountingRole: domain.EconomicExposure, Atomic: "1", Decimals: 18, Source: "synthetic", Snapshot: reference, Status: domain.OK}},
		Failures: []domain.Failure{{Scope: "protocol/second", Status: domain.AccessDenied}},
	}
	price, _ := domain.ParseDecimal("2")
	store := &memoryReports{}
	r, err := (Collector{Reports: store, Readers: []ChainReader{staticReader{snapshot}}, Discoverer: discoverer, Verifier: verifierStub{result: verification},
		Prices: priceResolverStub{result: domain.PriceResolution{Status: domain.OK, Quotes: []domain.PriceQuote{{ChainID: 1, UnitID: "token-unit", USD: &price, Source: "synthetic", AsOf: now, Status: domain.OK}}}}}).Run(context.Background(), []domain.Address{"wallet"})
	if err != nil || r.Status != "partial" || len(r.Verification.Positions) != 1 || len(r.Verification.Failures) != 1 || r.Verification.CoverageComplete {
		t.Fatal(r, err)
	}
	if len(store.reports) < 3 {
		t.Fatal("verification checkpoint missing")
	}
	if r.Aggregation.AvailableGrossAssetsUSD == nil || r.Aggregation.AvailableGrossAssetsUSD.String() != "0.000000000000000002" {
		t.Fatal(r.Aggregation)
	}
}

type staticReader struct{ snapshot domain.ChainSnapshot }

func (reader staticReader) Read(context.Context, []domain.Address) domain.ChainSnapshot {
	return reader.snapshot
}

type priceResolverStub struct {
	result domain.PriceResolution
	err    error
}

type riskResolverStub struct {
	inputs []domain.RiskUnitInput
	err    error
}

func (resolver riskResolverStub) ResolveRisk(context.Context, []domain.VerifiedPosition) ([]domain.RiskUnitInput, error) {
	return resolver.inputs, resolver.err
}

func (resolver priceResolverStub) Resolve(context.Context, []domain.VerifiedPosition) (domain.PriceResolution, error) {
	return resolver.result, resolver.err
}

func TestCollectorFinalizesCheckpointOnResolverError(t *testing.T) {
	expected := errors.New("price source failed")
	store := &memoryReports{}
	r, err := (Collector{Reports: store, Prices: priceResolverStub{err: expected}}).Run(context.Background(), nil)
	if !errors.Is(err, expected) || r.Status == "running" || r.CompletedAt.IsZero() {
		t.Fatal(r, err)
	}
	last := store.reports[len(store.reports)-1]
	if last.Status == "running" || last.CompletedAt.IsZero() {
		t.Fatal(last)
	}
}

func TestCollectorFinalizesCheckpointOnRiskResolverError(t *testing.T) {
	expected := errors.New("risk source failed")
	store := &memoryReports{}
	r, err := (Collector{Reports: store, Risks: riskResolverStub{err: expected}}).Run(context.Background(), nil)
	if !errors.Is(err, expected) || r.Status == "running" || r.CompletedAt.IsZero() {
		t.Fatal(r, err)
	}
	last := store.reports[len(store.reports)-1]
	if last.Status == "running" || last.CompletedAt.IsZero() {
		t.Fatal(last)
	}
}
