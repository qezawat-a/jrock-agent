package agent

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yourname/jrock-agent/internal/config"
	"github.com/yourname/jrock-agent/internal/provider"
	"github.com/yourname/jrock-agent/internal/trader"
)

func TestMemoryPersistsAtConfiguredPathAndIsIncludedInThink(t *testing.T) {
	memoryPath := filepath.Join(t.TempDir(), "volume", "memory.md")
	app := &App{Cfg: config.Config{}, Engine: trader.New(), MemoryPath: memoryPath}
	if err := app.memory("User prefers Bitunix futures data"); err != nil {
		t.Fatal(err)
	}
	loaded, err := app.loadMemory()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(loaded, "User prefers Bitunix futures data") {
		t.Fatalf("loaded memory=%q", loaded)
	}

	var gotSystem string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("request path=%q", r.URL.Path)
		}
		var request struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		if len(request.Messages) == 0 {
			t.Error("missing chat messages")
		} else {
			gotSystem = request.Messages[0].Content
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`)
	}))
	defer server.Close()
	app.AI = provider.New(server.URL, "test-key")
	app.Model = "test-model"
	if err := app.Handle("/think What data source should I use?"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(gotSystem, "User prefers Bitunix futures data") {
		t.Fatalf("long-term memory missing from model context: %q", gotSystem)
	}
}

func TestLoadMemoryMissingFileIsEmptyAndLargeMemoryIsBounded(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.md")
	app := &App{MemoryPath: path}
	memory, err := app.loadMemory()
	if err != nil || memory != "" {
		t.Fatalf("missing memory=(%q,%v), want empty/no error", memory, err)
	}
	if err := os.WriteFile(path, []byte(strings.Repeat("x", 12010)), 0o600); err != nil {
		t.Fatal(err)
	}
	memory, err = app.loadMemory()
	if err != nil {
		t.Fatal(err)
	}
	if len([]rune(memory)) != 12000 {
		t.Fatalf("memory length=%d runes, want 12000", len([]rune(memory)))
	}
}
