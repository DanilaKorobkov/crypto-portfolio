// Package application orchestrates discovery through inward-facing ports.
package application

import (
	"context"
	"errors"
	"github.com/DanilaKorobkov/crypto-portfolio/internal/domain"
	"time"
)

type ChainReader interface {
	Read(context.Context, []domain.Address) domain.ChainSnapshot
}
type PositionDiscoverer interface {
	Discover(context.Context, []domain.Address, func(domain.Discovery) error) (domain.Discovery, error)
}
type PositionVerifier interface {
	Verify(context.Context, []domain.NormalizedCandidate) (domain.Verification, error)
}
type PriceResolver interface {
	Resolve(context.Context, []domain.VerifiedPosition) (domain.PriceResolution, error)
}
type DebtRiskResolver interface {
	ResolveRisk(context.Context, []domain.VerifiedPosition) ([]domain.RiskUnitInput, error)
}
type ReportRepository interface{ Save(domain.Report) error }
type Collector struct {
	Readers    []ChainReader
	Discoverer PositionDiscoverer
	Verifier   PositionVerifier
	Prices     PriceResolver
	Risks      DebtRiskResolver
	Reports    ReportRepository
	Now        func() time.Time
}

func (c Collector) Run(ctx context.Context, wallets []domain.Address) (domain.Report, error) {
	now := c.Now
	if now == nil {
		now = time.Now
	}
	report := domain.Report{SchemaVersion: 9, Kind: "feasibility_only", Status: "running",
		StartedAt: now().UTC(), WalletCount: len(wallets),
		NotImplemented: []string{"concrete protocol adapters", "live price resolver", "live debt-risk resolver", "LP ranges", "fees24h", "APR7d", "durable history"}}
	save := func() error { return c.Reports.Save(report) }
	finishWithError := func(cause error) (domain.Report, error) {
		report.Finalize(now())
		return report, errors.Join(cause, save())
	}
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
			report.Normalization = domain.NormalizeDiscovery(report.Discovery)
			report.Failures = append(report.Failures, report.Normalization.Failures...)
			return finishWithError(err)
		}
	} else {
		report.Discovery.CatalogStatus = domain.MissingAPIKey
		for _, wallet := range wallets {
			report.Discovery.Wallets = append(report.Discovery.Wallets, domain.WalletDiscovery{Wallet: wallet, Status: domain.MissingAPIKey})
		}
	}
	report.Normalization = domain.NormalizeDiscovery(report.Discovery)
	report.Failures = append(report.Failures, report.Normalization.Failures...)
	if c.Verifier == nil {
		report.Verification.Status = domain.MissingConfiguration
		if len(report.Normalization.Candidates) > 0 {
			failure := domain.Failure{Scope: "protocol_verification", Status: domain.UnsupportedProtocol}
			report.Verification.Failures = append(report.Verification.Failures, failure)
			report.Failures = append(report.Failures, failure)
		}
	} else {
		verification, err := c.Verifier.Verify(ctx, report.Normalization.Candidates)
		report.Verification = domain.ValidateVerification(verification, report.Normalization.Candidates, report.Chains)
		report.Failures = append(report.Failures, report.Verification.Failures...)
		if saveErr := save(); saveErr != nil {
			return report, saveErr
		}
		if err != nil {
			return finishWithError(err)
		}
	}
	if c.Prices == nil {
		report.Valuation.Status = domain.MissingConfiguration
		if len(report.Verification.Positions) > 0 {
			failure := domain.Failure{Scope: "valuation", Status: domain.MissingPrice}
			report.Valuation.Failures = append(report.Valuation.Failures, failure)
			report.Failures = append(report.Failures, failure)
		}
	} else {
		resolution, err := c.Prices.Resolve(ctx, report.Verification.Positions)
		report.Valuation = domain.ValuePositions(report.Verification.Positions, resolution)
		report.Failures = append(report.Failures, report.Valuation.Failures...)
		if saveErr := save(); saveErr != nil {
			return report, saveErr
		}
		if err != nil {
			return finishWithError(err)
		}
	}
	report.Aggregation = domain.AggregatePortfolio(report.Verification.Positions, report.Valuation)
	report.Failures = append(report.Failures, report.Aggregation.Failures...)
	if c.Risks == nil {
		report.Risk.Status = domain.MissingConfiguration
		for _, position := range report.Verification.Positions {
			if position.Status == domain.OK && position.Class == domain.DebtPosition {
				failure := domain.Failure{Scope: "risk", Status: domain.InvalidRiskModel}
				report.Risk.Failures = append(report.Risk.Failures, failure)
				report.Failures = append(report.Failures, failure)
				break
			}
		}
	} else {
		inputs, err := c.Risks.ResolveRisk(ctx, report.Verification.Positions)
		report.Risk = domain.AssessDebtRisk(inputs, report.Verification.Positions)
		report.Failures = append(report.Failures, report.Risk.Failures...)
		if saveErr := save(); saveErr != nil {
			return report, saveErr
		}
		if err != nil {
			return finishWithError(err)
		}
	}
	report.Finalize(now())
	return report, save()
}
