package trader

import (
	"errors"
	"fmt"
	"math"
)

// PositionSide is the direction of a paper position. This manager deliberately
// does not submit exchange orders; it is a deterministic calculation layer.
type PositionSide string

const (
	SideLong  PositionSide = "LONG"
	SideShort PositionSide = "SHORT"
)

// ManagedPosition is the minimum state needed to calculate a paper protective
// stop. Prices must use the same contract price basis (e.g. Mark Price).
type ManagedPosition struct {
	Symbol           string
	Side             PositionSide
	EntryPrice       float64
	LiquidationPrice float64
	Leverage         int
	CurrentStop      float64
	BestPrice        float64
}

type ProtectionUpdate struct {
	StopPrice  float64
	ROIPct     float64
	PeakROIPct float64
	Breakeven  bool
	Trailing   bool
	StopHit    bool
}

// UpdateProtection applies a transparent PAPER-only policy:
//   - initial stop is LiqSLDistancePct of the entry-to-liquidation price gap
//     measured away from the liquidation price (50% means midpoint);
//   - ROI is unlevered price return multiplied by leverage, before fees/funding;
//   - after the breakeven ROI threshold, stop cannot be worse than entry;
//   - after the trailing ROI trigger, it trails the favorable extreme using both
//     an ROI lock-back and a price-distance stop, choosing the tighter one;
//   - stops only move in the protective direction.
//
// This is not connected to Bitunix and must not be used as an exchange guarantee.
// In particular, liquidation estimates and cross-margin behaviour are external.
func UpdateProtection(pos ManagedPosition, cfg Config, markPrice float64) (ManagedPosition, ProtectionUpdate, error) {
	if err := validateProtectionInputs(pos, cfg, markPrice); err != nil {
		return pos, ProtectionUpdate{}, err
	}

	if pos.BestPrice == 0 {
		pos.BestPrice = pos.EntryPrice
	}
	if pos.Side == SideLong && markPrice > pos.BestPrice || pos.Side == SideShort && markPrice < pos.BestPrice {
		pos.BestPrice = markPrice
	}

	roi := positionROI(pos.EntryPrice, markPrice, pos.Leverage, pos.Side)
	peakROI := positionROI(pos.EntryPrice, pos.BestPrice, pos.Leverage, pos.Side)
	stop := pos.CurrentStop
	if stop == 0 {
		gapPct := cfg.LiqSLDistancePct / 100
		if pos.Side == SideLong {
			stop = pos.LiquidationPrice + (pos.EntryPrice-pos.LiquidationPrice)*gapPct
		} else {
			stop = pos.LiquidationPrice - (pos.LiquidationPrice-pos.EntryPrice)*gapPct
		}
	}

	breakeven := roi >= cfg.BreakevenThresholdPct || (pos.Side == SideLong && stop >= pos.EntryPrice) || (pos.Side == SideShort && stop <= pos.EntryPrice)
	if roi >= cfg.BreakevenThresholdPct {
		if pos.Side == SideLong && stop < pos.EntryPrice {
			stop = pos.EntryPrice
		} else if pos.Side == SideShort && stop > pos.EntryPrice {
			stop = pos.EntryPrice
		}
	}

	trailing := peakROI >= cfg.TrailingTriggerROIPct
	if trailing {
		roiLock := peakROI - cfg.TrailingStopPct
		if roiLock < 0 {
			roiLock = 0
		}
		roiStop := priceAtROI(pos.EntryPrice, roiLock, pos.Leverage, pos.Side)
		var distanceStop float64
		if pos.Side == SideLong {
			distanceStop = pos.BestPrice * (1 - cfg.TrailingDistancePct/100)
			stop = math.Max(stop, math.Max(roiStop, distanceStop))
		} else {
			distanceStop = pos.BestPrice * (1 + cfg.TrailingDistancePct/100)
			stop = math.Min(stop, math.Min(roiStop, distanceStop))
		}
	}

	// Never loosen a stop when an update arrives with a worse mark or stale data.
	if pos.CurrentStop != 0 {
		if pos.Side == SideLong {
			stop = math.Max(stop, pos.CurrentStop)
		} else {
			stop = math.Min(stop, pos.CurrentStop)
		}
	}
	pos.CurrentStop = stop
	stopHit := (pos.Side == SideLong && markPrice <= stop) || (pos.Side == SideShort && markPrice >= stop)
	return pos, ProtectionUpdate{StopPrice: stop, ROIPct: roi, PeakROIPct: peakROI, Breakeven: breakeven, Trailing: trailing, StopHit: stopHit}, nil
}

func validateProtectionInputs(pos ManagedPosition, cfg Config, markPrice float64) error {
	finitePositive := func(v float64) bool { return v > 0 && !math.IsNaN(v) && !math.IsInf(v, 0) }
	if pos.Symbol == "" {
		return errors.New("symbol is required")
	}
	if pos.Side != SideLong && pos.Side != SideShort {
		return errors.New("side must be LONG or SHORT")
	}
	if !finitePositive(pos.EntryPrice) || !finitePositive(markPrice) || pos.Leverage < 1 {
		return errors.New("entry price and mark price must be positive finite values and leverage must be positive")
	}
	if !finitePositive(pos.LiquidationPrice) {
		return errors.New("a positive finite liquidation price is required for this paper calculation")
	}
	if pos.Side == SideLong && pos.LiquidationPrice >= pos.EntryPrice {
		return fmt.Errorf("long liquidation price must be below entry price")
	}
	if pos.Side == SideShort && pos.LiquidationPrice <= pos.EntryPrice {
		return fmt.Errorf("short liquidation price must be above entry price")
	}
	if cfg.LiqSLDistancePct <= 0 || cfg.LiqSLDistancePct > 100 || math.IsNaN(cfg.LiqSLDistancePct) || math.IsInf(cfg.LiqSLDistancePct, 0) {
		return errors.New("liquidation stop distance must be >0 and <=100 percent of the entry-to-liquidation gap")
	}
	for name, value := range map[string]float64{
		"breakeven threshold":  cfg.BreakevenThresholdPct,
		"trailing trigger ROI": cfg.TrailingTriggerROIPct,
		"trailing stop":        cfg.TrailingStopPct,
		"trailing distance":    cfg.TrailingDistancePct,
	} {
		if value <= 0 || math.IsNaN(value) || math.IsInf(value, 0) {
			return fmt.Errorf("%s must be a positive finite percentage", name)
		}
	}
	if pos.CurrentStop < 0 || math.IsNaN(pos.CurrentStop) || math.IsInf(pos.CurrentStop, 0) || pos.BestPrice < 0 || math.IsNaN(pos.BestPrice) || math.IsInf(pos.BestPrice, 0) {
		return errors.New("existing stop and best price must be finite and non-negative")
	}
	return nil
}

func positionROI(entry, price float64, leverage int, side PositionSide) float64 {
	direction := 1.0
	if side == SideShort {
		direction = -1
	}
	return direction * (price - entry) / entry * float64(leverage) * 100
}

func priceAtROI(entry, roiPct float64, leverage int, side PositionSide) float64 {
	move := roiPct / 100 / float64(leverage)
	if side == SideLong {
		return entry * (1 + move)
	}
	return entry * (1 - move)
}
