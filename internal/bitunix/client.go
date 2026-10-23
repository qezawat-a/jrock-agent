// Package bitunix implements a signed Bitunix futures REST client.
// Read methods require credentials but do not mutate the account. Mutating methods
// are disabled unless explicitly constructed with NewLiveClient and its opt-in gate.
package bitunix

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"
)

const (
	DefaultBaseURL = "https://fapi.bitunix.com"
	LiveOptInEnv   = "BITUNIX_LIVE_TRADING"
	LiveOptInValue = "I_ACKNOWLEDGE_REAL_TRADING"
)

type Client struct {
	baseURL     string
	apiKey      string
	secret      string
	http        *http.Client
	allowWrites bool
}

type apiEnvelope struct {
	Code int             `json:"code"`
	Data json.RawMessage `json:"data"`
}

// NewClient creates an authenticated, read-only client. Write endpoints remain disabled.
func NewClient(baseURL, apiKey, secret string) (*Client, error) {
	return newClient(baseURL, apiKey, secret, false, false)
}

// NewPublicClient creates an unauthenticated client limited to PublicGet paths.
func NewPublicClient(baseURL string) (*Client, error) {
	return newClient(baseURL, "", "", false, true)
}

// NewLiveClient enables mutating endpoints only when both the exact acknowledgement
// argument and a separate process environment opt-in are present.
func NewLiveClient(baseURL, apiKey, secret, acknowledgement string) (*Client, error) {
	if acknowledgement != LiveOptInValue || os.Getenv(LiveOptInEnv) != LiveOptInValue {
		return nil, errors.New("live writes are disabled: explicit acknowledgement and BITUNIX_LIVE_TRADING opt-in are required")
	}
	return newClient(baseURL, apiKey, secret, true, false)
}

func newClient(baseURL, apiKey, secret string, writes, publicOnly bool) (*Client, error) {
	if !publicOnly && (strings.TrimSpace(apiKey) == "" || strings.TrimSpace(secret) == "") {
		return nil, errors.New("Bitunix API key and secret are required")
	}
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	u, err := url.Parse(baseURL)
	if err != nil || u.Host == "" {
		return nil, errors.New("Bitunix base URL must be a valid HTTPS URL")
	}
	isLoopbackHTTP := u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1")
	if u.Scheme != "https" && !isLoopbackHTTP {
		return nil, errors.New("Bitunix base URL must use HTTPS (HTTP is allowed only for loopback tests)")
	}
	// Refuse redirects so signed credentials cannot be forwarded to another origin.
	hc := &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), apiKey: apiKey, secret: secret, http: hc, allowWrites: writes}, nil
}

// ReadOnly reports whether mutating methods are blocked on this client.
func (c *Client) ReadOnly() bool { return !c.allowWrites }

// doPrivate sends one signed request. Read methods can use GET; mutating methods
// are denied unless this client was created via NewLiveClient.
func (c *Client) doPrivate(ctx context.Context, method, path string, query url.Values, body any, out any) error {
	if c.apiKey == "" || c.secret == "" {
		return errors.New("this Bitunix client has no private API credentials")
	}
	if method != http.MethodGet && !c.allowWrites {
		return errors.New("Bitunix write request blocked: construct an explicitly opted-in live client first")
	}
	if !strings.HasPrefix(path, "/api/v1/futures/") || strings.Contains(path, "..") {
		return errors.New("refusing non-futures or invalid Bitunix API path")
	}
	if method != http.MethodGet && method != http.MethodPost {
		return fmt.Errorf("unsupported Bitunix HTTP method %q", method)
	}
	if query == nil {
		query = url.Values{}
	}
	bodyBytes := []byte{}
	if body != nil {
		var err error
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode Bitunix request: %w", err)
		}
	}
	nonce, err := randomNonce()
	if err != nil {
		return fmt.Errorf("create request nonce: %w", err)
	}
	timestamp := fmt.Sprintf("%d", time.Now().UTC().UnixMilli())
	sign := signature(nonce, timestamp, c.apiKey, c.secret, canonicalQuery(query), string(bodyBytes))
	requestURL := c.baseURL + path
	if encoded := query.Encode(); encoded != "" {
		requestURL += "?" + encoded
	}
	req, err := http.NewRequestWithContext(ctx, method, requestURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("create Bitunix request: %w", err)
	}
	req.Header.Set("api-key", c.apiKey)
	req.Header.Set("nonce", nonce)
	req.Header.Set("timestamp", timestamp)
	req.Header.Set("sign", sign)
	req.Header.Set("language", "en-US")
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("Bitunix request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Bitunix returned HTTP %d (response body omitted)", resp.StatusCode)
	}
	var envelope apiEnvelope
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&envelope); err != nil {
		return fmt.Errorf("decode Bitunix response: %w", err)
	}
	if envelope.Code != 0 {
		return fmt.Errorf("Bitunix API error code %d (message omitted)", envelope.Code)
	}
	if out == nil || len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return nil
	}
	if err := json.Unmarshal(envelope.Data, out); err != nil {
		return fmt.Errorf("decode Bitunix response data: %w", err)
	}
	return nil
}

func randomNonce() (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

// canonicalQuery follows Bitunix's documented signing rule: keys sorted in ASCII
// order, then each key immediately followed by its unescaped value (no '=' or '&').
func canonicalQuery(values url.Values) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, key := range keys {
		vals := append([]string(nil), values[key]...)
		sort.Strings(vals)
		for _, value := range vals {
			b.WriteString(key)
			b.WriteString(value)
		}
	}
	return b.String()
}
