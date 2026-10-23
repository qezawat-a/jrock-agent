package bitunix

import (
	"errors"
	"fmt"
	"math/big"
	"strings"
)

// OrderUnit is the denomination selected in the Bitunix UI. The REST API order
// qty is still expressed in base-asset quantity, so NominalValue and CostValue
// selections must be converted before calling place_order.
type OrderUnit string

const (
	UnitQuantity     OrderUnit = "QUANTITY"
	UnitNominalValue OrderUnit = "NOMINAL_VALUE"
	UnitCostValue    OrderUnit = "COST_VALUE"
)

type TradingPair struct {
	Symbol               string `json:"symbol"`
	Base                 string `json:"base"`
	Quote                string `json:"quote"`
	MinTradeVolume       string `json:"minTradeVolume"`
	MaxLimitOrderVolume  string `json:"maxLimitOrderVolume"`
	MaxMarketOrderVolume string `json:"maxMarketOrderVolume"`
	BasePrecision        int    `json:"basePrecision"`
	QuotePrecision       int    `json:"quotePrecision"`
	MinLeverage          int    `json:"minLeverage"`
	MaxLeverage          int    `json:"maxLeverage"`
	SymbolStatus         string `json:"symbolStatus"`
	IsAPISupported       bool   `json:"isApiSupported"`
}

type OrderKind string

const (
	OrderMarket OrderKind = "MARKET"
	OrderLimit  OrderKind = "LIMIT"
)

type OrderSizing struct {
	Qty          string    `json:"qty"`
	NominalValue string    `json:"nominalValue"`
	CostValue    string    `json:"costValue"`
	Leverage     int       `json:"leverage"`
	InputUnit    OrderUnit `json:"inputUnit"`
}

// CalculateOrderSize converts a Bitunix UI order amount to the base-asset qty
// expected by the REST API. It rounds quantity down to the pair's basePrecision
// and rejects amounts that violate known symbol/leverage limits. Fee is excluded
// from Cost Value, as the official example explicitly says when explaining the formula.
func CalculateOrderSize(amount, price string, leverage int, unit OrderUnit, kind OrderKind, pair TradingPair) (OrderSizing, error) {
	if pair.Symbol == "" {
		return OrderSizing{}, errors.New("trading-pair metadata is required")
	}
	if pair.SymbolStatus != "" && pair.SymbolStatus != "OPEN" {
		return OrderSizing{}, fmt.Errorf("symbol %s status %s does not allow new trades", pair.Symbol, pair.SymbolStatus)
	}
	if !pair.IsAPISupported {
		return OrderSizing{}, fmt.Errorf("API trading is not enabled for %s", pair.Symbol)
	}
	if leverage < 1 || (pair.MinLeverage > 0 && leverage < pair.MinLeverage) || (pair.MaxLeverage > 0 && leverage > pair.MaxLeverage) {
		return OrderSizing{}, fmt.Errorf("leverage %d is outside the symbol's allowed range", leverage)
	}
	if pair.BasePrecision < 0 || pair.BasePrecision > 18 {
		return OrderSizing{}, errors.New("invalid basePrecision in trading-pair metadata")
	}
	if kind != OrderMarket && kind != OrderLimit {
		return OrderSizing{}, fmt.Errorf("unsupported order kind %q", kind)
	}
	a, ok := new(big.Rat).SetString(amount)
	if !ok || a.Sign() <= 0 {
		return OrderSizing{}, errors.New("amount must be a positive decimal")
	}
	p, ok := new(big.Rat).SetString(price)
	if !ok || p.Sign() <= 0 {
		return OrderSizing{}, errors.New("price must be a positive decimal")
	}
	qty := new(big.Rat)
	switch unit {
	case UnitQuantity:
		qty.Set(a)
	case UnitNominalValue:
		qty.Quo(a, p)
	case UnitCostValue:
		qty.Mul(a, big.NewRat(int64(leverage), 1))
		qty.Quo(qty, p)
	default:
		return OrderSizing{}, fmt.Errorf("unsupported order unit %q", unit)
	}
	qty, qtyString, err := floorDecimal(qty, pair.BasePrecision)
	if err != nil {
		return OrderSizing{}, err
	}
	if qty.Sign() <= 0 {
		return OrderSizing{}, errors.New("order quantity rounds down to zero at the symbol precision")
	}
	if min, ok := decimal(pair.MinTradeVolume); ok && qty.Cmp(min) < 0 {
		return OrderSizing{}, fmt.Errorf("quantity %s is below minimum trade volume %s", qtyString, pair.MinTradeVolume)
	}
	maxVol := pair.MaxMarketOrderVolume
	if kind == OrderLimit {
		maxVol = pair.MaxLimitOrderVolume
	}
	if max, ok := decimal(maxVol); ok && max.Sign() > 0 && qty.Cmp(max) > 0 {
		return OrderSizing{}, fmt.Errorf("quantity %s exceeds maximum %s order volume %s", qtyString, strings.ToLower(string(kind)), maxVol)
	}
	nominal := new(big.Rat).Mul(qty, p)
	cost := new(big.Rat).Quo(new(big.Rat).Set(nominal), big.NewRat(int64(leverage), 1))
	return OrderSizing{Qty: qtyString, NominalValue: formatRat(nominal, 8), CostValue: formatRat(cost, 8), Leverage: leverage, InputUnit: unit}, nil
}

func decimal(value string) (*big.Rat, bool) {
	if strings.TrimSpace(value) == "" {
		return nil, false
	}
	r, ok := new(big.Rat).SetString(value)
	return r, ok
}

func floorDecimal(value *big.Rat, precision int) (*big.Rat, string, error) {
	if value.Sign() < 0 {
		return nil, "", errors.New("quantity cannot be negative")
	}
	factor := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(precision)), nil)
	scaled := new(big.Rat).Mul(value, new(big.Rat).SetInt(factor))
	units := new(big.Int).Quo(scaled.Num(), scaled.Denom()) // positive values truncate down
	result := new(big.Rat).SetFrac(units, factor)
	return result, formatFixed(units, precision), nil
}

func formatFixed(units *big.Int, precision int) string {
	neg := units.Sign() < 0
	abs := new(big.Int).Abs(new(big.Int).Set(units)).String()
	if precision == 0 {
		if neg {
			return "-" + abs
		}
		return abs
	}
	for len(abs) <= precision {
		abs = "0" + abs
	}
	whole, frac := abs[:len(abs)-precision], abs[len(abs)-precision:]
	frac = strings.TrimRight(frac, "0")
	out := whole
	if frac != "" {
		out += "." + frac
	}
	if neg {
		out = "-" + out
	}
	return out
}

func formatRat(value *big.Rat, precision int) string {
	if value == nil {
		return "0"
	}
	f := new(big.Float).SetPrec(256).SetRat(value)
	return strings.TrimRight(strings.TrimRight(f.Text('f', precision), "0"), ".")
}
