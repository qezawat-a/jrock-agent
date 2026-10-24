//go:build integration

package bitunix

import (
	"context"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/yourname/jrock-agent/internal/trader"
)

// This opt-in test calls Bitunix public market metadata only; it never authenticates
// or mutates an account. Run with: go test -tags=integration ./internal/bitunix.
func TestBitunixPublicTradingPairMetadata(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	client, err := NewPublicClient(DefaultBaseURL)
	if err != nil {
		t.Fatal(err)
	}
	pairs, err := client.GetTradingPairs(ctx, []string{"BTCUSDT"})
	if err != nil {
		t.Fatalf("Bitunix public trading_pairs request: %v", err)
	}
	if len(pairs) != 1 {
		t.Fatalf("returned %d BTCUSDT metadata rows, want 1", len(pairs))
	}
	p := pairs[0]
	if p.Symbol != "BTCUSDT" || p.Base != "BTC" || p.Quote != "USDT" {
		t.Fatalf("unexpected pair metadata: %+v", p)
	}
	if p.BasePrecision < 0 || p.BasePrecision > 18 || p.MinTradeVolume == "" {
		t.Fatalf("metadata lacks usable quantity rules: %+v", p)
	}
}

// This also verifies that the public candle feed can drive the paper signal
// calculation. It never authenticates, places orders, or changes settings.
func TestBitunixPublicKlinesFeedPaperSignal(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	client, err := NewPublicClient(DefaultBaseURL)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := client.GetKlines(ctx, "BTCUSDT", "15m", "LAST_PRICE", 100)
	if err != nil {
		t.Fatalf("Bitunix public kline request: %v", err)
	}
	if len(rows) < 50 {
		t.Fatalf("received %d candles; need at least 50 for the signal engine", len(rows))
	}
	candles := make([]trader.Candle, 0, len(rows))
	for _, row := range rows {
		open, e1 := strconv.ParseFloat(row.Open, 64)
		high, e2 := strconv.ParseFloat(row.High, 64)
		low, e3 := strconv.ParseFloat(row.Low, 64)
		closePrice, e4 := strconv.ParseFloat(row.Close, 64)
		volume, e5 := strconv.ParseFloat(row.BaseVol, 64)
		if e1 != nil || e2 != nil || e3 != nil || e4 != nil || e5 != nil {
			t.Fatalf("invalid candle row: %+v", row)
		}
		candles = append(candles, trader.Candle{Time: row.Time, Open: open, High: high, Low: low, Close: closePrice, BaseVolume: volume})
	}
	sort.Slice(candles, func(i, j int) bool { return candles[i].Time < candles[j].Time })
	if _, err := trader.EvaluateFrame(candles, 3, 60); err != nil {
		t.Fatalf("evaluate Bitunix public candles: %v", err)
	}
}
