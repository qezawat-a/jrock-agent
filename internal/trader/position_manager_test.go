package trader

import (
	"math"
	"testing"
)

func testProtectionConfig() Config {
	return Config{
		BreakevenThresholdPct: 10,
		TrailingTriggerROIPct: 10,
		TrailingStopPct:       5,
		TrailingDistancePct:   3,
		LiqSLDistancePct:      50,
	}
}

func TestInitialStopIsHalfwayBetweenEntryAndLiquidation(t *testing.T) {
	cfg := testProtectionConfig()
	cases := []struct {
		name string
		pos  ManagedPosition
		mark float64
		want float64
	}{
		{"long", ManagedPosition{Symbol: "BTCUSDT", Side: SideLong, EntryPrice: 100, LiquidationPrice: 50, Leverage: 10}, 100, 75},
		{"short", ManagedPosition{Symbol: "BTCUSDT", Side: SideShort, EntryPrice: 100, LiquidationPrice: 150, Leverage: 10}, 100, 125},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, got, err := UpdateProtection(tc.pos, cfg, tc.mark)
			if err != nil {
				t.Fatal(err)
			}
			if got.StopPrice != tc.want {
				t.Fatalf("stop=%v, want %v", got.StopPrice, tc.want)
			}
			if got.StopHit {
				t.Fatal("initial position should not be stopped")
			}
		})
	}
}

func TestBreakevenThresholdMovesStopForBothSides(t *testing.T) {
	cfg := testProtectionConfig()
	cfg.TrailingTriggerROIPct = 1000 // isolate the breakeven threshold from trailing logic
	cases := []struct {
		name string
		pos  ManagedPosition
		mark float64
		want float64
	}{
		{"long", ManagedPosition{Symbol: "BTCUSDT", Side: SideLong, EntryPrice: 100, LiquidationPrice: 50, Leverage: 10}, 101, 100},
		{"short", ManagedPosition{Symbol: "BTCUSDT", Side: SideShort, EntryPrice: 100, LiquidationPrice: 150, Leverage: 10}, 99, 100},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, got, err := UpdateProtection(tc.pos, cfg, tc.mark)
			if err != nil {
				t.Fatal(err)
			}
			if !got.Breakeven || got.StopPrice != tc.want {
				t.Fatalf("breakeven=%t stop=%v, want true/%v", got.Breakeven, got.StopPrice, tc.want)
			}
		})
	}
}

func TestTrailingStopUsesPeakAndNeverLoosens(t *testing.T) {
	cfg := testProtectionConfig()
	long := ManagedPosition{Symbol: "BTCUSDT", Side: SideLong, EntryPrice: 100, LiquidationPrice: 50, Leverage: 10}
	long, first, err := UpdateProtection(long, cfg, 103)
	if err != nil {
		t.Fatal(err)
	}
	if !first.Trailing || math.Abs(first.StopPrice-102.5) > 1e-9 || long.BestPrice != 103 {
		t.Fatalf("unexpected first trailing update: state=%+v update=%+v", long, first)
	}
	long, second, err := UpdateProtection(long, cfg, 101)
	if err != nil {
		t.Fatal(err)
	}
	if second.StopPrice != first.StopPrice || long.BestPrice != 103 {
		t.Fatalf("stop loosened or peak changed on retracement: first=%+v second=%+v state=%+v", first, second, long)
	}

	short := ManagedPosition{Symbol: "BTCUSDT", Side: SideShort, EntryPrice: 100, LiquidationPrice: 150, Leverage: 10}
	short, shortUpdate, err := UpdateProtection(short, cfg, 97)
	if err != nil {
		t.Fatal(err)
	}
	if !shortUpdate.Trailing || math.Abs(shortUpdate.StopPrice-97.5) > 1e-9 || short.BestPrice != 97 {
		t.Fatalf("unexpected short trailing update: state=%+v update=%+v", short, shortUpdate)
	}
}

func TestStopHitAndInputValidation(t *testing.T) {
	cfg := testProtectionConfig()
	pos := ManagedPosition{Symbol: "BTCUSDT", Side: SideLong, EntryPrice: 100, LiquidationPrice: 50, Leverage: 10}
	_, hit, err := UpdateProtection(pos, cfg, 74)
	if err != nil {
		t.Fatal(err)
	}
	if !hit.StopHit {
		t.Fatal("mark below initial stop should trigger the paper stop")
	}

	bad := pos
	bad.LiquidationPrice = 110
	if _, _, err := UpdateProtection(bad, cfg, 100); err == nil {
		t.Fatal("expected invalid long liquidation-price error")
	}
	if _, _, err := UpdateProtection(pos, Config{}, 100); err == nil {
		t.Fatal("expected incomplete protection config error")
	}
}
