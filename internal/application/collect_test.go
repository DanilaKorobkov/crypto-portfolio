package application

import (
	"context"
	"errors"
	"github.com/DanilaKorobkov/crypto-portfolio/internal/domain"
	"testing"
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
