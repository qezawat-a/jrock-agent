package bitunix

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetKlinesPublicRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/futures/market/kline" {
			t.Errorf("request=%s %s", r.Method, r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("symbol") != "BTCUSDT" || q.Get("interval") != "15m" || q.Get("limit") != "200" || q.Get("type") != "MARK_PRICE" {
			t.Errorf("query=%v", q)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"code":0,"data":[{"open":"100","high":"105","low":"99","close":"103","quoteVol":"1000","baseVol":"10","time":123,"type":"MARK_PRICE"}]}`)
	}))
	defer server.Close()
	c, err := NewPublicClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	klines, err := c.GetKlines(context.Background(), "BTCUSDT", "15m", "MARK_PRICE", 200)
	if err != nil {
		t.Fatal(err)
	}
	if len(klines) != 1 || klines[0].Close != "103" || klines[0].Time != 123 {
		t.Fatalf("klines=%+v", klines)
	}
}

func TestKlineTimeAcceptsNumberAndQuotedNumber(t *testing.T) {
	for _, raw := range []string{`{"time":123}`, `{"time":"123"}`} {
		var got Kline
		if err := json.Unmarshal([]byte(raw), &got); err != nil {
			t.Fatalf("Unmarshal(%s): %v", raw, err)
		}
		if got.Time != 123 {
			t.Fatalf("Unmarshal(%s) time=%d, want 123", raw, got.Time)
		}
	}
}

func TestGetKlinesRejectsInvalidInputsBeforeRequest(t *testing.T) {
	c, err := NewPublicClient("https://example.com")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, symbol, interval, priceType string
		limit                             int
	}{
		{"missing symbol", "", "15m", "LAST_PRICE", 100},
		{"bad interval", "BTCUSDT", "2m", "LAST_PRICE", 100},
		{"bad price type", "BTCUSDT", "15m", "INDEX", 100},
		{"too many candles", "BTCUSDT", "15m", "LAST_PRICE", 201},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := c.GetKlines(context.Background(), tc.symbol, tc.interval, tc.priceType, tc.limit); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}
