package domain

import (
	"math/big"
	"strconv"
	"strings"
	"time"
)

type PositionClass string
type AccountingRole string

const (
	AssetPosition    PositionClass  = "asset"
	DebtPosition     PositionClass  = "debt"
	LPPosition       PositionClass  = "lp"
	VaultPosition    PositionClass  = "vault"
	EconomicExposure AccountingRole = "economic"
	Representation   AccountingRole = "representation"
)

func (class PositionClass) Valid() bool {
	return class == AssetPosition || class == DebtPosition || class == LPPosition || class == VaultPosition
}

// VerifiedPosition is protocol-confirmed state. A normalized discovery
// candidate cannot become one without a verifier naming its source and
// snapshot. Atomic remains an exact base-unit integer string.
type VerifiedPosition struct {
	Wallet          Address           `json:"wallet"`
	Chain           string            `json:"chain"`
	Protocol        string            `json:"protocol"`
	InstrumentID    string            `json:"instrument_id"`
	ValuationUnitID string            `json:"valuation_unit_id"`
	ExposureID      string            `json:"exposure_id"`
	AccountingRole  AccountingRole    `json:"accounting_role"`
	Class           PositionClass     `json:"class"`
	Atomic          string            `json:"atomic"`
	Decimals        int               `json:"decimals"`
	Source          string            `json:"source"`
	Snapshot        SnapshotReference `json:"snapshot"`
	Status          Status            `json:"status"`
}

type SnapshotFinality string

const UnknownFinality SnapshotFinality = "unknown"

// SnapshotReference identifies observed chain state without claiming that the
// block is finalized. Finality stays unknown until a chain-specific policy is
// implemented and verified.
type SnapshotReference struct {
	Chain       string           `json:"chain"`
	ChainID     uint64           `json:"chain_id"`
	BlockNumber string           `json:"block_number"`
	BlockHash   string           `json:"block_hash"`
	BlockTime   time.Time        `json:"block_time"`
	Finality    SnapshotFinality `json:"finality"`
}

func ReferenceSnapshot(snapshot ChainSnapshot) (SnapshotReference, bool) {
	chain := strings.ToLower(strings.TrimSpace(snapshot.Chain))
	if snapshot.Status != OK || chain == "" || snapshot.ChainID == 0 || strings.TrimSpace(snapshot.BlockNumber) == "" ||
		strings.TrimSpace(snapshot.BlockHash) == "" || snapshot.BlockTime.IsZero() {
		return SnapshotReference{}, false
	}
	return SnapshotReference{
		Chain: chain, ChainID: snapshot.ChainID, BlockNumber: snapshot.BlockNumber, BlockHash: snapshot.BlockHash,
		BlockTime: snapshot.BlockTime.UTC(), Finality: UnknownFinality,
	}, true
}

type Verification struct {
	Status           Status             `json:"status"`
	Positions        []VerifiedPosition `json:"positions"`
	Failures         []Failure          `json:"failures"`
	CoverageComplete bool               `json:"coverage_complete"`
}

// ValidateVerification ensures adapter output belongs to the supplied
// discovery evidence and to a snapshot actually collected in this report.
func ValidateVerification(verification Verification, candidates []NormalizedCandidate, snapshots []ChainSnapshot) Verification {
	knownCandidates := make(map[string]bool)
	for _, candidate := range candidates {
		if candidate.Status == OK {
			knownCandidates[candidateKey(candidate.Wallet, candidate.Chain, candidate.ID, candidate.Atomic, candidate.Decimals, candidate.Protocol)] = true
		}
	}
	knownSnapshots := make(map[string]bool)
	for _, snapshot := range snapshots {
		if reference, ok := ReferenceSnapshot(snapshot); ok {
			knownSnapshots[snapshotKey(reference)] = true
		}
	}
	verifiedByCandidate := make(map[string][]int)
	for index := range verification.Positions {
		position := &verification.Positions[index]
		candidateKnown := knownCandidates[candidateKey(position.Wallet, position.Chain, position.InstrumentID, position.Atomic, position.Decimals, position.Protocol)]
		validFields := position.Class.Valid() && position.AccountingRole.Valid() && position.Source != "" && position.Source == strings.TrimSpace(position.Source) &&
			position.ValuationUnitID != "" && position.ValuationUnitID == strings.TrimSpace(position.ValuationUnitID) &&
			position.ExposureID != "" && position.ExposureID == strings.TrimSpace(position.ExposureID)
		atomic, validAtomic := new(big.Int).SetString(position.Atomic, 10)
		validFields = validFields && validAtomic && atomic.Sign() >= 0 && atomic.String() == position.Atomic && position.Decimals >= 0 && position.Decimals <= 255
		if !candidateKnown || !validFields {
			position.Status = InvalidResponse
			verification.Failures = append(verification.Failures, Failure{Scope: "verification/" + position.InstrumentID, Status: InvalidResponse})
			verification.CoverageComplete = false
			continue
		}
		if position.Chain != position.Snapshot.Chain || !knownSnapshots[snapshotKey(position.Snapshot)] {
			position.Status = InconsistentSnapshot
			verification.Failures = append(verification.Failures, Failure{Scope: "snapshot/" + position.InstrumentID, Status: InconsistentSnapshot})
			verification.CoverageComplete = false
			continue
		}
		position.Status = OK
		key := candidateKey(position.Wallet, position.Chain, position.InstrumentID, position.Atomic, position.Decimals, position.Protocol)
		verifiedByCandidate[key] = append(verifiedByCandidate[key], index)
	}
	for _, indexes := range verifiedByCandidate {
		if len(indexes) < 2 {
			continue
		}
		for _, index := range indexes {
			verification.Positions[index].Status = InvalidResponse
		}
		verification.Failures = append(verification.Failures, Failure{Scope: "verification/duplicate/" + verification.Positions[indexes[0]].InstrumentID, Status: InvalidResponse})
		verification.CoverageComplete = false
	}
	verification = ReconcileVerificationSnapshots(verification)
	if len(verification.Failures) > 0 && (verification.Status == "" || verification.Status == OK) {
		verification.Status = InvalidResponse
	} else if verification.Status == "" {
		verification.Status = OK
	}
	// Provider success does not prove complete protocol or portfolio coverage.
	verification.CoverageComplete = false
	return verification
}

func candidateKey(wallet Address, chain, id, atomic string, decimals int, protocol string) string {
	return strings.Join([]string{string(wallet), chain, id, atomic, strconv.Itoa(decimals), protocol}, "\x00")
}

func snapshotKey(snapshot SnapshotReference) string {
	return strings.Join([]string{snapshot.Chain, strconv.FormatUint(snapshot.ChainID, 10), snapshot.BlockNumber, snapshot.BlockHash, snapshot.BlockTime.UTC().Format(time.RFC3339Nano)}, "\x00")
}

// ConfirmPosition enforces the boundary between provider evidence and a
// protocol-confirmed position. It deliberately does not infer class, source,
// or snapshot from discovery metadata.
func ConfirmPosition(candidate NormalizedCandidate, class PositionClass, source, valuationUnitID, exposureID string, role AccountingRole, snapshot SnapshotReference) (VerifiedPosition, bool) {
	source = strings.TrimSpace(source)
	valuationUnitID = strings.TrimSpace(valuationUnitID)
	exposureID = strings.TrimSpace(exposureID)
	atomic, validAtomic := new(big.Int).SetString(candidate.Atomic, 10)
	if candidate.Status != OK || candidate.Wallet == "" || candidate.ID == "" || candidate.Chain == "" ||
		candidate.Decimals < 0 || candidate.Decimals > 255 || !validAtomic || atomic.Sign() < 0 || atomic.String() != candidate.Atomic ||
		!class.Valid() || !role.Valid() || source == "" || valuationUnitID == "" || exposureID == "" || !snapshot.valid() || candidate.Chain != snapshot.Chain {
		return VerifiedPosition{}, false
	}
	return VerifiedPosition{
		Wallet: candidate.Wallet, Chain: candidate.Chain, Protocol: candidate.Protocol,
		InstrumentID: candidate.ID, ValuationUnitID: valuationUnitID, ExposureID: exposureID, AccountingRole: role, Class: class, Atomic: candidate.Atomic,
		Decimals: candidate.Decimals, Source: source, Snapshot: snapshot, Status: OK,
	}, true
}

func (role AccountingRole) Valid() bool { return role == EconomicExposure || role == Representation }

func (snapshot SnapshotReference) valid() bool {
	return snapshot.Chain != "" && snapshot.Chain == strings.ToLower(strings.TrimSpace(snapshot.Chain)) && snapshot.ChainID != 0 && strings.TrimSpace(snapshot.BlockNumber) != "" && strings.TrimSpace(snapshot.BlockHash) != "" &&
		!snapshot.BlockTime.IsZero() && snapshot.Finality == UnknownFinality
}

// ReconcileVerificationSnapshots marks every position on a chain uncertain if
// a verifier mixes block identities. It preserves the evidence for diagnosis
// but prevents consumers from treating those positions as confirmed.
func ReconcileVerificationSnapshots(verification Verification) Verification {
	type observed struct {
		reference SnapshotReference
		indexes   []int
	}
	byChain := make(map[string]observed)
	failed := make(map[string]bool)
	for index := range verification.Positions {
		position := &verification.Positions[index]
		chainKey := strconv.FormatUint(position.Snapshot.ChainID, 10)
		if position.Status == "" {
			position.Status = OK
		}
		if position.Status != OK {
			continue
		}
		if !position.Snapshot.valid() {
			position.Status = InconsistentSnapshot
			appendSnapshotFailure(&verification, failed, chainKey)
			continue
		}
		if failed[chainKey] {
			position.Status = InconsistentSnapshot
			continue
		}
		seen, found := byChain[chainKey]
		if !found {
			byChain[chainKey] = observed{reference: position.Snapshot, indexes: []int{index}}
			continue
		}
		seen.indexes = append(seen.indexes, index)
		byChain[chainKey] = seen
		if sameSnapshot(seen.reference, position.Snapshot) {
			continue
		}
		for _, conflicting := range seen.indexes {
			verification.Positions[conflicting].Status = InconsistentSnapshot
		}
		appendSnapshotFailure(&verification, failed, chainKey)
	}
	if len(failed) > 0 {
		verification.Status = InconsistentSnapshot
		verification.CoverageComplete = false
	}
	return verification
}

func sameSnapshot(a, b SnapshotReference) bool {
	return a.Chain == b.Chain && a.ChainID == b.ChainID && a.BlockNumber == b.BlockNumber && a.BlockHash == b.BlockHash && a.BlockTime.Equal(b.BlockTime)
}

func appendSnapshotFailure(verification *Verification, failed map[string]bool, chain string) {
	if failed[chain] {
		return
	}
	verification.Failures = append(verification.Failures, Failure{Scope: "snapshot/" + chain, Status: InconsistentSnapshot})
	failed[chain] = true
}
