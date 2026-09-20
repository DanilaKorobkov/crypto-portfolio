package domain

import (
	"math/big"
	"strings"
	"time"
)

type Rational struct {
	Numerator   string `json:"numerator"`
	Denominator string `json:"denominator"`
}

func NewRational(numerator, denominator *big.Int) (Rational, bool) {
	if numerator == nil || denominator == nil || denominator.Sign() == 0 {
		return Rational{}, false
	}
	value := new(big.Rat).SetFrac(new(big.Int).Set(numerator), new(big.Int).Set(denominator))
	if value.Denom().Sign() < 0 {
		value.Neg(value)
	}
	return Rational{Numerator: value.Num().String(), Denominator: value.Denom().String()}, true
}

func ParseRational(value Rational) (*big.Rat, bool) {
	numerator, denominator := new(big.Int), new(big.Int)
	if _, ok := numerator.SetString(value.Numerator, 10); !ok {
		return nil, false
	}
	if _, ok := denominator.SetString(value.Denominator, 10); !ok || denominator.Sign() <= 0 {
		return nil, false
	}
	canonical, ok := NewRational(numerator, denominator)
	if !ok || canonical != value {
		return nil, false
	}
	return new(big.Rat).SetFrac(numerator, denominator), true
}

type RiskAmount struct {
	InstrumentID  string          `json:"instrument_id"`
	UnitID        string          `json:"unit_id"`
	Atomic        string          `json:"atomic"`
	Decimals      int             `json:"decimals"`
	OraclePrice   OracleQuote     `json:"oracle_price"`
	PriceRelation PriceDependency `json:"price_relation"`
}

type RiskModel string
type PriceDependency string

const (
	LinearWeightedCollateral RiskModel       = "linear_weighted_collateral"
	IndependentPrice         PriceDependency = "independent"
	LinkedPrice              PriceDependency = "linked"
	UnknownPriceDependency   PriceDependency = "unknown"
)

type OracleQuote struct {
	UnitID   string            `json:"unit_id"`
	USD      *Decimal          `json:"usd"`
	Source   string            `json:"source"`
	AsOf     time.Time         `json:"as_of"`
	Snapshot SnapshotReference `json:"snapshot"`
	Status   Status            `json:"status"`
}

type CollateralRiskInput struct {
	Amount               RiskAmount `json:"amount"`
	LiquidationThreshold *Decimal   `json:"liquidation_threshold"`
}

type RiskUnitInput struct {
	RiskUnitID string                `json:"risk_unit_id"`
	Wallet     Address               `json:"wallet"`
	Protocol   string                `json:"protocol"`
	MarketID   string                `json:"market_id"`
	Model      RiskModel             `json:"model"`
	Snapshot   SnapshotReference     `json:"snapshot"`
	Collateral []CollateralRiskInput `json:"collateral"`
	Debts      []RiskAmount          `json:"debts"`
	NativeHF   *Rational             `json:"native_hf"`
	Source     string                `json:"source"`
}

type DebtRisk struct {
	RiskUnitID                  string            `json:"risk_unit_id"`
	Wallet                      Address           `json:"wallet"`
	Protocol                    string            `json:"protocol"`
	Snapshot                    SnapshotReference `json:"snapshot"`
	CollateralValueUSD          *Decimal          `json:"collateral_value_usd"`
	DebtValueUSD                *Decimal          `json:"debt_value_usd"`
	NativeHF                    *Rational         `json:"native_hf"`
	DerivedHF                   *Rational         `json:"derived_hf"`
	EffectiveHF                 *Rational         `json:"effective_hf"`
	EffectiveHFSource           string            `json:"effective_hf_source,omitempty"`
	HFComparison                string            `json:"hf_comparison"`
	RedZone                     *bool             `json:"red_zone"`
	LiquidationPriceUSD         *Rational         `json:"liquidation_price_usd"`
	DropToLiquidationPercent    *Rational         `json:"drop_to_liquidation_percent"`
	LiquidationScenarioEligible bool              `json:"liquidation_scenario_eligible"`
	ScenarioStatus              Status            `json:"scenario_status"`
	Status                      Status            `json:"status"`
	Failures                    []Failure         `json:"failures"`
}

type RiskAssessment struct {
	Status           Status     `json:"status"`
	Units            []DebtRisk `json:"units"`
	Failures         []Failure  `json:"failures"`
	CoverageComplete bool       `json:"coverage_complete"`
}

func AssessDebtRisk(inputs []RiskUnitInput, positions []VerifiedPosition) RiskAssessment {
	result := RiskAssessment{Status: OK}
	unitCounts := make(map[string]int)
	for _, input := range inputs {
		unitCounts[strings.TrimSpace(input.RiskUnitID)]++
	}
	known := make(map[string]VerifiedPosition)
	for _, position := range positions {
		if position.Status == OK {
			known[riskPositionKey(position.Wallet, position.Protocol, position.InstrumentID)] = position
		}
	}
	for _, input := range inputs {
		if unitCounts[strings.TrimSpace(input.RiskUnitID)] > 1 {
			unit := DebtRisk{RiskUnitID: strings.TrimSpace(input.RiskUnitID), Wallet: input.Wallet, Protocol: strings.TrimSpace(input.Protocol),
				Snapshot: input.Snapshot, Status: InvalidRiskModel, HFComparison: "not_available", ScenarioStatus: ScenarioUnavailable,
				Failures: []Failure{{Scope: "risk/" + strings.TrimSpace(input.RiskUnitID) + "/duplicate", Status: InvalidRiskModel}}}
			result.Units = append(result.Units, unit)
			result.Failures = append(result.Failures, unit.Failures...)
			continue
		}
		unit := assessRiskUnit(input, known)
		result.Units = append(result.Units, unit)
		result.Failures = append(result.Failures, unit.Failures...)
	}
	if len(result.Failures) > 0 {
		result.Status = InvalidRiskModel
	}
	result.CoverageComplete = false
	return result
}

func assessRiskUnit(input RiskUnitInput, known map[string]VerifiedPosition) DebtRisk {
	result := DebtRisk{RiskUnitID: strings.TrimSpace(input.RiskUnitID), Wallet: input.Wallet, Protocol: strings.TrimSpace(input.Protocol), Snapshot: input.Snapshot, Status: OK,
		HFComparison: "not_available", ScenarioStatus: ScenarioUnavailable}
	fail := func(scope string, status Status) {
		result.Failures = append(result.Failures, Failure{Scope: "risk/" + result.RiskUnitID + "/" + scope, Status: status})
		result.Status = InvalidRiskModel
	}
	if result.RiskUnitID == "" || result.Wallet == "" || result.Protocol == "" || strings.TrimSpace(input.MarketID) == "" || input.Model != LinearWeightedCollateral ||
		strings.TrimSpace(input.Source) == "" || !input.Snapshot.valid() || len(input.Collateral) == 0 {
		fail("identity", InvalidRiskModel)
		return result
	}
	legs := make(map[string]bool)
	for _, collateral := range input.Collateral {
		if legs[collateral.Amount.InstrumentID] {
			fail("duplicate_leg/"+collateral.Amount.InstrumentID, InvalidRiskModel)
			return result
		}
		legs[collateral.Amount.InstrumentID] = true
	}
	for _, debt := range input.Debts {
		if legs[debt.InstrumentID] {
			fail("duplicate_leg/"+debt.InstrumentID, InvalidRiskModel)
			return result
		}
		legs[debt.InstrumentID] = true
	}

	var collateralTotal *Decimal
	weightedCollateral := new(big.Rat)
	allCollateral := true
	for _, collateral := range input.Collateral {
		position, ok := known[riskPositionKey(input.Wallet, input.Protocol, collateral.Amount.InstrumentID)]
		if !ok || position.Class == DebtPosition || !riskAmountMatches(collateral.Amount, position, input.Snapshot) || collateral.LiquidationThreshold == nil || !validThreshold(*collateral.LiquidationThreshold) {
			fail("collateral/"+collateral.Amount.InstrumentID, InvalidRiskModel)
			allCollateral = false
			continue
		}
		value, ok := valueRiskAmount(collateral.Amount, input.Snapshot)
		if !ok {
			fail("collateral_price/"+collateral.Amount.InstrumentID, MissingPrice)
			allCollateral = false
			continue
		}
		collateralTotal = addOptionalDecimal(collateralTotal, value)
		weightedCollateral.Add(weightedCollateral, new(big.Rat).Mul(decimalRat(*value), decimalRat(*collateral.LiquidationThreshold)))
	}

	var debtTotal *Decimal
	allDebt := true
	for _, debt := range input.Debts {
		position, ok := known[riskPositionKey(input.Wallet, input.Protocol, debt.InstrumentID)]
		if !ok || position.Class != DebtPosition || !riskAmountMatches(debt, position, input.Snapshot) {
			fail("debt/"+debt.InstrumentID, InvalidRiskModel)
			allDebt = false
			continue
		}
		value, ok := valueRiskAmount(debt, input.Snapshot)
		if !ok {
			fail("debt_price/"+debt.InstrumentID, MissingPrice)
			allDebt = false
			continue
		}
		debtTotal = addOptionalDecimal(debtTotal, value)
	}
	result.CollateralValueUSD, result.DebtValueUSD = collateralTotal, debtTotal

	if len(input.Debts) == 0 || allDebt && debtTotal != nil && decimalRat(*debtTotal).Sign() == 0 {
		result.Status = NoDebt
		result.ScenarioStatus = NoDebt
		return result
	}
	if input.NativeHF != nil {
		if native, ok := ParseRational(*input.NativeHF); ok && native.Sign() >= 0 {
			copy := *input.NativeHF
			result.NativeHF = &copy
		} else {
			fail("native_hf", InvalidRiskModel)
		}
	}
	if allCollateral && allDebt {
		derived := new(big.Rat).Quo(weightedCollateral, decimalRat(*debtTotal))
		value, _ := NewRational(derived.Num(), derived.Denom())
		result.DerivedHF = &value
	}
	if result.NativeHF != nil {
		copy := *result.NativeHF
		result.EffectiveHF, result.EffectiveHFSource = &copy, "native"
	} else if result.DerivedHF != nil {
		copy := *result.DerivedHF
		result.EffectiveHF, result.EffectiveHFSource = &copy, "derived"
	}
	if result.NativeHF != nil && result.DerivedHF != nil {
		native, _ := ParseRational(*result.NativeHF)
		derived, _ := ParseRational(*result.DerivedHF)
		result.HFComparison = "different"
		if native.Cmp(derived) == 0 {
			result.HFComparison = "equal"
		}
	}
	if result.EffectiveHF != nil {
		hf, _ := ParseRational(*result.EffectiveHF)
		red := hf.Cmp(big.NewRat(14, 10)) < 0
		result.RedZone = &red
	}

	independentDebt := true
	for _, debt := range input.Debts {
		if debt.PriceRelation != IndependentPrice {
			independentDebt = false
		}
	}
	if len(input.Collateral) == 1 && allCollateral && allDebt && independentDebt {
		collateral := input.Collateral[0]
		quantity := atomicRat(collateral.Amount.Atomic, collateral.Amount.Decimals)
		threshold := decimalRat(*collateral.LiquidationThreshold)
		denominator := new(big.Rat).Mul(quantity, threshold)
		if denominator.Sign() > 0 {
			liquidation := new(big.Rat).Quo(decimalRat(*debtTotal), denominator)
			value, _ := NewRational(liquidation.Num(), liquidation.Denom())
			result.LiquidationPriceUSD = &value
			result.LiquidationScenarioEligible = true
			result.ScenarioStatus = OK
			current := decimalRat(*collateral.Amount.OraclePrice.USD)
			hf := new(big.Rat).Quo(weightedCollateral, decimalRat(*debtTotal))
			if hf.Cmp(big.NewRat(1, 1)) <= 0 {
				result.ScenarioStatus = ThresholdReached
			} else if current.Sign() > 0 {
				drop := new(big.Rat).Mul(big.NewRat(100, 1), new(big.Rat).Sub(big.NewRat(1, 1), new(big.Rat).Quo(liquidation, current)))
				if drop.Sign() >= 0 {
					percentage, _ := NewRational(drop.Num(), drop.Denom())
					result.DropToLiquidationPercent = &percentage
				}
			}
		}
	}
	return result
}

func riskPositionKey(wallet Address, protocol, instrumentID string) string {
	return strings.Join([]string{string(wallet), strings.TrimSpace(protocol), strings.TrimSpace(instrumentID)}, "\x00")
}

func riskAmountMatches(amount RiskAmount, position VerifiedPosition, snapshot SnapshotReference) bool {
	return amount.UnitID == position.ValuationUnitID && amount.Atomic == position.Atomic && amount.Decimals == position.Decimals && sameSnapshot(snapshot, position.Snapshot)
}

func validThreshold(value Decimal) bool {
	if int(value.scale) > maxInputDecimalScale {
		return false
	}
	ratio := decimalRat(value)
	return ratio.Sign() > 0 && ratio.Cmp(big.NewRat(1, 1)) <= 0
}

func valueRiskAmount(amount RiskAmount, snapshot SnapshotReference) (*Decimal, bool) {
	quote := amount.OraclePrice
	if quote.Status != OK || quote.USD == nil || quote.USD.coefficient.Sign() <= 0 || int(quote.USD.scale) > maxInputDecimalScale ||
		strings.TrimSpace(quote.UnitID) != amount.UnitID || strings.TrimSpace(quote.Source) == "" || quote.AsOf.IsZero() || !sameSnapshot(quote.Snapshot, snapshot) {
		return nil, false
	}
	value, ok := multiplyAtomic(amount.Atomic, amount.Decimals, *quote.USD)
	if !ok {
		return nil, false
	}
	return &value, true
}

func decimalRat(value Decimal) *big.Rat {
	denominator := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(value.scale)), nil)
	return new(big.Rat).SetFrac(new(big.Int).Set(&value.coefficient), denominator)
}

func atomicRat(atomic string, decimals int) *big.Rat {
	numerator, ok := new(big.Int).SetString(atomic, 10)
	if !ok || decimals < 0 || decimals > maxTokenDecimals {
		return new(big.Rat)
	}
	denominator := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)
	return new(big.Rat).SetFrac(numerator, denominator)
}
