package bitunix

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPlaceOrderByUnitSendsConvertedBaseAssetQty(t *testing.T) {
	t.Setenv(LiveOptInEnv, LiveOptInValue)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/futures/trade/place_order" {
			t.Errorf("request=%s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("sign") == "" || r.Header.Get("api-key") != "mock-key" {
			t.Error("expected signed request headers")
		}
		var payload struct {
			Symbol string `json:"symbol"`
			Qty    string `json:"qty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode body: %v", err)
		}
		if payload.Symbol != "BTCUSDT" || payload.Qty != "0.1" {
			t.Errorf("order payload=%+v, want BTCUSDT qty=0.1", payload)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"data":{"orderId":"mock-order"},"msg":"Success"}`))
	}))
	defer server.Close()

	client, err := NewLiveClient(server.URL, "mock-key", "mock-secret", LiveOptInValue)
	if err != nil {
		t.Fatal(err)
	}
	pair := TradingPair{
		Symbol: "BTCUSDT", MinTradeVolume: "0.001", MaxMarketOrderVolume: "100",
		MaxLimitOrderVolume: "100", BasePrecision: 3, MinLeverage: 1, MaxLeverage: 100,
		SymbolStatus: "OPEN", IsAPISupported: true,
	}
	result, sizing, err := client.PlaceOrderByUnit(context.Background(), OrderRequest{
		Symbol: "BTCUSDT", Side: "BUY", OrderType: "MARKET",
	}, "1000", "10000", 10, UnitNominalValue, pair)
	if err != nil {
		t.Fatal(err)
	}
	if result.OrderID != "mock-order" {
		t.Fatalf("order id=%q", result.OrderID)
	}
	if sizing.Qty != "0.1" || sizing.NominalValue != "1000" || sizing.CostValue != "100" {
		t.Fatalf("unexpected sizing result: %+v", sizing)
	}
}
