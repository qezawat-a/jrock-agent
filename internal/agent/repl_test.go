package agent

import (
	"strings"
	"testing"

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
