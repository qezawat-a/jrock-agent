package trader

import (
	"reflect"
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	t.Parallel()
	e := New()
	if e.Config.MarginMode != "ISOLATION" || e.Config.PositionMode != "ONE_WAY" {
		t.Fatalf("mode defaults = %s/%s, want ISOLATION/ONE_WAY", e.Config.MarginMode, e.Config.PositionMode)
	}
	if e.Config.ScanIntervalSec != 15 || e.Config.GuardIntervalSec != 15 || e.Config.ManageIntervalSec != 15 || e.Config.ReportIntervalSec != 30 {
		t.Fatalf("unexpected interval defaults: %+v", e.Config)
	}
	if e.Config.MinAgreeingStrategies != 3 || e.Config.MinConfidence != 80 || e.Config.TFMinConfidence != 60 {
		t.Fatalf("unexpected signal defaults: %+v", e.Config)
	}
	if !reflect.DeepEqual(e.Config.Timeframes, []string{"1m", "3m", "5m", "15m"}) {
		t.Fatalf("timeframes = %v", e.Config.Timeframes)
	}
}

func TestEngine_Set(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		key  string
		val  string
		want func(Config) bool
	}{
		{name: "scan interval", key: "scan_interval_sec", val: "20", want: func(c Config) bool { return c.ScanIntervalSec == 20 }},
		{name: "confidence", key: "min_confidence", val: "75", want: func(c Config) bool { return c.MinConfidence == 75 }},
		{name: "breakeven float", key: "breakeven_threshold_pct", val: "12.5", want: func(c Config) bool { return c.BreakevenThresholdPct == 12.5 }},
		{name: "margin mode", key: "margin_mode", val: "CROSS", want: func(c Config) bool { return c.MarginMode == "CROSS" }},
		{name: "position mode", key: "position_mode", val: "HEDGE", want: func(c Config) bool { return c.PositionMode == "HEDGE" }},
		{name: "symbols normalized", key: "symbols", val: " btcusdt, ethusdt ", want: func(c Config) bool { return reflect.DeepEqual(c.Symbols, []string{"BTCUSDT", "ETHUSDT"}) }},
		{name: "timeframes normalized", key: "timeframes", val: "1M, 5m, 15M", want: func(c Config) bool { return reflect.DeepEqual(c.Timeframes, []string{"1m", "5m", "15m"}) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			e := New()
			if err := e.Set(tt.key, tt.val); err != nil {
				t.Fatalf("Set(%q, %q) error = %v", tt.key, tt.val, err)
			}
			if !tt.want(e.Config) {
				t.Fatalf("setting not applied: %+v", e.Config)
			}
		})
	}
}

func TestEngine_SetRejectsInvalidWithoutChangingConfig(t *testing.T) {
	t.Parallel()
	tests := []struct{ name, key, val string }{
		{name: "zero interval", key: "scan_interval_sec", val: "0"},
		{name: "non-integer interval", key: "scan_interval_sec", val: "15.5"},
		{name: "confidence above range", key: "min_confidence", val: "101"},
		{name: "too many agreeing indicators", key: "min_agreeing_strategies", val: "10"},
		{name: "invalid mode", key: "position_mode", val: "YOLO"},
		{name: "invalid margin mode", key: "margin_mode", val: "CROSS_MARGIN"},
		{name: "trailing numeric suffix", key: "trailing_stop_pct", val: "5oops"},
		{name: "nan percentage", key: "trailing_distance_pct", val: "NaN"},
		{name: "liquidation gap above 100 percent", key: "liq_sl_distance_pct", val: "101"},
		{name: "trailing distance at 100 percent", key: "trailing_distance_pct", val: "100"},
		{name: "empty symbols", key: "symbols", val: " , "},
		{name: "empty timeframes", key: "timeframes", val: ""},
		{name: "unsupported timeframe", key: "timeframes", val: "1m,2m"},
		{name: "unknown setting", key: "made_up", val: "1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			e := New()
			before := e.Config
			if err := e.Set(tt.key, tt.val); err == nil {
				t.Fatalf("Set(%q, %q) unexpectedly succeeded", tt.key, tt.val)
			}
			if !reflect.DeepEqual(e.Config, before) {
				t.Fatalf("invalid setting mutated config: before=%+v after=%+v", before, e.Config)
			}
		})
	}
}

func TestEngine_StatusReportsPaperModeAndSettings(t *testing.T) {
	t.Parallel()
	status := New().Status()
	for _, want := range []string{"mode=PAPER", "margin=ISOLATION", "position=ONE_WAY", "scan=15s", "report=30s", "confidence=80"} {
		if !strings.Contains(status, want) {
			t.Errorf("Status() = %q, missing %q", status, want)
		}
	}
}
