package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAutoSetModelByKeySelectsFirstUsableModelFromAuthenticatedList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/models" {
			t.Errorf("request = %s %s, want GET /v1/models", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer locally-configured-key" {
			t.Errorf("authorization = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"data":[{"id":"  "},{"id":"first-available"},{"id":"second"}]}`)
	}))
	defer server.Close()

	got, err := New(server.URL+"/v1", "locally-configured-key").AutoSetModelByKey(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got != "first-available" {
		t.Fatalf("AutoSetModelByKey() = %q, want first-available", got)
	}
}

func TestAutoSetModelByKeyRejectsEmptyModelList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"data":[{}, {"id":" "}]}`)
	}))
	defer server.Close()

	if _, err := New(server.URL, "key").AutoSetModelByKey(context.Background()); err == nil {
		t.Fatal("expected error when provider returns no usable model IDs")
	}
}
