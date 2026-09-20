package domain

import "testing"

import "time"

func TestConfirmPositionRequiresExplicitVerificationEvidence(t *testing.T) {
	candidate := NormalizedCandidate{Wallet: "wallet", ID: "token", Chain: "ethereum", Protocol: "aave-v3", Atomic: "12345678901234567890", Decimals: 18, Status: OK}
	snapshot := SnapshotReference{Chain: "ethereum", ChainID: 1, BlockNumber: "0x123", BlockHash: "0xabc", BlockTime: time.Unix(1, 0).UTC(), Finality: UnknownFinality}
	position, ok := ConfirmPosition(candidate, DebtPosition, "aave-v3-pool", "usdc", "loan-1", EconomicExposure, snapshot)
	if !ok || position.Atomic != candidate.Atomic || position.Class != DebtPosition || position.Snapshot.BlockNumber != "0x123" {
		t.Fatal(position, ok)
	}
	for _, test := range []struct {
		name      string
		candidate NormalizedCandidate
		class     PositionClass
		source    string
		unit      string
		snapshot  SnapshotReference
	}{
		{name: "unverified candidate", candidate: func() NormalizedCandidate { value := candidate; value.Status = InconsistentPagination; return value }(), class: DebtPosition, source: "pool", unit: "usdc", snapshot: snapshot},
		{name: "unknown class", candidate: candidate, class: "unknown", source: "pool", unit: "usdc", snapshot: snapshot},
		{name: "missing source", candidate: candidate, class: DebtPosition, unit: "usdc", snapshot: snapshot},
		{name: "missing unit", candidate: candidate, class: DebtPosition, source: "pool", snapshot: snapshot},
		{name: "missing snapshot", candidate: candidate, class: DebtPosition, source: "pool", unit: "usdc"},
		{name: "noncanonical quantity", candidate: func() NormalizedCandidate { value := candidate; value.Atomic = "001"; return value }(), class: DebtPosition, source: "pool", unit: "usdc", snapshot: snapshot},
		{name: "negative quantity", candidate: func() NormalizedCandidate { value := candidate; value.Atomic = "-1"; return value }(), class: DebtPosition, source: "pool", unit: "usdc", snapshot: snapshot},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, ok := ConfirmPosition(test.candidate, test.class, test.source, test.unit, "exposure", EconomicExposure, test.snapshot); ok {
				t.Fatal("accepted position without complete verification evidence")
			}
		})
	}
}

func TestReferenceSnapshotPreservesIdentityWithoutClaimingFinality(t *testing.T) {
	now := time.Unix(123, 456).UTC()
	reference, ok := ReferenceSnapshot(ChainSnapshot{Chain: "base", ChainID: 8453, Status: OK, BlockNumber: "0x10", BlockHash: "0xabc", BlockTime: now})
	if !ok || reference.ChainID != 8453 || reference.BlockHash != "0xabc" || !reference.BlockTime.Equal(now) || reference.Finality != UnknownFinality {
		t.Fatal(reference, ok)
	}
	if _, ok := ReferenceSnapshot(ChainSnapshot{Chain: "base", ChainID: 8453, Status: InconsistentSnapshot, BlockNumber: "0x10", BlockHash: "0xabc", BlockTime: now}); ok {
		t.Fatal("accepted inconsistent snapshot")
	}
}

func TestReconcileVerificationSnapshotsRejectsMixedBlocksPerChain(t *testing.T) {
	base := SnapshotReference{Chain: "ethereum", ChainID: 1, BlockNumber: "0x10", BlockHash: "0xaaa", BlockTime: time.Unix(10, 0).UTC(), Finality: UnknownFinality}
	other := base
	other.BlockHash = "0xbbb"
	verification := Verification{Status: OK, CoverageComplete: true, Positions: []VerifiedPosition{
		{Chain: "ethereum", InstrumentID: "one", Snapshot: base, Status: OK},
		{Chain: "ethereum", InstrumentID: "two", Snapshot: other, Status: OK},
		{Chain: "ethereum", InstrumentID: "one-again", Snapshot: base, Status: OK},
		{Chain: "base", InstrumentID: "three", Snapshot: SnapshotReference{Chain: "base", ChainID: 8453, BlockNumber: "0x20", BlockHash: "0xccc", BlockTime: time.Unix(20, 0).UTC(), Finality: UnknownFinality}, Status: OK},
	}}
	got := ReconcileVerificationSnapshots(verification)
	if got.Status != InconsistentSnapshot || got.CoverageComplete || len(got.Failures) != 1 {
		t.Fatal(got)
	}
	if got.Positions[0].Status != InconsistentSnapshot || got.Positions[1].Status != InconsistentSnapshot || got.Positions[2].Status != InconsistentSnapshot || got.Positions[3].Status != OK {
		t.Fatal(got.Positions)
	}
}

func TestValidateVerificationRejectsFabricatedPositionAndSnapshot(t *testing.T) {
	now := time.Unix(10, 0).UTC()
	collected := ChainSnapshot{Chain: "ethereum", ChainID: 1, Status: OK, BlockNumber: "0x10", BlockHash: "0xaaa", BlockTime: now}
	reference, _ := ReferenceSnapshot(collected)
	candidate := NormalizedCandidate{Wallet: "wallet", Chain: "ethereum", ID: "token", Protocol: "aave", Atomic: "10", Decimals: 6, Status: OK}
	valid, _ := ConfirmPosition(candidate, AssetPosition, "pool", "token-unit", "asset-1", EconomicExposure, reference)
	fabricated := valid
	fabricated.InstrumentID = "other"
	wrongSnapshot := valid
	wrongSnapshot.Snapshot.BlockHash = "0xbbb"
	got := ValidateVerification(Verification{Status: OK, CoverageComplete: true, Positions: []VerifiedPosition{valid, fabricated, wrongSnapshot}}, []NormalizedCandidate{candidate}, []ChainSnapshot{collected})
	if got.CoverageComplete || got.Positions[0].Status != OK || got.Positions[1].Status != InvalidResponse || got.Positions[2].Status != InconsistentSnapshot || len(got.Failures) != 2 {
		t.Fatal(got)
	}
}

func TestValidateVerificationRejectsDuplicatePromotionOfCandidate(t *testing.T) {
	now := time.Unix(10, 0).UTC()
	collected := ChainSnapshot{Chain: "ethereum", ChainID: 1, Status: OK, BlockNumber: "0x10", BlockHash: "0xaaa", BlockTime: now}
	reference, _ := ReferenceSnapshot(collected)
	candidate := NormalizedCandidate{Wallet: "wallet", Chain: "ethereum", ID: "token", Protocol: "aave", Atomic: "10", Decimals: 6, Status: OK}
	first, _ := ConfirmPosition(candidate, AssetPosition, "pool", "token-unit", "first", EconomicExposure, reference)
	second, _ := ConfirmPosition(candidate, AssetPosition, "pool", "token-unit", "second", EconomicExposure, reference)
	got := ValidateVerification(Verification{Status: OK, Positions: []VerifiedPosition{first, second}}, []NormalizedCandidate{candidate}, []ChainSnapshot{collected})
	if got.Positions[0].Status != InvalidResponse || got.Positions[1].Status != InvalidResponse || len(got.Failures) != 1 {
		t.Fatal(got)
	}
}

func TestValidateVerificationRejectsWrongChainBinding(t *testing.T) {
	now := time.Unix(10, 0).UTC()
	collected := ChainSnapshot{Chain: "ethereum", ChainID: 1, Status: OK, BlockNumber: "0x10", BlockHash: "0xaaa", BlockTime: now}
	reference, _ := ReferenceSnapshot(collected)
	candidate := NormalizedCandidate{Wallet: "wallet", Chain: "polygon", ID: "token", Protocol: "aave", Atomic: "10", Decimals: 6, Status: OK}
	if _, ok := ConfirmPosition(candidate, AssetPosition, "pool", "token-unit", "asset", EconomicExposure, reference); ok {
		t.Fatal("accepted snapshot from another chain")
	}
}
