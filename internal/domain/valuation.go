package domain

import (
	"encoding/json"
	"errors"
	"math/big"
	"strings"
	"time"
)

const (
	maxInputDecimalScale = 255
	maxDecimalScale      = 510
	maxTokenDecimals     = 255
)

// Decimal is an exact base-10 value. Its zero value is numeric zero; absence
// must be represented by a nil *Decimal, never by a zero Decimal.
type Decimal struct {
	coefficient big.Int
	scale       uint16
}

func ParseDecimal(value string) (Decimal, error) {
	if value == "" || strings.TrimSpace(value) != value || strings.HasPrefix(value, "+") {
		return Decimal{}, errors.New("invalid decimal")
	}
	negative := strings.HasPrefix(value, "-")
	digits := value
	if negative {
		digits = strings.TrimPrefix(value, "-")
	}
	parts := strings.Split(digits, ".")
	if len(parts) > 2 || parts[0] == "" || len(parts) == 2 && parts[1] == "" || len(parts) == 2 && len(parts[1]) > maxDecimalScale {
		return Decimal{}, errors.New("invalid decimal")
	}
	joined := strings.Join(parts, "")
	for _, character := range joined {
		if character < '0' || character > '9' {
			return Decimal{}, errors.New("invalid decimal")
		}
	}
	coefficient := new(big.Int)
	if _, ok := coefficient.SetString(joined, 10); !ok {
		return Decimal{}, errors.New("invalid decimal")
	}
	if negative {
		coefficient.Neg(coefficient)
	}
	scale := 0
	if len(parts) == 2 {
		scale = len(parts[1])
	}
	return normalizeDecimal(coefficient, scale), nil
}

func normalizeDecimal(coefficient *big.Int, scale int) Decimal {
	coefficient = new(big.Int).Set(coefficient)
	ten := big.NewInt(10)
	zero := new(big.Int)
	for scale > 0 {
		quotient, remainder := new(big.Int), new(big.Int)
		quotient.QuoRem(coefficient, ten, remainder)
		if remainder.Cmp(zero) != 0 {
			break
		}
		coefficient = quotient
		scale--
	}
	return Decimal{coefficient: *coefficient, scale: uint16(scale)}
}

func (decimal Decimal) String() string {
	digits := decimal.coefficient.String()
	negative := strings.HasPrefix(digits, "-")
	if negative {
		digits = strings.TrimPrefix(digits, "-")
	}
	if decimal.scale > 0 {
		for len(digits) <= int(decimal.scale) {
			digits = "0" + digits
		}
		split := len(digits) - int(decimal.scale)
		digits = digits[:split] + "." + digits[split:]
	}
	if negative && decimal.coefficient.Sign() != 0 {
		return "-" + digits
	}
	return digits
}

func (decimal Decimal) MarshalJSON() ([]byte, error) { return json.Marshal(decimal.String()) }

func (decimal *Decimal) UnmarshalJSON(raw []byte) error {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return errors.New("decimal must be a string")
	}
	parsed, err := ParseDecimal(value)
	if err != nil {
		return err
	}
	*decimal = parsed
	return nil
}

func multiplyAtomic(atomic string, decimals int, price Decimal) (Decimal, bool) {
	quantity := new(big.Int)
	if _, ok := quantity.SetString(atomic, 10); !ok || decimals < 0 || decimals > maxTokenDecimals || int(price.scale) > maxInputDecimalScale {
		return Decimal{}, false
	}
	coefficient := new(big.Int).Mul(quantity, &price.coefficient)
	return normalizeDecimal(coefficient, decimals+int(price.scale)), true
}

type PriceQuote struct {
	ChainID uint64    `json:"chain_id"`
	UnitID  string    `json:"unit_id"`
	USD     *Decimal  `json:"usd"`
	Source  string    `json:"source,omitempty"`
	AsOf    time.Time `json:"as_of,omitempty"`
	Status  Status    `json:"status"`
}

type PriceResolution struct {
	Status   Status       `json:"status"`
	Quotes   []PriceQuote `json:"quotes"`
	Failures []Failure    `json:"failures"`
}

type PositionValuation struct {
	Wallet         Address           `json:"wallet"`
	Chain          string            `json:"chain"`
	Protocol       string            `json:"protocol"`
	InstrumentID   string            `json:"instrument_id"`
	ExposureID     string            `json:"exposure_id"`
	Class          PositionClass     `json:"class"`
	AccountingRole AccountingRole    `json:"accounting_role"`
	Atomic         string            `json:"atomic"`
	Decimals       int               `json:"decimals"`
	Snapshot       SnapshotReference `json:"snapshot"`
	Price          *Decimal          `json:"price_usd"`
	Value          *Decimal          `json:"value_usd"`
	Source         string            `json:"source,omitempty"`
	AsOf           time.Time         `json:"as_of,omitempty"`
	Status         Status            `json:"status"`
}

type Valuation struct {
	Status            Status              `json:"status"`
	Items             []PositionValuation `json:"items"`
	AvailableTotalUSD *Decimal            `json:"available_total_usd"`
	Failures          []Failure           `json:"failures"`
	CoverageComplete  bool                `json:"coverage_complete"`
}

func ValuePosition(position VerifiedPosition, quote PriceQuote) PositionValuation {
	result := PositionValuation{Wallet: position.Wallet, Chain: position.Chain, Protocol: position.Protocol, InstrumentID: position.InstrumentID,
		ExposureID: position.ExposureID, Class: position.Class, AccountingRole: position.AccountingRole, Atomic: position.Atomic,
		Decimals: position.Decimals, Snapshot: position.Snapshot, Status: MissingPrice}
	if position.Status != OK || quote.Status != OK || quote.USD == nil {
		return result
	}
	atomic, validAtomic := new(big.Int).SetString(position.Atomic, 10)
	if !validAtomic || atomic.Sign() < 0 || atomic.String() != position.Atomic || position.Decimals < 0 || position.Decimals > maxTokenDecimals {
		result.Status = InvalidResponse
		return result
	}
	if quote.USD.coefficient.Sign() <= 0 || quote.ChainID != position.Snapshot.ChainID || strings.TrimSpace(quote.UnitID) != position.ValuationUnitID || strings.TrimSpace(quote.Source) == "" || quote.AsOf.IsZero() {
		result.Status = InvalidResponse
		return result
	}
	value, ok := multiplyAtomic(position.Atomic, position.Decimals, *quote.USD)
	if !ok {
		result.Status = InvalidResponse
		return result
	}
	price := *quote.USD
	result.Price, result.Value, result.Source, result.AsOf, result.Status = &price, &value, strings.TrimSpace(quote.Source), quote.AsOf.UTC(), OK
	return result
}

func ValuePositions(positions []VerifiedPosition, resolution PriceResolution) Valuation {
	result := Valuation{Status: resolution.Status, Failures: append([]Failure(nil), resolution.Failures...)}
	quotes := make(map[string]PriceQuote)
	conflicts := make(map[string]bool)
	for _, quote := range resolution.Quotes {
		key := quoteKey(quote.ChainID, quote.UnitID)
		if previous, found := quotes[key]; found && !equalQuote(previous, quote) {
			delete(quotes, key)
			if !conflicts[key] {
				result.Failures = append(result.Failures, Failure{Scope: "price/" + strings.TrimSpace(quote.UnitID), Status: InvalidResponse})
				conflicts[key] = true
			}
			continue
		}
		if !conflicts[key] {
			quotes[key] = quote
		}
	}
	for _, position := range positions {
		quote, found := quotes[quoteKey(position.Snapshot.ChainID, position.ValuationUnitID)]
		if !found {
			result.Items = append(result.Items, ValuePosition(position, PriceQuote{Status: MissingPrice}))
			continue
		}
		result.Items = append(result.Items, ValuePosition(position, quote))
	}
	result.AvailableTotalUSD = SumAvailableValues(result.Items)
	if result.Status == "" {
		result.Status = OK
	}
	if len(result.Failures) > 0 {
		result.Status = InvalidResponse
	}
	result.CoverageComplete = false
	return result
}

func quoteKey(chainID uint64, unitID string) string {
	return new(big.Int).SetUint64(chainID).String() + "\x00" + strings.TrimSpace(unitID)
}

func equalQuote(left, right PriceQuote) bool {
	if left.ChainID != right.ChainID || strings.TrimSpace(left.UnitID) != strings.TrimSpace(right.UnitID) || left.Status != right.Status ||
		strings.TrimSpace(left.Source) != strings.TrimSpace(right.Source) || !left.AsOf.Equal(right.AsOf) || left.USD == nil != (right.USD == nil) {
		return false
	}
	return left.USD == nil || left.USD.String() == right.USD.String()
}

func SumAvailableValues(items []PositionValuation) *Decimal {
	var sum *Decimal
	for _, item := range items {
		if item.Status != OK || item.Value == nil {
			continue
		}
		if sum == nil {
			value := *item.Value
			sum = &value
			continue
		}
		sum = addDecimals(*sum, *item.Value)
	}
	return sum
}

func addDecimals(left, right Decimal) *Decimal {
	scale := max(int(left.scale), int(right.scale))
	leftCoefficient := scaledCoefficient(left, scale)
	rightCoefficient := scaledCoefficient(right, scale)
	value := normalizeDecimal(new(big.Int).Add(leftCoefficient, rightCoefficient), scale)
	return &value
}

func scaledCoefficient(decimal Decimal, scale int) *big.Int {
	coefficient := new(big.Int).Set(&decimal.coefficient)
	if difference := scale - int(decimal.scale); difference > 0 {
		coefficient.Mul(coefficient, new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(difference)), nil))
	}
	return coefficient
}
