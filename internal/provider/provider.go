package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	BaseURL, APIKey string
	HTTP            *http.Client
}
type Model struct {
	ID     string `json:"id"`
	Object string `json:"object,omitempty"`
}
type modelReply struct {
	Data []Model `json:"data"`
}
type chatRequest struct {
	Model       string    `json:"model"`
	Messages    []message `json:"messages"`
	Temperature float64   `json:"temperature,omitempty"`
}
type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
type chatReply struct {
	Choices []struct {
		Message message `json:"message"`
	} `json:"choices"`
}

func New(base, key string) *Client {
	return &Client{strings.TrimRight(base, "/"), key, &http.Client{Timeout: 25 * time.Second}}
}
func (c *Client) do(ctx context.Context, method, path string, body any, out any) error {
	if c.BaseURL == "" {
		return errors.New("AI_BASE_URL is empty")
	}
	var b []byte
	var err error
	if body != nil {
		b, err = json.Marshal(body)
		if err != nil {
			return err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("provider returned HTTP %d (response body omitted to avoid leaking details)", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// Models queries the OpenAI-compatible models endpoint; it does not assume that every
// listed model supports chat completion. /check_ai can test a selected model separately.
func (c *Client) Models(ctx context.Context) ([]Model, error) {
	var r modelReply
	err := c.do(ctx, http.MethodGet, "/models", nil, &r)
	return r.Data, err
}
func (c *Client) CheckChat(ctx context.Context, model string) error {
	var r chatReply
	return c.do(ctx, http.MethodPost, "/chat/completions", chatRequest{Model: model, Messages: []message{{Role: "user", Content: "Reply with exactly: OK"}}, Temperature: 0}, &r)
}
func (c *Client) Ask(ctx context.Context, model, system, prompt string) (string, error) {
	var r chatReply
	err := c.do(ctx, http.MethodPost, "/chat/completions", chatRequest{Model: model, Messages: []message{{Role: "system", Content: system}, {Role: "user", Content: prompt}}}, &r)
	if err != nil {
		return "", err
	}
	if len(r.Choices) == 0 {
		return "", errors.New("provider response contained no choices")
	}
	return r.Choices[0].Message.Content, nil
}
