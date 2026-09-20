// Command zerionprobe validates the authenticated Zerion chain-catalog contract.
// It never accepts a wallet address and never prints credentials or response bodies.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/DanilaKorobkov/crypto-portfolio/internal/adapters/transport"
	"github.com/DanilaKorobkov/crypto-portfolio/internal/adapters/zerion"
	"github.com/DanilaKorobkov/crypto-portfolio/internal/domain"
)

type catalogDiscoverer interface {
	Discover(context.Context, []domain.Address, func(domain.Discovery) error) (domain.Discovery, error)
}

var publicTestAddress = domain.Address("0x0000000000000000000000000000000000000000")

type probeResult struct {
	CatalogStatus    domain.Status
	CatalogComplete  bool
	Chains           int
	PositionStatus   domain.Status
	PositionComplete bool
	Pages            int
	Candidates       int
	Requests         int
}

func probe(ctx context.Context, key string, discoverer catalogDiscoverer) (probeResult, error) {
	if key == "" {
		return probeResult{CatalogStatus: domain.MissingAPIKey}, errors.New("ZERION_API_KEY is required")
	}
	result, err := discoverer.Discover(ctx, []domain.Address{publicTestAddress}, func(domain.Discovery) error { return nil })
	summary := probeResult{
		CatalogStatus: result.CatalogStatus, CatalogComplete: result.CatalogComplete,
		Chains: len(result.Chains), Requests: result.Requests,
	}
	if len(result.Wallets) == 1 {
		summary.PositionStatus = result.Wallets[0].Status
		summary.PositionComplete = result.Wallets[0].ResponseComplete
		summary.Pages = result.Wallets[0].Pages
		summary.Candidates = len(result.Wallets[0].Candidates)
	}
	if err != nil {
		return summary, errors.New("provider contract probe failed")
	}
	if result.CatalogStatus != domain.OK || !result.CatalogComplete || len(result.Chains) == 0 {
		return summary, errors.New("catalog contract not satisfied")
	}
	if len(result.Wallets) != 1 || result.Wallets[0].Wallet != publicTestAddress || result.Wallets[0].Status != domain.OK || !result.Wallets[0].ResponseComplete {
		return summary, errors.New("positions contract not satisfied")
	}
	return summary, nil
}

func run() error {
	key := os.Getenv("ZERION_API_KEY")
	discoverer := zerion.New(transport.New(), key)
	// Feasibility probes favor quota safety over throughput. The production
	// discoverer remains independently bounded and configurable.
	discoverer.Interval = 2 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	result, err := probe(ctx, key, discoverer)
	// Counts and the normalized status are safe D1 evidence. Provider payloads,
	// chain identifiers, credentials, and transport diagnostics stay private.
	fmt.Printf("zerion_contract catalog_status=%s catalog_complete=%t chains=%d positions_status=%s positions_complete=%t pages=%d candidates=%d requests=%d\n",
		result.CatalogStatus, result.CatalogComplete, result.Chains, result.PositionStatus, result.PositionComplete, result.Pages, result.Candidates, result.Requests)
	return err
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
