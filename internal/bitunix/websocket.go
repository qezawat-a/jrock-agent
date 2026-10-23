package bitunix

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// Bitunix allows at most five client-to-server websocket messages per second.
	wsMinWriteInterval = 200 * time.Millisecond
	wsMaxSubscriptions = 300
)

type WSSubscription struct {
	Symbol  string `json:"symbol,omitempty"`
	Channel string `json:"ch"`
}

type wsLoginArg struct {
	APIKey    string `json:"apiKey"`
	Timestamp int64  `json:"timestamp"`
	Nonce     string `json:"nonce"`
	Sign      string `json:"sign"`
}

type wsLoginRequest struct {
	Op   string       `json:"op"`
	Args []wsLoginArg `json:"args"`
}

type wsOperation struct {
	Op   string           `json:"op"`
	Args []WSSubscription `json:"args"`
}

type wsPingRequest struct {
	Op   string `json:"op"`
	Ping int64  `json:"ping"`
}

// WSClient is a low-level Bitunix websocket transport. It does not reconnect,
// reconcile exchange state, or submit trades. For private sessions, read and
// validate the login acknowledgement before subscribing to private channels.
type WSClient struct {
	conn      *websocket.Conn
	writeMu   sync.Mutex
	nextWrite time.Time
	private   bool
}

func DialPublicWS(ctx context.Context, endpoint string) (*WSClient, error) {
	conn, err := dialBitunixWS(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	return &WSClient{conn: conn}, nil
}

// DialPrivateWS opens the private socket and sends Bitunix's signed login
// message. Timestamp is Unix seconds (unlike signed REST requests, which use ms).
// The caller must read the login response and confirm success before subscribing.
func DialPrivateWS(ctx context.Context, endpoint, apiKey, secret string) (*WSClient, error) {
	if strings.TrimSpace(apiKey) == "" || strings.TrimSpace(secret) == "" {
		return nil, errors.New("Bitunix API key and secret are required for private websocket login")
	}
	conn, err := dialBitunixWS(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	client := &WSClient{conn: conn, private: true}
	nonce, err := newWSNonce()
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("generate websocket nonce: %w", err)
	}
	timestamp := time.Now().Unix()
	login := wsLoginRequest{Op: "login", Args: []wsLoginArg{{
		APIKey: apiKey, Timestamp: timestamp, Nonce: nonce,
		Sign: signature(nonce, fmt.Sprintf("%d", timestamp), apiKey, secret, "", ""),
	}}}
	if err := client.WriteJSON(login); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("send Bitunix websocket login: %w", err)
	}
	return client, nil
}

func dialBitunixWS(ctx context.Context, endpoint string) (*websocket.Conn, error) {
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" || (u.Scheme != "wss" && u.Scheme != "ws") {
		return nil, errors.New("Bitunix websocket endpoint must be a valid ws:// or wss:// URL")
	}
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("connect Bitunix websocket: %w", err)
	}
	return conn, nil
}

func newWSNonce() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (c *WSClient) Subscribe(args ...WSSubscription) error {
	return c.writeSubscription("subscribe", args)
}

func (c *WSClient) Unsubscribe(args ...WSSubscription) error {
	return c.writeSubscription("unsubscribe", args)
}

func (c *WSClient) writeSubscription(op string, args []WSSubscription) error {
	if c == nil || c.conn == nil {
		return errors.New("websocket is not connected")
	}
	if len(args) == 0 || len(args) > wsMaxSubscriptions {
		return fmt.Errorf("subscription must contain between 1 and %d channels", wsMaxSubscriptions)
	}
	for i, arg := range args {
		if strings.TrimSpace(arg.Channel) == "" {
			return fmt.Errorf("subscription %d requires a channel name", i)
		}
	}
	return c.WriteJSON(wsOperation{Op: op, Args: args})
}

// SubscribePrivateCore subscribes to balance, position, and order pushes. The
// tpsl channel is pushed automatically after successful private login per docs.
func (c *WSClient) SubscribePrivateCore() error {
	if c == nil || !c.private {
		return errors.New("private channel subscriptions require a private websocket")
	}
	return c.Subscribe(WSSubscription{Channel: "balance"}, WSSubscription{Channel: "position"}, WSSubscription{Channel: "order"})
}

// Ping sends Bitunix's JSON application heartbeat. Use at a modest interval and
// keep all client messages, including subscriptions, at or below 5 per second.
func (c *WSClient) Ping(unixSeconds int64) error {
	if unixSeconds <= 0 {
		return errors.New("ping timestamp must be positive Unix seconds")
	}
	return c.WriteJSON(wsPingRequest{Op: "ping", Ping: unixSeconds})
}

// WriteJSON serializes concurrent writers and enforces Bitunix's documented
// five-message-per-second client limit with a 200ms minimum spacing.
func (c *WSClient) WriteJSON(value any) error {
	if c == nil || c.conn == nil {
		return errors.New("websocket is not connected")
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if wait := time.Until(c.nextWrite); wait > 0 {
		time.Sleep(wait)
	}
	_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	if err := c.conn.WriteJSON(value); err != nil {
		return err
	}
	c.nextWrite = time.Now().Add(wsMinWriteInterval)
	return nil
}

func (c *WSClient) ReadMessage() ([]byte, error) {
	if c == nil || c.conn == nil {
		return nil, errors.New("websocket is not connected")
	}
	_, message, err := c.conn.ReadMessage()
	if err != nil {
		return nil, err
	}
	return message, nil
}

func (c *WSClient) ReadJSON(value any) error {
	if c == nil || c.conn == nil {
		return errors.New("websocket is not connected")
	}
	return c.conn.ReadJSON(value)
}

func (c *WSClient) SetReadDeadline(deadline time.Time) error {
	if c == nil || c.conn == nil {
		return errors.New("websocket is not connected")
	}
	return c.conn.SetReadDeadline(deadline)
}

func (c *WSClient) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_ = c.conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Now().Add(time.Second))
	return c.conn.Close()
}
