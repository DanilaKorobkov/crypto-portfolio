// Package domain defines the portfolio discovery bounded context.
// Candidates are evidence to verify, not valued portfolio positions.
package domain

import (
	"errors"
	"regexp"
	"strings"
	"time"
)

type Address string

var addressPattern = regexp.MustCompile(`^0x[0-9a-fA-F]{40}$`)

func ParseAddress(value string) (Address, error) {
	if !addressPattern.MatchString(value) {
		return "", errors.New("invalid EVM address")
	}
	return Address(strings.ToLower(value)), nil
}

type Status string

const (
	OK                     Status = "ok"
	MissingConfiguration   Status = "missing_configuration"
	MissingAPIKey          Status = "missing_api_key"
	AccessDenied           Status = "access_denied"
	AuthRequired           Status = "auth_required"
	PaymentRequired        Status = "payment_required"
	RateLimited            Status = "rate_limited"
	ProviderStopped        Status = "provider_stopped"
	Timeout                Status = "timeout"
	InvalidResponse        Status = "invalid_response"
	APIError               Status = "api_error"
	TransportError         Status = "transport_error"
	WrongChain             Status = "wrong_chain"
	InconsistentSnapshot   Status = "inconsistent_snapshot"
	RequestLimit           Status = "request_limit"
	PageLimit              Status = "page_limit"
	UnsafePagination       Status = "unsafe_pagination"
	PaginationCycle        Status = "pagination_cycle"
	InconsistentPagination Status = "inconsistent_pagination"
)

func (s Status) StopsProvider() bool {
	return s == AccessDenied || s == AuthRequired || s == PaymentRequired || s == RateLimited
}

type Failure struct {
	Scope  string `json:"scope"`
	Status Status `json:"status"`
}
type NativeBalance struct {
	Wallet Address `json:"wallet"`
	Atomic string  `json:"atomic,omitempty"`
	Status Status  `json:"status"`
}
type ChainSnapshot struct {
	ChainID     uint64          `json:"chain_id"`
	Status      Status          `json:"status"`
	BlockNumber string          `json:"block_number,omitempty"`
	BlockHash   string          `json:"block_hash,omitempty"`
	BlockTime   time.Time       `json:"block_time,omitempty"`
	Balances    []NativeBalance `json:"balances"`
}
type ChainSupport struct {
	ID                string `json:"id"`
	ExternalID        string `json:"external_id,omitempty"`
	SupportsPositions *bool  `json:"supports_positions"`
}
type Candidate struct {
	ID       string `json:"id"`
	Chain    string `json:"chain,omitempty"`
	Protocol string `json:"protocol,omitempty"`
	Kind     string `json:"kind,omitempty"`
	Atomic   string `json:"atomic,omitempty"`
	Decimals *int   `json:"decimals,omitempty"`
	Trash    *bool  `json:"trash,omitempty"`
	Verified *bool  `json:"verified,omitempty"`
}
type WalletDiscovery struct {
	Wallet           Address     `json:"wallet"`
	Status           Status      `json:"status"`
	Pages            int         `json:"pages"`
	ResponseComplete bool        `json:"response_complete"`
	Candidates       []Candidate `json:"candidates"`
}
type Discovery struct {
	CatalogStatus   Status            `json:"catalog_status"`
	CatalogComplete bool              `json:"catalog_complete"`
	Chains          []ChainSupport    `json:"chains"`
	Wallets         []WalletDiscovery `json:"wallets"`
	Requests        int               `json:"requests"`
	// A completed provider response never proves protocol or portfolio coverage.
	CoverageComplete bool `json:"coverage_complete"`
}
type Report struct {
	SchemaVersion     int             `json:"schema_version"`
	Kind              string          `json:"kind"`
	Status            string          `json:"status"`
	StartedAt         time.Time       `json:"started_at"`
	CompletedAt       time.Time       `json:"completed_at,omitempty"`
	PortfolioComplete bool            `json:"portfolio_complete"`
	WalletCount       int             `json:"wallet_count"`
	Chains            []ChainSnapshot `json:"chains"`
	Discovery         Discovery       `json:"discovery"`
	Failures          []Failure       `json:"failures"`
	NotImplemented    []string        `json:"not_implemented"`
}

// Finalize deliberately has no "complete" state until coverage is verified.
func (r *Report) Finalize(now time.Time) {
	r.CompletedAt = now.UTC()
	r.Status = "failed"
	for _, chain := range r.Chains {
		for _, balance := range chain.Balances {
			if balance.Status == OK {
				r.Status = "partial"
			}
		}
	}
	for _, wallet := range r.Discovery.Wallets {
		if wallet.Pages > 0 {
			r.Status = "partial"
		}
	}
}
