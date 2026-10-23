package bitunix

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func wsURL(serverURL string) string {
	u, _ := url.Parse(serverURL)
	u.Scheme = "ws"
	return u.String()
}

func TestPublicWebSocketSubscribePingAndReceive(t *testing.T) {
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		defer conn.Close()
		var sub wsOperation
		if err := conn.ReadJSON(&sub); err != nil {
			t.Errorf("read subscribe: %v", err)
			return
		}
		if sub.Op != "subscribe" || len(sub.Args) != 1 || sub.Args[0].Symbol != "BTCUSDT" || sub.Args[0].Channel != "price" {
			t.Errorf("subscribe payload = %+v", sub)
			return
		}
		var ping wsPingRequest
		if err := conn.ReadJSON(&ping); err != nil {
			t.Errorf("read ping: %v", err)
			return
		}
		if ping.Op != "ping" || ping.Ping <= 0 {
			t.Errorf("ping payload = %+v", ping)
			return
		}
		_ = conn.WriteJSON(map[string]any{"ch": "price", "symbol": "BTCUSDT", "data": map[string]string{"mp": "65000"}})
	}))
	defer server.Close()

	client, err := DialPublicWS(context.Background(), wsURL(server.URL))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if err := client.Subscribe(WSSubscription{Symbol: "BTCUSDT", Channel: "price"}); err != nil {
		t.Fatal(err)
	}
	if err := client.Ping(time.Now().Unix()); err != nil {
		t.Fatal(err)
	}
	var got struct {
		Channel string `json:"ch"`
		Symbol  string `json:"symbol"`
		Data    struct {
			MP string `json:"mp"`
		} `json:"data"`
	}
	if err := client.ReadJSON(&got); err != nil {
		t.Fatal(err)
	}
	if got.Channel != "price" || got.Symbol != "BTCUSDT" || got.Data.MP != "65000" {
		t.Fatalf("push = %+v", got)
	}
}

func TestPrivateWebSocketLoginSignatureAndSubscriptions(t *testing.T) {
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		defer conn.Close()
		var login wsLoginRequest
		if err := conn.ReadJSON(&login); err != nil {
			t.Errorf("read login: %v", err)
			return
		}
		if login.Op != "login" || len(login.Args) != 1 {
			t.Errorf("login payload = %+v", login)
			return
		}
		arg := login.Args[0]
		if arg.APIKey != "mock-key" || arg.Timestamp <= 0 || len(arg.Nonce) != 32 {
			t.Errorf("login auth fields = %+v", arg)
			return
		}
		want := signature(arg.Nonce, strconv.FormatInt(arg.Timestamp, 10), "mock-key", "mock-secret", "", "")
		if arg.Sign != want {
			t.Errorf("login signature = %q, want %q", arg.Sign, want)
			return
		}
		_ = conn.WriteJSON(map[string]any{"op": "login", "code": 0})
		var sub wsOperation
		if err := conn.ReadJSON(&sub); err != nil {
			t.Errorf("read subscribe: %v", err)
			return
		}
		if sub.Op != "subscribe" || len(sub.Args) != 3 || sub.Args[0].Channel != "balance" || sub.Args[1].Channel != "position" || sub.Args[2].Channel != "order" {
			t.Errorf("private subscription = %+v", sub)
			return
		}
		_ = conn.WriteJSON(map[string]any{"op": "login", "code": 0})
	}))
	defer server.Close()

	client, err := DialPrivateWS(context.Background(), wsURL(server.URL), "mock-key", "mock-secret")
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	var ack map[string]any
	if err := client.ReadJSON(&ack); err != nil {
		t.Fatal(err)
	}
	if code, ok := ack["code"].(float64); !ok || code != 0 {
		t.Fatalf("login acknowledgement = %#v", ack)
	}
	if err := client.SubscribePrivateCore(); err != nil {
		t.Fatal(err)
	}
	if err := client.ReadJSON(&ack); err != nil {
		t.Fatal(err)
	}
}

func TestWebSocketValidation(t *testing.T) {
	if _, err := DialPublicWS(context.Background(), "https://example.com/ws"); err == nil {
		t.Fatal("expected invalid websocket URL error")
	}
	if _, err := DialPrivateWS(context.Background(), "wss://example.com/private", "", "secret"); err == nil {
		t.Fatal("expected missing API key error")
	}
	if err := (*WSClient)(nil).Subscribe(WSSubscription{Channel: "price"}); err == nil {
		t.Fatal("expected disconnected client error")
	}
}
