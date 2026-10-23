package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClient_Models(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/models" {
			t.Errorf("request = %s %s, want GET /v1/models", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-secret" {
			t.Errorf("Authorization = %q, want bearer token", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"data":[{"id":"model-a"},{"id":"model-b","object":"model"}]}`)
	}))
	defer server.Close()

	client := New(server.URL+"/v1/", "test-secret")
	models, err := client.Models(context.Background())
	if err != nil {
		t.Fatalf("Models() error = %v", err)
	}
	if len(models) != 2 || models[0].ID != "model-a" || models[1].Object != "model" {
		t.Fatalf("Models() = %+v", models)
	}
}

func TestClient_Ask(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/chat/completions" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		var got chatRequest
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		if got.Model != "selected-model" || len(got.Messages) != 2 {
			t.Errorf("chat request = %+v", got)
		}
		if got.Messages[0].Role != "system" || got.Messages[0].Content != "be candid" {
			t.Errorf("system message = %+v", got.Messages[0])
		}
		if got.Messages[1].Role != "user" || got.Messages[1].Content != "question" {
			t.Errorf("user message = %+v", got.Messages[1])
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"answer"}}]}`)
	}))
	defer server.Close()

	client := New(server.URL, "")
	got, err := client.Ask(context.Background(), "selected-model", "be candid", "question")
	if err != nil {
		t.Fatalf("Ask() error = %v", err)
	}
	if got != "answer" {
		t.Fatalf("Ask() = %q, want answer", got)
	}
}

func TestClient_CheckChat(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var got chatRequest
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		if r.URL.Path != "/chat/completions" || got.Model != "m" || len(got.Messages) != 1 || got.Messages[0].Content != "Reply with exactly: OK" {
			t.Errorf("unexpected probe request path=%q body=%+v", r.URL.Path, got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"OK"}}]}`)
	}))
	defer server.Close()
	if err := New(server.URL, "").CheckChat(context.Background(), "m"); err != nil {
		t.Fatalf("CheckChat() error = %v", err)
	}
}

func TestClient_RejectsBadResponses(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		wantErr string
	}{
		{name: "provider error omits response body", status: http.StatusUnauthorized, body: `{"error":"secret diagnostic"}`, wantErr: "HTTP 401"},
		{name: "malformed JSON", status: http.StatusOK, body: "not-json", wantErr: "invalid character"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = fmt.Fprint(w, tt.body)
			}))
			defer server.Close()
			_, err := New(server.URL, "").Models(context.Background())
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Models() error = %v, want containing %q", err, tt.wantErr)
			}
			if strings.Contains(err.Error(), "secret diagnostic") {
				t.Fatalf("provider response body leaked in error: %v", err)
			}
		})
	}
}

func TestClient_AskRejectsEmptyChoices(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"choices":[]}`)
	}))
	defer server.Close()
	_, err := New(server.URL, "").Ask(context.Background(), "m", "system", "prompt")
	if err == nil || !strings.Contains(err.Error(), "no choices") {
		t.Fatalf("Ask() error = %v, want no choices error", err)
	}
}

func TestClient_EmptyBaseURL(t *testing.T) {
	t.Parallel()
	_, err := New("", "").Models(context.Background())
	if err == nil || !strings.Contains(err.Error(), "AI_BASE_URL is empty") {
		t.Fatalf("Models() error = %v", err)
	}
}
