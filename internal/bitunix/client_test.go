package bitunix

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
)

func TestSignature_MatchesOfficialVector(t *testing.T) {
	got := signature("123456", "20241120123045", "yourApiKey", "yourSecretKey", "id1uid200", `{"uid":"2899","arr":[{"id":1,"name":"maple"},{"id":2,"name":"lily"}]}`)
	if want := "00397cd1e52c7dce3258067324363b6361fabc9178a0912b330c138db8745655"; got != want {
		t.Fatalf("signature = %s, want %s", got, want)
	}
}

func TestCanonicalQuerySortsKeysAndValues(t *testing.T) {
	q := url.Values{"symbol": {"ETHUSDT"}, "marginCoin": {"USDT"}, "foo": {"z", "a"}}
	if got, want := canonicalQuery(q), "fooafoozmarginCoinUSDTsymbolETHUSDT"; got != want {
		t.Fatalf("canonicalQuery() = %q, want %q", got, want)
	}
}

func TestNewClientRejectsMissingSecretsAndInsecureNonLoopbackURL(t *testing.T) {
	if _, err := NewClient("", "", ""); err == nil {
		t.Fatal("missing credentials should be rejected")
	}
	if _, err := NewClient("http://example.com", "key", "secret"); err == nil {
		t.Fatal("non-loopback HTTP must be rejected")
	}
}

func TestNewLiveClientRequiresDoubleOptIn(t *testing.T) {
	t.Setenv(LiveOptInEnv, "")
	if _, err := NewLiveClient("", "key", "secret", LiveOptInValue); err == nil {
		t.Fatal("live client constructed without environment opt-in")
	}
	t.Setenv(LiveOptInEnv, LiveOptInValue)
	if _, err := NewLiveClient("", "key", "secret", ""); err == nil {
		t.Fatal("live client constructed without call-site acknowledgement")
	}
}

func TestReadOnlyClientBlocksWritesBeforeNetwork(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests.Add(1) }))
	defer server.Close()
	client, err := NewClient(server.URL, "key", "secret")
	if err != nil {
		t.Fatal(err)
	}
	if !client.ReadOnly() {
		t.Fatal("NewClient should be read-only")
	}
	_, err = client.ChangeLeverage(context.Background(), LeverageRequest{Symbol: "BTCUSDT", MarginCoin: "USDT", Leverage: 2})
	if err == nil || !strings.Contains(err.Error(), "write request blocked") {
		t.Fatalf("write error = %v", err)
	}
	if requests.Load() != 0 {
		t.Fatalf("blocked write reached server; requests=%d", requests.Load())
	}
}

func TestSignedReadRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/futures/account" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		if got := r.URL.Query().Get("marginCoin"); got != "USDT" {
			t.Errorf("marginCoin = %q", got)
		}
		assertSignedRequest(t, r, "test-key", "test-secret", "marginCoinUSDT", "")
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"code":0,"data":[{"marginCoin":"USDT","available":"100"}],"msg":"Success"}`)
	}))
	defer server.Close()
	client, err := NewClient(server.URL, "test-key", "test-secret")
	if err != nil {
		t.Fatal(err)
	}
	data, err := client.GetAccount(context.Background(), "USDT")
	if err != nil {
		t.Fatalf("GetAccount() error = %v", err)
	}
	if !strings.Contains(string(data), `"available":"100"`) {
		t.Fatalf("account data = %s", data)
	}
}

func TestLiveOrderWritesOnlyToLocalMock(t *testing.T) {
	t.Setenv(LiveOptInEnv, LiveOptInValue)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/futures/trade/place_order" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		assertSignedRequest(t, r, "test-key", "test-secret", "", string(body))
		if !strings.Contains(string(body), `"qty":"0.001"`) {
			t.Errorf("order qty body = %s", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"code":0,"data":{"orderId":"mock-order-1","clientId":"test-client"},"msg":"Success"}`)
	}))
	defer server.Close()
	client, err := NewLiveClient(server.URL, "test-key", "test-secret", LiveOptInValue)
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.PlaceOrder(context.Background(), OrderRequest{Symbol: "BTCUSDT", Qty: "0.001", Side: "BUY", TradeSide: "OPEN", OrderType: "MARKET"})
	if err != nil {
		t.Fatalf("PlaceOrder() error = %v", err)
	}
	if result.OrderID != "mock-order-1" {
		t.Fatalf("order result = %+v", result)
	}
}

func TestTradingPairsPublicRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/futures/market/trading_pairs" || r.URL.Query().Get("symbols") != "BTCUSDT" {
			t.Errorf("request path/query = %s?%s", r.URL.Path, r.URL.RawQuery)
		}
		if r.Header.Get("api-key") != "" {
			t.Error("public endpoint unexpectedly sent API key")
		}
		_, _ = fmt.Fprint(w, `{"code":0,"data":[{"symbol":"BTCUSDT","base":"BTC","quote":"USDT","minTradeVolume":"0.0001","basePrecision":4,"quotePrecision":1,"maxLeverage":125,"symbolStatus":"OPEN","isApiSupported":true}],"msg":"Success"}`)
	}))
	defer server.Close()
	client, err := NewClient(server.URL, "key", "secret")
	if err != nil {
		t.Fatal(err)
	}
	pairs, err := client.GetTradingPairs(context.Background(), []string{"BTCUSDT"})
	if err != nil {
		t.Fatalf("GetTradingPairs() error = %v", err)
	}
	if len(pairs) != 1 || pairs[0].BasePrecision != 4 || !pairs[0].IsAPISupported {
		t.Fatalf("pairs = %+v", pairs)
	}
}

func TestPrivateAPIErrorDoesNotExposeResponseBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"code":123,"data":null,"msg":"secret account detail"}`)
	}))
	defer server.Close()
	client, err := NewClient(server.URL, "key", "secret")
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.GetAccount(context.Background(), "USDT")
	if err == nil || !strings.Contains(err.Error(), "code 123") || strings.Contains(err.Error(), "secret account detail") {
		t.Fatalf("unexpected API error: %v", err)
	}
}

func TestPlaceOrderByUnitUsesConvertedBaseQuantity(t *testing.T) {
	t.Setenv(LiveOptInEnv, LiveOptInValue)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		if !strings.Contains(string(body), `"qty":"0.1"`) {
			t.Errorf("REST qty must be base quantity after nominal-value conversion, body=%s", body)
		}
		_, _ = fmt.Fprint(w, `{"code":0,"data":{"orderId":"mock-order"},"msg":"Success"}`)
	}))
	defer server.Close()
	client, err := NewLiveClient(server.URL, "test-key", "test-secret", LiveOptInValue)
	if err != nil {
		t.Fatal(err)
	}
	result, sizing, err := client.PlaceOrderByUnit(context.Background(), OrderRequest{Symbol: "BTCUSDT", Side: "BUY", TradeSide: "OPEN", OrderType: "MARKET"}, "1000", "10000", 10, UnitNominalValue, testPair())
	if err != nil {
		t.Fatalf("PlaceOrderByUnit() error = %v", err)
	}
	if result.OrderID != "mock-order" || sizing.Qty != "0.1" || sizing.NominalValue != "1000" || sizing.CostValue != "100" {
		t.Fatalf("result=%+v sizing=%+v", result, sizing)
	}
}

func assertSignedRequest(t *testing.T, r *http.Request, apiKey, secret, canonical, body string) {
	t.Helper()
	nonce, timestamp := r.Header.Get("nonce"), r.Header.Get("timestamp")
	if nonce == "" || timestamp == "" || r.Header.Get("api-key") != apiKey {
		t.Fatalf("missing or invalid signature headers")
	}
	got := r.Header.Get("sign")
	want := signature(nonce, timestamp, apiKey, secret, canonical, body)
	if got != want {
		t.Fatalf("sign header mismatch: got %s, want %s", got, want)
	}
}
