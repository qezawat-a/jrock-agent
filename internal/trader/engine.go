// Package trader is a paper-only strategy/configuration scaffold. It intentionally
// contains no exchange order placement or credential signing implementation.
package trader

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

type Config struct {
	Symbols                                                                                              []string
	MarginMode, PositionMode                                                                             string
	MaxPositions                                                                                         int
	ScanIntervalSec, GuardIntervalSec, ManageIntervalSec, ReportIntervalSec                              int
	MinAgreeingStrategies, MinConfidence, TFMinConfidence                                                int
	Timeframes                                                                                           []string
	BreakevenThresholdPct, TrailingTriggerROIPct, TrailingStopPct, TrailingDistancePct, LiqSLDistancePct float64
	Indicators                                                                                           []string
}

type Engine struct {
	On, AutoTrade, ReportOn, ScannerOn, TraderOn bool
	Config                                       Config
}

func New() *Engine {
	return &Engine{Config: Config{
		Symbols: []string{"BTCUSDT"}, MarginMode: "ISOLATION", PositionMode: "ONE_WAY", MaxPositions: 1,
		ScanIntervalSec: 15, GuardIntervalSec: 15, ManageIntervalSec: 15, ReportIntervalSec: 30,
		MinAgreeingStrategies: 3, MinConfidence: 80, TFMinConfidence: 60,
		Timeframes:            []string{"1m", "3m", "5m", "15m"},
		BreakevenThresholdPct: 10, TrailingTriggerROIPct: 10, TrailingStopPct: 5,
		TrailingDistancePct: 3, LiqSLDistancePct: 50,
		Indicators: []string{"EMA", "MA", "RSI", "MACD", "VOLUME", "MOMENTUM", "ATR_BREAKOUT", "SUPERTREND", "BOLLINGER"},
	}}
}

func (e *Engine) Status() string {
	return fmt.Sprintf("mode=PAPER | engine=%t scanner=%t trader=%t autotrade=%t reports=%t | symbols=%v | margin=%s position=%s maxPositions=%d | scan=%ds guard=%ds management=%ds report=%ds | minAgree=%d confidence=%d tfConfidence=%d | frames=%v | indicators=%v",
		e.On, e.ScannerOn, e.TraderOn, e.AutoTrade, e.ReportOn, e.Config.Symbols,
		e.Config.MarginMode, e.Config.PositionMode, e.Config.MaxPositions,
		e.Config.ScanIntervalSec, e.Config.GuardIntervalSec, e.Config.ManageIntervalSec,
		e.Config.ReportIntervalSec, e.Config.MinAgreeingStrategies, e.Config.MinConfidence,
		e.Config.TFMinConfidence, e.Config.Timeframes, e.Config.Indicators)
}

func (e *Engine) Set(key, val string) error {
	var n int
	var err error
	switch key {
	case "scan_interval_sec", "guard_interval_sec", "management_interval_sec", "report_interval_sec", "min_agreeing_strategies", "min_confidence", "tf_min_confidence", "max_positions":
		n, err = strconv.Atoi(val)
		if err != nil || n < 1 {
			return fmt.Errorf("%s requires a positive integer", key)
		}
		switch key {
		case "scan_interval_sec":
			e.Config.ScanIntervalSec = n
		case "guard_interval_sec":
			e.Config.GuardIntervalSec = n
		case "management_interval_sec":
			e.Config.ManageIntervalSec = n
		case "report_interval_sec":
			e.Config.ReportIntervalSec = n
		case "min_agreeing_strategies":
			if n > len(e.Config.Indicators) {
				return fmt.Errorf("min_agreeing_strategies cannot exceed the number of configured indicators (%d)", len(e.Config.Indicators))
			}
			e.Config.MinAgreeingStrategies = n
		case "min_confidence":
			if n > 100 {
				return fmt.Errorf("confidence must be 1-100")
			}
			e.Config.MinConfidence = n
		case "tf_min_confidence":
			if n > 100 {
				return fmt.Errorf("confidence must be 1-100")
			}
			e.Config.TFMinConfidence = n
		case "max_positions":
			e.Config.MaxPositions = n
		}
	case "breakeven_threshold_pct", "trailing_trigger_roi_pct", "trailing_stop_pct", "trailing_distance_pct", "liq_sl_distance_pct":
		f, parseErr := strconv.ParseFloat(val, 64)
		if parseErr != nil || math.IsNaN(f) || math.IsInf(f, 0) || f <= 0 {
			return fmt.Errorf("%s requires a finite positive number", key)
		}
		if key == "liq_sl_distance_pct" && f > 100 {
			return fmt.Errorf("liq_sl_distance_pct must be <=100")
		}
		if key == "trailing_distance_pct" && f >= 100 {
			return fmt.Errorf("trailing_distance_pct must be <100")
		}
		switch key {
		case "breakeven_threshold_pct":
			e.Config.BreakevenThresholdPct = f
		case "trailing_trigger_roi_pct":
			e.Config.TrailingTriggerROIPct = f
		case "trailing_stop_pct":
			e.Config.TrailingStopPct = f
		case "trailing_distance_pct":
			e.Config.TrailingDistancePct = f
		case "liq_sl_distance_pct":
			e.Config.LiqSLDistancePct = f
		}
	case "symbols":
		symbols := splitCSV(val)
		if len(symbols) == 0 {
			return fmt.Errorf("at least one symbol is required")
		}
		e.Config.Symbols = symbols
	case "timeframes":
		frames := splitCSV(val)
		if len(frames) == 0 {
			return fmt.Errorf("at least one timeframe is required")
		}
		canonical := make([]string, 0, len(frames))
		for _, frame := range frames {
			switch frame {
			case "1M":
				canonical = append(canonical, "1m")
			case "3M":
				canonical = append(canonical, "3m")
			case "5M":
				canonical = append(canonical, "5m")
			case "15M":
				canonical = append(canonical, "15m")
			default:
				return fmt.Errorf("unsupported timeframe %q; use 1m, 3m, 5m, or 15m", frame)
			}
		}
		e.Config.Timeframes = canonical
	case "margin_mode":
		if val != "ISOLATION" && val != "CROSS" {
			return fmt.Errorf("margin_mode must be ISOLATION or CROSS")
		}
		e.Config.MarginMode = val
	case "position_mode":
		if val != "HEDGE" && val != "ONE_WAY" {
			return fmt.Errorf("position_mode must be HEDGE or ONE_WAY")
		}
		e.Config.PositionMode = val
	default:
		return fmt.Errorf("unknown setting %q", key)
	}
	return nil
}

func splitCSV(s string) []string {
	out := []string{}
	for _, v := range strings.Split(s, ",") {
		v = strings.ToUpper(strings.TrimSpace(v))
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}
