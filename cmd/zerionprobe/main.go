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

func probe(ctx context.Context, key string, discoverer catalogDiscoverer) (domain.Status, int, int, error) {
	if key == "" {
		return domain.MissingAPIKey, 0, 0, errors.New("ZERION_API_KEY is required")
	}
	result, err := discoverer.Discover(ctx, nil, func(domain.Discovery) error { return nil })
	if err != nil {
		return result.CatalogStatus, len(result.Chains), result.Requests, errors.New("catalog probe failed")
	}
	if result.CatalogStatus != domain.OK || !result.CatalogComplete || len(result.Chains) == 0 {
		return result.CatalogStatus, len(result.Chains), result.Requests, errors.New("catalog contract not satisfied")
	}
	return result.CatalogStatus, len(result.Chains), result.Requests, nil
}

func run() error {
	key := os.Getenv("ZERION_API_KEY")
	discoverer := zerion.New(transport.New(), key)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	status, chains, requests, err := probe(ctx, key, discoverer)
	// Counts and the normalized status are safe D1 evidence. Provider payloads,
	// chain identifiers, credentials, and transport diagnostics stay private.
	fmt.Printf("zerion_catalog status=%s complete=%t chains=%d requests=%d\n", status, err == nil, chains, requests)
	return err
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
