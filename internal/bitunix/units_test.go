package bitunix

import "testing"

func testPair() TradingPair {
	return TradingPair{Symbol: "BTCUSDT", Base: "BTC", Quote: "USDT", MinTradeVolume: "0.0001", MaxLimitOrderVolume: "100000", MaxMarketOrderVolume: "50000", BasePrecision: 4, QuotePrecision: 1, MinLeverage: 1, MaxLeverage: 125, SymbolStatus: "OPEN", IsAPISupported: true}
}

func TestCalculateOrderSize(t *testing.T) {
	tests := []struct {
		name, amount, price            string
		leverage                       int
		unit                           OrderUnit
		kind                           OrderKind
		wantQty, wantNominal, wantCost string
	}{
		{name: "nominal value converted to base qty", amount: "1000", price: "10000", leverage: 10, unit: UnitNominalValue, kind: OrderMarket, wantQty: "0.1", wantNominal: "1000", wantCost: "100"},
		{name: "cost value applies leverage", amount: "100", price: "10000", leverage: 10, unit: UnitCostValue, kind: OrderMarket, wantQty: "0.1", wantNominal: "1000", wantCost: "100"},
		{name: "quantity unit is base coin", amount: "1", price: "10000", leverage: 10, unit: UnitQuantity, kind: OrderLimit, wantQty: "1", wantNominal: "10000", wantCost: "1000"},
		{name: "rounds quantity down to base precision", amount: "0.123456", price: "10000", leverage: 1, unit: UnitQuantity, kind: OrderLimit, wantQty: "0.1234", wantNominal: "1234", wantCost: "1234"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CalculateOrderSize(tt.amount, tt.price, tt.leverage, tt.unit, tt.kind, testPair())
			if err != nil {
				t.Fatalf("CalculateOrderSize() error = %v", err)
			}
			if got.Qty != tt.wantQty || got.NominalValue != tt.wantNominal || got.CostValue != tt.wantCost {
				t.Fatalf("CalculateOrderSize() = %+v, want qty=%s nominal=%s cost=%s", got, tt.wantQty, tt.wantNominal, tt.wantCost)
			}
		})
	}
}

func TestCalculateOrderSizeRejectsUnsafeInputs(t *testing.T) {
	disabledStatus := testPair()
	disabledStatus.SymbolStatus = "STOP"
	disabledAPI := testPair()
	disabledAPI.IsAPISupported = false
	tests := []struct {
		name, amount, price string
		leverage            int
		unit                OrderUnit
		kind                OrderKind
		pair                TradingPair
	}{
		{name: "invalid input unit", amount: "100", price: "10000", leverage: 1, unit: "USD", kind: OrderMarket, pair: testPair()},
		{name: "zero price", amount: "100", price: "0", leverage: 1, unit: UnitNominalValue, kind: OrderMarket, pair: testPair()},
		{name: "below minimum", amount: "0.00001", price: "10000", leverage: 1, unit: UnitQuantity, kind: OrderMarket, pair: testPair()},
		{name: "market maximum", amount: "50001", price: "1", leverage: 1, unit: UnitQuantity, kind: OrderMarket, pair: testPair()},
		{name: "leverage above tier max", amount: "100", price: "10000", leverage: 126, unit: UnitCostValue, kind: OrderMarket, pair: testPair()},
		{name: "symbol not open", amount: "100", price: "10000", leverage: 1, unit: UnitNominalValue, kind: OrderMarket, pair: disabledStatus},
		{name: "API trading disabled", amount: "100", price: "10000", leverage: 1, unit: UnitNominalValue, kind: OrderMarket, pair: disabledAPI},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := CalculateOrderSize(tt.amount, tt.price, tt.leverage, tt.unit, tt.kind, tt.pair); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}
