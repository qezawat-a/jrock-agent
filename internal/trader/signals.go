package trader

import (
	"errors"
	"math"
)

type Candle struct {
	Time       int64
	Open       float64
	High       float64
	Low        float64
	Close      float64
	BaseVolume float64
}

type FrameSignal struct {
	Direction   int     // 1 long, -1 short, 0 neutral
	Confidence  float64 // winning indicator votes / non-neutral votes
	LongVotes   int
	ShortVotes  int
	ActiveVotes int
}

type ConsensusSignal struct {
	Direction   int
	Confidence  float64 // aligned frames / requested frames
	LongFrames  int
	ShortFrames int
}

// EvaluateFrame implements the nine indicator vote definitions used by the
// project's Bitunix historical research. It expects chronological closed candles.
func EvaluateFrame(candles []Candle, minAgreeing int, minConfidence float64) (FrameSignal, error) {
	if len(candles) < 50 {
		return FrameSignal{}, errors.New("at least 50 closed candles are required")
	}
	if minAgreeing < 1 || minConfidence <= 0 || minConfidence > 100 {
		return FrameSignal{}, errors.New("invalid frame signal thresholds")
	}
	for i, c := range candles {
		if c.Open <= 0 || c.High <= 0 || c.Low <= 0 || c.Close <= 0 || c.BaseVolume < 0 || c.High < c.Low || c.Time < 0 {
			return FrameSignal{}, errors.New("invalid candle data")
		}
		if i > 0 && c.Time <= candles[i-1].Time {
			return FrameSignal{}, errors.New("candles must be sorted by increasing open time")
		}
	}
	votes := indicatorVotes(candles)
	long, short := 0, 0
	for _, v := range votes {
		if v > 0 {
			long++
		} else if v < 0 {
			short++
		}
	}
	active := long + short
	result := FrameSignal{LongVotes: long, ShortVotes: short, ActiveVotes: active}
	if active == 0 || long == short {
		return result, nil
	}
	winning := long
	if short > long {
		winning = short
		result.Direction = -1
	} else {
		result.Direction = 1
	}
	result.Confidence = 100 * float64(winning) / float64(active)
	if winning < minAgreeing || result.Confidence < minConfidence {
		result.Direction = 0
	}
	return result, nil
}

// CombineFrames requires the configured percentage of requested timeframes to
// agree in the same direction. Neutral frames remain in the denominator.
func CombineFrames(frames map[string]FrameSignal, requested []string, minConfidence float64) ConsensusSignal {
	out := ConsensusSignal{}
	if len(requested) == 0 || minConfidence <= 0 || minConfidence > 100 {
		return out
	}
	for _, frame := range requested {
		switch frames[frame].Direction {
		case 1:
			out.LongFrames++
		case -1:
			out.ShortFrames++
		}
	}
	winning := out.LongFrames
	if out.ShortFrames > winning {
		winning = out.ShortFrames
	}
	if out.LongFrames > out.ShortFrames {
		out.Direction = 1
	} else if out.ShortFrames > out.LongFrames {
		out.Direction = -1
	}
	out.Confidence = 100 * float64(winning) / float64(len(requested))
	if out.Direction == 0 || out.Confidence < minConfidence {
		out.Direction = 0
	}
	return out
}

func indicatorVotes(c []Candle) []int {
	n := len(c)
	close := make([]float64, n)
	open := make([]float64, n)
	high := make([]float64, n)
	low := make([]float64, n)
	volume := make([]float64, n)
	for i := range c {
		close[i], open[i], high[i], low[i], volume[i] = c[i].Close, c[i].Open, c[i].High, c[i].Low, c[i].BaseVolume
	}
	votes := make([]int, 9)
	ema9, ema21 := ema(close, 9), ema(close, 21)
	votes[0] = compare(ema9[n-1], ema21[n-1])
	ma20, ma50 := mean(close[n-20:]), mean(close[n-50:])
	votes[1] = compare(ma20, ma50)

	rsi := rsiWilder(close, 14)
	if rsi < 30 {
		votes[2] = 1
	} else if rsi > 70 {
		votes[2] = -1
	}
	macdLine := subtract(ema(close, 12), ema(close, 26))
	macdSignal := ema(macdLine, 9)
	votes[3] = compare(macdLine[n-1], macdSignal[n-1])
	volMean := mean(volume[n-20:])
	if volume[n-1] > volMean && close[n-1] > open[n-1] {
		votes[4] = 1
	} else if volume[n-1] > volMean && close[n-1] < open[n-1] {
		votes[4] = -1
	}
	momentum := close[n-1] / close[n-11]
	votes[5] = compare(momentum, 1)
	priorHigh, priorLow := max(high[n-21:n-1]), min(low[n-21:n-1])
	if close[n-1] > priorHigh {
		votes[6] = 1
	} else if close[n-1] < priorLow {
		votes[6] = -1
	}
	votes[7] = supertrendDirection(high, low, close)
	mid, sd := mean(close[n-20:]), stdPopulation(close[n-20:])
	if close[n-1] < mid-2*sd {
		votes[8] = 1
	} else if close[n-1] > mid+2*sd {
		votes[8] = -1
	}
	return votes
}

func ema(values []float64, period int) []float64 {
	out := make([]float64, len(values))
	if len(values) == 0 || period <= 0 {
		return out
	}
	alpha := 2 / float64(period+1)
	out[0] = values[0]
	for i := 1; i < len(values); i++ {
		out[i] = alpha*values[i] + (1-alpha)*out[i-1]
	}
	return out
}

func rsiWilder(close []float64, period int) float64 {
	gains, losses := make([]float64, len(close)), make([]float64, len(close))
	for i := 1; i < len(close); i++ {
		delta := close[i] - close[i-1]
		if delta > 0 {
			gains[i] = delta
		} else {
			losses[i] = -delta
		}
	}
	avgGain := emaAlpha(gains, period)
	avgLoss := emaAlpha(losses, period)
	if avgLoss[len(close)-1] == 0 {
		if avgGain[len(close)-1] == 0 {
			return 50
		}
		return 100
	}
	rs := avgGain[len(close)-1] / avgLoss[len(close)-1]
	return 100 - 100/(1+rs)
}

func emaAlpha(values []float64, period int) []float64 {
	out := make([]float64, len(values))
	if len(values) == 0 || period <= 0 {
		return out
	}
	alpha := 1 / float64(period)
	out[0] = values[0]
	for i := 1; i < len(values); i++ {
		out[i] = alpha*values[i] + (1-alpha)*out[i-1]
	}
	return out
}

func supertrendDirection(high, low, close []float64) int {
	n := len(close)
	tr := make([]float64, n)
	for i := range close {
		tr[i] = high[i] - low[i]
		if i > 0 {
			tr[i] = math.Max(tr[i], math.Max(math.Abs(high[i]-close[i-1]), math.Abs(low[i]-close[i-1])))
		}
	}
	atr := emaAlpha(tr, 10)
	if atr[n-1] <= math.Abs(close[n-1])*1e-12 {
		return 0
	}
	finalUp := make([]float64, n)
	finalDown := make([]float64, n)
	dir := make([]int, n)
	for i := range close {
		hl2 := (high[i] + low[i]) / 2
		basicUp, basicDown := hl2+3*atr[i], hl2-3*atr[i]
		finalUp[i], finalDown[i] = basicUp, basicDown
		if i == 0 {
			continue
		}
		if basicUp < finalUp[i-1] || close[i-1] > finalUp[i-1] {
			finalUp[i] = basicUp
		} else {
			finalUp[i] = finalUp[i-1]
		}
		if basicDown > finalDown[i-1] || close[i-1] < finalDown[i-1] {
			finalDown[i] = basicDown
		} else {
			finalDown[i] = finalDown[i-1]
		}
		if dir[i-1] <= 0 {
			if close[i] > finalUp[i] {
				dir[i] = 1
			} else {
				dir[i] = -1
			}
		} else if close[i] < finalDown[i] {
			dir[i] = -1
		} else {
			dir[i] = 1
		}
	}
	return dir[n-1]
}

func compare(a, b float64) int {
	tol := math.Max(math.Abs(a), math.Abs(b))*1e-12 + 1e-14
	if a > b+tol {
		return 1
	}
	if a < b-tol {
		return -1
	}
	return 0
}

func subtract(a, b []float64) []float64 {
	out := make([]float64, len(a))
	for i := range a {
		out[i] = a[i] - b[i]
	}
	return out
}
func mean(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	s := 0.0
	for _, x := range v {
		s += x
	}
	return s / float64(len(v))
}
func stdPopulation(v []float64) float64 {
	m := mean(v)
	ss := 0.0
	for _, x := range v {
		d := x - m
		ss += d * d
	}
	return math.Sqrt(ss / float64(len(v)))
}
func max(v []float64) float64 {
	m := math.Inf(-1)
	for _, x := range v {
		if x > m {
			m = x
		}
	}
	return m
}
func min(v []float64) float64 {
	m := math.Inf(1)
	for _, x := range v {
		if x < m {
			m = x
		}
	}
	return m
}
