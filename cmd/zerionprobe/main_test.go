package main

import (
	"context"
	"errors"
	"testing"

	"github.com/DanilaKorobkov/crypto-portfolio/internal/domain"
)

type fakeDiscoverer struct {
	result  domain.Discovery
	err     error
	wallets []domain.Address
}

func (f *fakeDiscoverer) Discover(_ context.Context, wallets []domain.Address, _ func(domain.Discovery) error) (domain.Discovery, error) {
	f.wallets = wallets
	return f.result, f.err
}

func TestProbeRequiresKeyWithoutCallingProvider(t *testing.T) {
	fake := &fakeDiscoverer{}
	status, chains, requests, err := probe(context.Background(), "", fake)
	if err == nil || status != domain.MissingAPIKey || chains != 0 || requests != 0 || fake.wallets != nil {
		t.Fatalf("unexpected result: %s %d %d %v", status, chains, requests, err)
	}
}

func TestProbeUsesCatalogOnly(t *testing.T) {
	fake := &fakeDiscoverer{result: domain.Discovery{
		CatalogStatus: domain.OK, CatalogComplete: true, Requests: 1,
		Chains: []domain.ChainSupport{{ID: "fixture"}},
	}}
	status, chains, requests, err := probe(context.Background(), "secret", fake)
	if err != nil || status != domain.OK || chains != 1 || requests != 1 || len(fake.wallets) != 0 {
		t.Fatalf("unexpected result: %s %d %d %v", status, chains, requests, err)
	}
}

func TestProbeRejectsIncompleteEmptyAndAdapterErrors(t *testing.T) {
	tests := []fakeDiscoverer{
		{result: domain.Discovery{CatalogStatus: domain.OK, CatalogComplete: false}},
		{result: domain.Discovery{CatalogStatus: domain.OK, CatalogComplete: true}},
		{result: domain.Discovery{CatalogStatus: domain.TransportError}, err: errors.New("network")},
	}
	for i := range tests {
		if _, _, _, err := probe(context.Background(), "secret", &tests[i]); err == nil {
			t.Fatalf("case %d unexpectedly passed", i)
		}
	}
}
