package domain

import (
	"strconv"
	"strings"
)

type AggregateLine struct {
	ValuationUnitID string        `json:"valuation_unit_id"`
	Class           PositionClass `json:"class"`
	AvailableUSD    *Decimal      `json:"available_usd"`
	PositionCount   int           `json:"position_count"`
	Status          Status        `json:"status"`
}

type ExcludedExposure struct {
	Wallet       Address `json:"wallet"`
	Chain        string  `json:"chain"`
	InstrumentID string  `json:"instrument_id"`
	ExposureID   string  `json:"exposure_id"`
	Reason       Status  `json:"reason"`
}

type Aggregation struct {
	Status                  Status             `json:"status"`
	Lines                   []AggregateLine    `json:"lines"`
	Excluded                []ExcludedExposure `json:"excluded"`
	AvailableGrossAssetsUSD *Decimal           `json:"available_gross_assets_usd"`
	AvailableGrossDebtUSD   *Decimal           `json:"available_gross_debt_usd"`
	Failures                []Failure          `json:"failures"`
	AssetCoverage           CoverageCounts     `json:"asset_coverage"`
	DebtCoverage            CoverageCounts     `json:"debt_coverage"`
	AssetCoverageComplete   bool               `json:"asset_coverage_complete"`
	DebtCoverageComplete    bool               `json:"debt_coverage_complete"`
}

type CoverageCounts struct {
	EconomicPositions int `json:"economic_positions"`
	ValuedPositions   int `json:"valued_positions"`
	UnpricedPositions int `json:"unpriced_positions"`
	ExcludedPositions int `json:"excluded_positions"`
}

// AggregatePortfolio totals only independently counted economic exposures.
// Representation positions remain visible but never add value. Duplicate
// economic owners of one exposure are all excluded rather than guessed.
func AggregatePortfolio(positions []VerifiedPosition, valuation Valuation) Aggregation {
	result := Aggregation{Status: OK}
	values := make(map[string]PositionValuation)
	valueConflicts := make(map[string]bool)
	for _, item := range valuation.Items {
		key := valuationIdentityKey(item)
		if _, exists := values[key]; exists {
			delete(values, key)
			valueConflicts[key] = true
			result.Failures = append(result.Failures, Failure{Scope: "aggregation/valuation/" + item.InstrumentID, Status: InvalidResponse})
			continue
		}
		if !valueConflicts[key] {
			values[key] = item
		}
	}

	exposureIndexes := make(map[string][]int)
	for index, position := range positions {
		if position.Status == OK && position.AccountingRole == EconomicExposure {
			key := exposureKey(position)
			exposureIndexes[key] = append(exposureIndexes[key], index)
		}
	}
	conflicted := make(map[int]bool)
	for exposureID, indexes := range exposureIndexes {
		if len(indexes) < 2 {
			continue
		}
		result.Failures = append(result.Failures, Failure{Scope: "exposure/" + exposureID, Status: DoubleCountRisk})
		for _, index := range indexes {
			conflicted[index] = true
		}
	}

	type lineKey struct {
		unit  string
		class PositionClass
	}
	lineIndexes := make(map[lineKey]int)
	assetItems := make([]PositionValuation, 0)
	debtItems := make([]PositionValuation, 0)
	for index, position := range positions {
		coverage := &result.AssetCoverage
		if position.Class == DebtPosition {
			coverage = &result.DebtCoverage
		}
		if position.AccountingRole == EconomicExposure {
			coverage.EconomicPositions++
		}
		if position.Status != OK || !position.Class.Valid() || !position.AccountingRole.Valid() {
			result.Excluded = append(result.Excluded, ExcludedExposure{Wallet: position.Wallet, Chain: position.Chain, InstrumentID: position.InstrumentID, ExposureID: position.ExposureID, Reason: position.Status})
			coverage.ExcludedPositions++
			if position.Status == "" {
				result.Excluded[len(result.Excluded)-1].Reason = InvalidResponse
			}
			result.Failures = append(result.Failures, Failure{Scope: "aggregation/position/" + position.InstrumentID, Status: InvalidResponse})
			continue
		}
		if position.AccountingRole == Representation || conflicted[index] {
			reason := Represented
			if conflicted[index] {
				reason = DoubleCountRisk
			}
			result.Excluded = append(result.Excluded, ExcludedExposure{Wallet: position.Wallet, Chain: position.Chain, InstrumentID: position.InstrumentID, ExposureID: position.ExposureID, Reason: reason})
			coverage.ExcludedPositions++
			continue
		}
		identityKey := positionIdentityKey(position)
		item, found := values[identityKey]
		if valueConflicts[identityKey] {
			result.Excluded = append(result.Excluded, ExcludedExposure{Wallet: position.Wallet, Chain: position.Chain, InstrumentID: position.InstrumentID, ExposureID: position.ExposureID, Reason: InvalidResponse})
			coverage.ExcludedPositions++
			continue
		}
		if !found || item.Status != OK || item.Value == nil {
			result.Failures = append(result.Failures, Failure{Scope: "aggregation/" + position.InstrumentID, Status: MissingPrice})
			coverage.UnpricedPositions++
			continue
		}
		coverage.ValuedPositions++
		groupingKey := lineKey{unit: position.ValuationUnitID, class: position.Class}
		lineIndex, exists := lineIndexes[groupingKey]
		if !exists {
			lineIndex = len(result.Lines)
			lineIndexes[groupingKey] = lineIndex
			result.Lines = append(result.Lines, AggregateLine{ValuationUnitID: position.ValuationUnitID, Class: position.Class, Status: OK})
		}
		line := &result.Lines[lineIndex]
		line.PositionCount++
		line.AvailableUSD = addOptionalDecimal(line.AvailableUSD, item.Value)
		if position.Class == DebtPosition {
			debtItems = append(debtItems, item)
		} else {
			assetItems = append(assetItems, item)
		}
	}
	result.AvailableGrossAssetsUSD = SumAvailableValues(assetItems)
	result.AvailableGrossDebtUSD = SumAvailableValues(debtItems)
	if len(result.Failures) > 0 {
		result.Status = InvalidResponse
	}
	// Discovery and price-source coverage are not proven by aggregation.
	result.AssetCoverageComplete = false
	result.DebtCoverageComplete = false
	return result
}

func positionIdentityKey(position VerifiedPosition) string {
	return strings.Join([]string{string(position.Wallet), position.Chain, position.Protocol, position.InstrumentID, position.ExposureID,
		string(position.Class), string(position.AccountingRole), position.Atomic, strconv.Itoa(position.Decimals), snapshotKey(position.Snapshot)}, "\x00")
}

func valuationIdentityKey(item PositionValuation) string {
	return strings.Join([]string{string(item.Wallet), item.Chain, item.Protocol, item.InstrumentID, item.ExposureID,
		string(item.Class), string(item.AccountingRole), item.Atomic, strconv.Itoa(item.Decimals), snapshotKey(item.Snapshot)}, "\x00")
}

func exposureKey(position VerifiedPosition) string {
	return strings.Join([]string{string(position.Wallet), position.Snapshot.Chain, strconv.FormatUint(position.Snapshot.ChainID, 10), position.Protocol, position.ExposureID}, "\x00")
}

func addOptionalDecimal(left, right *Decimal) *Decimal {
	if left == nil {
		value := *right
		return &value
	}
	return addDecimals(*left, *right)
}
