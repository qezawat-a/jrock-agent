package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yourname/jrock-agent/internal/bitunix"
	"github.com/yourname/jrock-agent/internal/trader"
)

func TestScanOnceReadsClosedCandlesAndComputesPaperSignal(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/futures/market/kline" || r.URL.Query().Get("symbol") != "BTCUSDT" || r.URL.Query().Get("interval") != "15m" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.String())
		}
		var body strings.Builder
		body.WriteString(`{"code":0,"data":[`)
		for i := 59; i >= 0; i-- { // newest-first response exercises local chronological sorting
			if i != 59 {
				body.WriteByte(',')
			}
			price := 100 + i
			volume := 1
			if i == 59 {
				volume = 10
			}
			fmt.Fprintf(&body, `{"open":"%.1f","high":"%.1f","low":"%.1f","close":"%d","baseVol":"%d","time":%d}`, float64(price)-0.1, float64(price)+0.2, float64(price)-0.2, price, volume, i+1)
		}
		body.WriteString(`]}`)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body.String()))
	}))
	defer server.Close()
	client, err := bitunix.NewPublicClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	cfg := trader.Config{
		Symbols: []string{"BTCUSDT"}, Timeframes: []string{"15m"}, ScanIntervalSec: 15,
		MinAgreeingStrategies: 3, MinConfidence: 80, TFMinConfidence: 60,
	}
	if err := scanOnce(context.Background(), client, cfg); err != nil {
		t.Fatalf("scanOnce() error=%v", err)
	}
}
