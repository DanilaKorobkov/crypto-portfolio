// Package domain defines the portfolio discovery bounded context.
// Candidates are evidence to verify, not valued portfolio positions.
package domain

import (
	"errors"
	"math/big"
	"regexp"
	"slices"
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
	UnsupportedProtocol    Status = "unsupported_protocol"
	MissingPrice           Status = "missing_price"
	DoubleCountRisk        Status = "double_count_risk"
	Represented            Status = "represented"
	InvalidRiskModel       Status = "invalid_risk_model"
	NoDebt                 Status = "no_debt"
	ScenarioUnavailable    Status = "scenario_unavailable"
	ThresholdReached       Status = "threshold_reached_or_crossed"
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
	Chain       string          `json:"chain"`
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

// NormalizedCandidate is provider evidence prepared for later protocol
// verification. It is not yet a portfolio position or a valuation.
type NormalizedCandidate struct {
	Wallet   Address `json:"wallet"`
	ID       string  `json:"id"`
	Chain    string  `json:"chain"`
	Protocol string  `json:"protocol,omitempty"`
	Kind     string  `json:"kind,omitempty"`
	Atomic   string  `json:"atomic"`
	Decimals int     `json:"decimals"`
	Trash    *bool   `json:"trash,omitempty"`
	Verified *bool   `json:"verified,omitempty"`
	Status   Status  `json:"status"`
}

type Normalization struct {
	Candidates       []NormalizedCandidate `json:"candidates"`
	Failures         []Failure             `json:"failures"`
	CoverageComplete bool                  `json:"coverage_complete"`
}

// NormalizeDiscovery validates exact quantities and collapses only identical
// evidence from the same wallet. Conflicting evidence is retained and marked;
// candidates from different wallets are never merged.
func NormalizeDiscovery(discovery Discovery) Normalization {
	result := Normalization{}
	seen := make(map[string][]int)
	conflicts := make(map[string]bool)
	for _, wallet := range discovery.Wallets {
		for _, candidate := range wallet.Candidates {
			normalized, ok := normalizeCandidate(wallet.Wallet, candidate)
			if !ok {
				result.Failures = append(result.Failures, Failure{Scope: candidateScope(wallet.Wallet, candidate.ID), Status: InvalidResponse})
				continue
			}
			key := string(wallet.Wallet) + "\x00" + normalized.Chain + "\x00" + normalized.ID
			indexes := seen[key]
			duplicate := false
			for _, previous := range indexes {
				if equalCandidate(result.Candidates[previous], normalized) {
					duplicate = true
					break
				}
			}
			if duplicate {
				continue
			}
			if len(indexes) > 0 {
				for _, previous := range indexes {
					result.Candidates[previous].Status = InconsistentPagination
				}
				normalized.Status = InconsistentPagination
				if !conflicts[key] {
					result.Failures = append(result.Failures, Failure{Scope: candidateScope(wallet.Wallet, candidate.ID), Status: InconsistentPagination})
					conflicts[key] = true
				}
			}
			seen[key] = append(indexes, len(result.Candidates))
			result.Candidates = append(result.Candidates, normalized)
		}
	}
	return result
}

func normalizeCandidate(wallet Address, candidate Candidate) (NormalizedCandidate, bool) {
	id := strings.TrimSpace(candidate.ID)
	chain := strings.ToLower(strings.TrimSpace(candidate.Chain))
	if wallet == "" || id == "" || chain == "" || candidate.Decimals == nil || *candidate.Decimals < 0 || *candidate.Decimals > 255 {
		return NormalizedCandidate{}, false
	}
	atomic := new(big.Int)
	if strings.TrimSpace(candidate.Atomic) != candidate.Atomic || candidate.Atomic == "" {
		return NormalizedCandidate{}, false
	}
	if _, ok := atomic.SetString(candidate.Atomic, 10); !ok {
		return NormalizedCandidate{}, false
	}
	normalized := NormalizedCandidate{
		Wallet: wallet, ID: id, Chain: chain,
		Protocol: strings.ToLower(strings.TrimSpace(candidate.Protocol)),
		Kind:     strings.ToLower(strings.TrimSpace(candidate.Kind)),
		Atomic:   atomic.String(), Decimals: *candidate.Decimals,
		Trash: candidate.Trash, Verified: candidate.Verified, Status: OK,
	}
	return normalized, true
}

func equalCandidate(a, b NormalizedCandidate) bool {
	a.Status, b.Status = "", ""
	return a.Wallet == b.Wallet && a.ID == b.ID && a.Chain == b.Chain && a.Protocol == b.Protocol && a.Kind == b.Kind &&
		a.Atomic == b.Atomic && a.Decimals == b.Decimals && equalOptionalBool(a.Trash, b.Trash) && equalOptionalBool(a.Verified, b.Verified)
}

func equalOptionalBool(a, b *bool) bool {
	return a == nil && b == nil || a != nil && b != nil && *a == *b
}

func candidateScope(wallet Address, id string) string {
	parts := []string{"candidate", string(wallet), strings.TrimSpace(id)}
	return strings.Join(slices.DeleteFunc(parts, func(value string) bool { return value == "" }), "/")
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
	Normalization     Normalization   `json:"normalization"`
	Verification      Verification    `json:"verification"`
	Valuation         Valuation       `json:"valuation"`
	Aggregation       Aggregation     `json:"aggregation"`
	Risk              RiskAssessment  `json:"risk"`
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
	if len(r.Verification.Positions) > 0 {
		r.Status = "partial"
	}
}
