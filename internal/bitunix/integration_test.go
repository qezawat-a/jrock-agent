//go:build integration

package bitunix

import (
	"context"
	"testing"
	"time"
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
