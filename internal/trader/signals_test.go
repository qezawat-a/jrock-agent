package trader

import "testing"

func TestEvaluateFrameFlatCandlesAreNeutral(t *testing.T) {
	candles := make([]Candle, 60)
	for i := range candles {
		candles[i] = Candle{Time: int64(i + 1), Open: 100, High: 100, Low: 100, Close: 100, BaseVolume: 1}
	}
	got, err := EvaluateFrame(candles, 3, 60)
	if err != nil {
		t.Fatal(err)
	}
	if got.Direction != 0 || got.ActiveVotes != 0 {
		t.Fatalf("flat-market signal=%+v, want neutral", got)
	}
}

func TestEvaluateFrameFindsStrongRisingTrend(t *testing.T) {
	candles := make([]Candle, 60)
	for i := range candles {
		closePrice := 100 + float64(i)
		volume := 1.0
		if i == len(candles)-1 {
			volume = 10
		}
		candles[i] = Candle{Time: int64(i + 1), Open: closePrice - 0.1, High: closePrice + 0.2, Low: closePrice - 0.2, Close: closePrice, BaseVolume: volume}
	}
	got, err := EvaluateFrame(candles, 3, 60)
	if err != nil {
		t.Fatal(err)
	}
	if got.Direction != 1 || got.LongVotes < 3 || got.Confidence < 60 {
		t.Fatalf("rising trend signal=%+v, want qualified long", got)
	}
}

func TestEvaluateFrameRejectsUnclosedOrInvalidInput(t *testing.T) {
	if _, err := EvaluateFrame(make([]Candle, 49), 3, 60); err == nil {
		t.Fatal("expected insufficient-candle error")
	}
	candles := make([]Candle, 50)
	for i := range candles {
		candles[i] = Candle{Time: int64(i + 1), Open: 100, High: 101, Low: 99, Close: 100, BaseVolume: 1}
	}
	candles[30].Time = candles[29].Time
	if _, err := EvaluateFrame(candles, 3, 60); err == nil {
		t.Fatal("expected unsorted-candle error")
	}
}

func TestCombineFramesUsesAllRequestedFramesInDenominator(t *testing.T) {
	frames := map[string]FrameSignal{
		"1m":  {Direction: 1},
		"3m":  {Direction: 1},
		"5m":  {Direction: 0},
		"15m": {Direction: 0},
	}
	requested := []string{"1m", "3m", "5m", "15m"}
	got := CombineFrames(frames, requested, 80)
	if got.Direction != 0 || got.Confidence != 50 {
		t.Fatalf("consensus=%+v, want neutral at 50%%", got)
	}
	frames["5m"], frames["15m"] = FrameSignal{Direction: 1}, FrameSignal{Direction: 1}
	got = CombineFrames(frames, requested, 80)
	if got.Direction != 1 || got.Confidence != 100 {
		t.Fatalf("consensus=%+v, want long at 100%%", got)
	}
}
