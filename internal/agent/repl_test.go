package agent

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/yourname/jrock-agent/internal/provider"
	"github.com/yourname/jrock-agent/internal/trader"
)

func TestApp_HandleTraderStateCommands(t *testing.T) {
	tests := []struct {
		name    string
		command string
		check   func(*trader.Engine) bool
	}{
		{name: "engine on enables paper components", command: "/engine_on", check: func(e *trader.Engine) bool { return e.On && e.ScannerOn && e.TraderOn && !e.AutoTrade }},
		{name: "engine off disables all components", command: "/engine_off", check: func(e *trader.Engine) bool { return !e.On && !e.ScannerOn && !e.TraderOn && !e.AutoTrade }},
		{name: "report on toggles report flag", command: "/report_on", check: func(e *trader.Engine) bool { return e.ReportOn }},
		{name: "report off clears report flag", command: "/report_off", check: func(e *trader.Engine) bool { return !e.ReportOn }},
		{name: "live trading command does not enable autotrade", command: "/autotrade_on", check: func(e *trader.Engine) bool { return !e.AutoTrade }},
		{name: "stop trading disables autotrade", command: "/stop_trading", check: func(e *trader.Engine) bool { return !e.AutoTrade }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &App{Engine: trader.New()}
			if tt.command == "/stop_trading" {
				app.Engine.AutoTrade = true
			}
			if err := app.Handle(tt.command); err != nil {
				t.Fatalf("Handle(%q) error = %v", tt.command, err)
			}
			if !tt.check(app.Engine) {
				t.Fatalf("command %q left unexpected state: %+v", tt.command, app.Engine)
			}
		})
	}
}

func TestApp_HandleSettings(t *testing.T) {
	app := &App{Engine: trader.New()}
	if err := app.Handle("/settings margin_mode CROSS"); err != nil {
		t.Fatalf("Handle settings: %v", err)
	}
	if app.Engine.Config.MarginMode != "CROSS" {
		t.Fatalf("margin mode = %q, want CROSS", app.Engine.Config.MarginMode)
	}
	if err := app.Handle("/settings timeframes 1m,5m"); err != nil {
		t.Fatalf("Handle timeframes: %v", err)
	}
	if got := strings.Join(app.Engine.Config.Timeframes, ","); got != "1m,5m" {
		t.Fatalf("timeframes = %q", got)
	}
}

func TestApp_AutoSetModelByKeyCommand(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/models" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("missing configured key authorization")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"data":[{"id":"auto-model"}]}`)
	}))
	defer server.Close()

	app := &App{Engine: trader.New(), AI: provider.New(server.URL+"/v1", "test-key")}
	if err := app.Handle("/auto_set_model_by_key"); err != nil {
		t.Fatal(err)
	}
	if app.Model != "auto-model" || app.Cfg.Model != "auto-model" {
		t.Fatalf("model was not saved in app state: Model=%q Cfg.Model=%q", app.Model, app.Cfg.Model)
	}
}

func TestApp_RunAutoSelectsModelWhenKeyExistsAndModelIsUnset(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			t.Errorf("request path=%q, want /models", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"data":[{"id":"startup-model"}]}`)
	}))
	defer server.Close()

	readEnd, writeEnd, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := writeEnd.Close(); err != nil {
		t.Fatal(err)
	}
	originalStdin := os.Stdin
	os.Stdin = readEnd
	defer func() {
		os.Stdin = originalStdin
		_ = readEnd.Close()
	}()

	app := &App{Engine: trader.New(), AI: provider.New(server.URL, "key")}
	app.Run()
	if app.Model != "startup-model" {
		t.Fatalf("startup auto-selected model=%q, want startup-model", app.Model)
	}
}

func TestApp_AutoSetModelByKeyRequiresConfiguredKey(t *testing.T) {
	app := &App{Engine: trader.New(), AI: provider.New("https://example.invalid/v1", "")}
	if err := app.Handle("/auto_set_model_by_key"); err == nil {
		t.Fatal("expected missing-key error")
	}
}

func TestApp_HandleRejectsUnknownAndInvalidCommands(t *testing.T) {
	app := &App{Engine: trader.New()}
	before := app.Engine.Config
	for _, command := range []string{"/not_a_command", "/settings position_mode YOLO", "/min_confidence 101"} {
		if err := app.Handle(command); err == nil {
			t.Errorf("Handle(%q) unexpectedly succeeded", command)
		}
	}
	if app.Engine.Config.MarginMode != before.MarginMode || app.Engine.Config.PositionMode != before.PositionMode || app.Engine.Config.MinConfidence != before.MinConfidence {
		t.Fatalf("invalid commands changed config: before=%+v after=%+v", before, app.Engine.Config)
	}
}
