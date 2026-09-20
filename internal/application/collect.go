// Package application orchestrates discovery through inward-facing ports.
package application

import (
	"context"
	"github.com/DanilaKorobkov/crypto-portfolio/internal/domain"
	"time"
)

type ChainReader interface {
	Read(context.Context, []domain.Address) domain.ChainSnapshot
}
type PositionDiscoverer interface {
	Discover(context.Context, []domain.Address, func(domain.Discovery) error) (domain.Discovery, error)
}
type ReportRepository interface{ Save(domain.Report) error }
type Collector struct {
	Readers    []ChainReader
	Discoverer PositionDiscoverer
	Reports    ReportRepository
	Now        func() time.Time
}

func (c Collector) Run(ctx context.Context, wallets []domain.Address) (domain.Report, error) {
	now := c.Now
	if now == nil {
		now = time.Now
	}
	report := domain.Report{SchemaVersion: 3, Kind: "feasibility_only", Status: "running",
		StartedAt: now().UTC(), WalletCount: len(wallets),
		NotImplemented: []string{"protocol verification", "USD valuation", "HF", "LP ranges", "fees24h", "APR7d", "durable history"}}
	save := func() error { return c.Reports.Save(report) }
	if len(wallets) == 0 {
		report.Failures = append(report.Failures, domain.Failure{Scope: "wallets", Status: domain.MissingConfiguration})
	}
	if len(c.Readers) == 0 {
		report.Failures = append(report.Failures, domain.Failure{Scope: "rpc", Status: domain.MissingConfiguration})
	}
	if err := save(); err != nil {
		return report, err
	}
	for _, reader := range c.Readers {
		if ctx.Err() != nil {
			report.Failures = append(report.Failures, domain.Failure{Scope: "remaining_rpc", Status: domain.Timeout})
			break
		}
		report.Chains = append(report.Chains, reader.Read(ctx, wallets))
		if err := save(); err != nil {
			return report, err
		}
	}
	if c.Discoverer != nil {
		discovery, err := c.Discoverer.Discover(ctx, wallets, func(value domain.Discovery) error { report.Discovery = value; return save() })
		report.Discovery = discovery
		if err != nil {
			return report, err
		}
	} else {
		report.Discovery.CatalogStatus = domain.MissingAPIKey
		for _, wallet := range wallets {
			report.Discovery.Wallets = append(report.Discovery.Wallets, domain.WalletDiscovery{Wallet: wallet, Status: domain.MissingAPIKey})
		}
	}
	report.Finalize(now())
	return report, save()
}
