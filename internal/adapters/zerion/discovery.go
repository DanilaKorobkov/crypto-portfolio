// Package zerion translates provider resources into discovery-domain candidates.
package zerion

import (
	"bytes"
	"context"
	"encoding/json"
	"net/url"
	"time"

	"github.com/DanilaKorobkov/crypto-portfolio/internal/domain"
)

type Transport interface {
	Do(context.Context, string, string, string, string) ([]byte, domain.Status)
}
type Discoverer struct {
	HTTP        Transport
	Key         string
	MaxRequests int
	MaxPages    int
	Interval    time.Duration
}

func New(http Transport, key string) *Discoverer {
	return &Discoverer{HTTP: http, Key: key, MaxRequests: 60, MaxPages: 20, Interval: 400 * time.Millisecond}
}

type resource struct {
	ID            string          `json:"id"`
	Type          string          `json:"type"`
	Attributes    json.RawMessage `json:"attributes"`
	Relationships struct {
		Chain struct {
			Data struct {
				ID string `json:"id"`
			} `json:"data"`
		} `json:"chain"`
		Dapp struct {
			Data struct {
				ID string `json:"id"`
			} `json:"data"`
		} `json:"dapp"`
	} `json:"relationships"`
}
type page struct {
	Data  *[]json.RawMessage `json:"data"`
	Links *struct {
		Next *string `json:"next"`
	} `json:"links"`
	Errors json.RawMessage `json:"errors"`
}
type list struct {
	status   domain.Status
	complete bool
	pages    int
	rows     []resource
}
type session struct {
	adapter  *Discoverer
	requests int
	stopped  domain.Status
	next     time.Time
}

func (s *session) fetch(ctx context.Context, endpoint string) ([]byte, domain.Status) {
	if s.stopped != "" {
		return nil, domain.ProviderStopped
	}
	if s.requests >= s.adapter.MaxRequests {
		return nil, domain.RequestLimit
	}
	if delay := time.Until(s.next); delay > 0 {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return nil, domain.Timeout
		case <-timer.C:
		}
	}
	if ctx.Err() != nil {
		return nil, domain.Timeout
	}
	s.requests++
	s.next = time.Now().Add(s.adapter.Interval)
	raw, status := s.adapter.HTTP.Do(ctx, "GET", endpoint, "", s.adapter.Key)
	if status.StopsProvider() {
		s.stopped = status
	}
	return raw, status
}

func validNext(link, path string, required url.Values) bool {
	u, err := url.Parse(link)
	if err != nil || len(link) > 8192 || u.Scheme != "https" || u.Host != "api.zerion.io" || u.Path != path || u.RawPath != "" || u.User != nil || u.Fragment != "" {
		return false
	}
	query, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return false
	}
	for key, want := range required {
		got := query[key]
		if len(got) != len(want) {
			return false
		}
		for i := range want {
			if got[i] != want[i] {
				return false
			}
		}
	}
	for key, values := range query {
		if _, retained := required[key]; retained {
			continue
		}
		if (key != "page[after]" && key != "page[before]" && key != "page[size]") || len(values) != 1 {
			return false
		}
	}
	return true
}

func (s *session) collect(ctx context.Context, path, kind string, query url.Values, checkpoint func(list) error) (list, error) {
	endpoint := "https://api.zerion.io" + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	result := list{status: domain.PageLimit}
	seenURLs := map[string]bool{}
	seenIDs := map[string]json.RawMessage{}
	finish := func(status domain.Status) (list, error) { result.status = status; return result, checkpoint(result) }
	for range s.adapter.MaxPages {
		if seenURLs[endpoint] {
			return finish(domain.PaginationCycle)
		}
		seenURLs[endpoint] = true
		raw, status := s.fetch(ctx, endpoint)
		if status != domain.OK {
			return finish(status)
		}
		var p page
		if json.Unmarshal(raw, &p) != nil {
			return finish(domain.InvalidResponse)
		}
		if len(p.Errors) > 0 && string(p.Errors) != "null" && string(p.Errors) != "[]" {
			return finish(domain.APIError)
		}
		if p.Data == nil || p.Links == nil {
			return finish(domain.InvalidResponse)
		}
		for _, rawRow := range *p.Data {
			var row resource
			if json.Unmarshal(rawRow, &row) != nil || row.Type != kind || row.ID == "" || len(row.Attributes) == 0 || row.Attributes[0] != '{' {
				return finish(domain.InvalidResponse)
			}
			if previous, found := seenIDs[row.ID]; found {
				if !bytes.Equal(previous, rawRow) {
					return finish(domain.InconsistentPagination)
				}
				continue
			}
			seenIDs[row.ID] = rawRow
			result.rows = append(result.rows, row)
		}
		result.pages++
		if p.Links.Next == nil {
			result.complete = true
			return finish(domain.OK)
		}
		if !validNext(*p.Links.Next, path, query) {
			return finish(domain.UnsafePagination)
		}
		if err := checkpoint(result); err != nil {
			return result, err
		}
		endpoint = *p.Links.Next
	}
	return finish(domain.PageLimit)
}

func (d *Discoverer) Discover(ctx context.Context, wallets []domain.Address, checkpoint func(domain.Discovery) error) (domain.Discovery, error) {
	result := domain.Discovery{}
	if d.Key == "" {
		result.CatalogStatus = domain.MissingAPIKey
		for _, wallet := range wallets {
			result.Wallets = append(result.Wallets, domain.WalletDiscovery{Wallet: wallet, Status: domain.MissingAPIKey})
		}
		return result, checkpoint(result)
	}
	s := session{adapter: d}
	_, err := s.collect(ctx, "/v1/chains/", "chains", nil, func(value list) error {
		result.CatalogStatus, result.CatalogComplete, result.Requests = value.status, value.complete, s.requests
		result.Chains = nil
		for _, row := range value.rows {
			var attrs struct {
				ExternalID string `json:"external_id"`
				Flags      struct {
					Positions *bool `json:"supports_positions"`
				} `json:"flags"`
			}
			if json.Unmarshal(row.Attributes, &attrs) != nil {
				result.CatalogStatus = domain.InvalidResponse
				result.CatalogComplete = false
				continue
			}
			result.Chains = append(result.Chains, domain.ChainSupport{ID: row.ID, ExternalID: attrs.ExternalID, SupportsPositions: attrs.Flags.Positions})
		}
		return checkpoint(result)
	})
	if err != nil {
		return result, err
	}
	for _, wallet := range wallets {
		index := len(result.Wallets)
		result.Wallets = append(result.Wallets, domain.WalletDiscovery{Wallet: wallet})
		_, err := s.collect(ctx, "/v1/wallets/"+string(wallet)+"/positions/", "positions", url.Values{
			"filter[positions]": {"no_filter"}, "filter[trash]": {"no_filter"}, "currency": {"usd"},
		}, func(value list) error {
			target := &result.Wallets[index]
			target.Status, target.ResponseComplete, target.Pages = value.status, value.complete, value.pages
			target.Candidates = nil
			result.Requests = s.requests
			for _, row := range value.rows {
				var attrs struct {
					Protocol string `json:"protocol"`
					Kind     string `json:"position_type"`
					Quantity struct {
						Atomic   string `json:"int"`
						Decimals *int   `json:"decimals"`
					} `json:"quantity"`
					Flags struct {
						Trash *bool `json:"is_trash"`
					} `json:"flags"`
					Fungible struct {
						Flags struct {
							Verified *bool `json:"verified"`
						} `json:"flags"`
					} `json:"fungible_info"`
				}
				if json.Unmarshal(row.Attributes, &attrs) != nil {
					target.Status = domain.InvalidResponse
					target.ResponseComplete = false
					continue
				}
				protocol := row.Relationships.Dapp.Data.ID
				if protocol == "" {
					protocol = attrs.Protocol
				}
				target.Candidates = append(target.Candidates, domain.Candidate{ID: row.ID, Chain: row.Relationships.Chain.Data.ID, Protocol: protocol, Kind: attrs.Kind,
					Atomic: attrs.Quantity.Atomic, Decimals: attrs.Quantity.Decimals, Trash: attrs.Flags.Trash, Verified: attrs.Fungible.Flags.Verified})
			}
			return checkpoint(result)
		})
		if err != nil {
			return result, err
		}
	}
	return result, nil
}
