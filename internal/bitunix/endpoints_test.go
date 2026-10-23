package bitunix

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestClientEndpointRoutesAgainstLocalMock(t *testing.T) {
	t.Setenv(LiveOptInEnv, LiveOptInValue)
	tests := []struct {
		name   string
		method string
		path   string
		call   func(context.Context, *Client) error
	}{
		{name: "leverage change", method: http.MethodPost, path: "/api/v1/futures/account/change_leverage", call: func(ctx context.Context, c *Client) error {
			_, e := c.ChangeLeverage(ctx, LeverageRequest{Symbol: "BTCUSDT", MarginCoin: "USDT", Leverage: 3})
			return e
		}},
		{name: "margin mode change", method: http.MethodPost, path: "/api/v1/futures/account/change_margin_mode", call: func(ctx context.Context, c *Client) error {
			_, e := c.ChangeMarginMode(ctx, MarginModeRequest{Symbol: "BTCUSDT", MarginCoin: "USDT", MarginMode: "ISOLATION"})
			return e
		}},
		{name: "position mode change", method: http.MethodPost, path: "/api/v1/futures/account/change_position_mode", call: func(ctx context.Context, c *Client) error {
			_, e := c.ChangePositionMode(ctx, PositionModeRequest{PositionMode: "HEDGE"})
			return e
		}},
		{name: "position stop placement", method: http.MethodPost, path: "/api/v1/futures/tpsl/position/place_order", call: func(ctx context.Context, c *Client) error {
			_, e := c.PlacePositionTPSL(ctx, PositionTPSLRequest{Symbol: "BTCUSDT", PositionID: "position-1", SLPrice: "60000", SLStopType: "MARK_PRICE"})
			return e
		}},
		{name: "position stop modification", method: http.MethodPost, path: "/api/v1/futures/tpsl/position/modify_order", call: func(ctx context.Context, c *Client) error {
			_, e := c.ModifyPositionTPSL(ctx, PositionTPSLRequest{Symbol: "BTCUSDT", PositionID: "position-1", SLPrice: "61000", SLStopType: "MARK_PRICE"})
			return e
		}},
		{name: "TP/SL cancellation", method: http.MethodPost, path: "/api/v1/futures/tpsl/cancel_order", call: func(ctx context.Context, c *Client) error { _, e := c.CancelTPSL(ctx, "BTCUSDT", "tp-1"); return e }},
		{name: "position close", method: http.MethodPost, path: "/api/v1/futures/trade/flash_close_position", call: func(ctx context.Context, c *Client) error { _, e := c.FlashClosePosition(ctx, "position-1"); return e }},
		{name: "positions read", method: http.MethodGet, path: "/api/v1/futures/position/get_pending_positions", call: func(ctx context.Context, c *Client) error {
			_, e := c.GetPendingPositions(ctx, "BTCUSDT", "")
			return e
		}},
		{name: "positions history read", method: http.MethodGet, path: "/api/v1/futures/position/get_history_positions", call: func(ctx context.Context, c *Client) error {
			_, e := c.GetHistoryPositions(ctx, url.Values{"symbol": {"BTCUSDT"}})
			return e
		}},
		{name: "order history read", method: http.MethodGet, path: "/api/v1/futures/trade/get_history_orders", call: func(ctx context.Context, c *Client) error {
			_, e := c.GetHistoryOrders(ctx, url.Values{"symbol": {"BTCUSDT"}})
			return e
		}},
		{name: "trade history read", method: http.MethodGet, path: "/api/v1/futures/trade/get_history_trades", call: func(ctx context.Context, c *Client) error {
			_, e := c.GetHistoryTrades(ctx, url.Values{"symbol": {"BTCUSDT"}})
			return e
		}},
		{name: "pending TP/SL read", method: http.MethodGet, path: "/api/v1/futures/tpsl/get_pending_orders", call: func(ctx context.Context, c *Client) error {
			_, e := c.GetPendingTPSL(ctx, url.Values{"symbol": {"BTCUSDT"}})
			return e
		}},
		{name: "TP/SL history read", method: http.MethodGet, path: "/api/v1/futures/tpsl/get_history_orders", call: func(ctx context.Context, c *Client) error {
			_, e := c.GetHistoryTPSL(ctx, url.Values{"symbol": {"BTCUSDT"}})
			return e
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != tt.method || r.URL.Path != tt.path {
					t.Errorf("request=%s %s, want %s %s", r.Method, r.URL.Path, tt.method, tt.path)
				}
				if tt.method == http.MethodPost {
					if r.Header.Get("sign") == "" || r.Header.Get("api-key") == "" {
						t.Error("signed write headers missing")
					}
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = fmt.Fprint(w, `{"code":0,"data":{"ok":true},"msg":"Success"}`)
			}))
			defer server.Close()
			c, err := NewLiveClient(server.URL, "mock-key", "mock-secret", LiveOptInValue)
			if err != nil {
				t.Fatal(err)
			}
			if err := tt.call(context.Background(), c); err != nil {
				t.Fatalf("endpoint call error=%v", err)
			}
		})
	}
}
