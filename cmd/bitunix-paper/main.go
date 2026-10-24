// Command bitunix-paper continuously reads closed Bitunix futures candles and
// prints deterministic multi-timeframe paper signals. It never authenticates to
// the private API or sends orders.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"syscall"
	"time"

	"github.com/yourname/jrock-agent/internal/bitunix"
	"github.com/yourname/jrock-agent/internal/config"
	"github.com/yourname/jrock-agent/internal/trader"
)

func main() {
	once := flag.Bool("once", false, "scan current Bitunix closed candles once and exit")
	flag.Parse()
	cfg := config.Load()
	client, err := bitunix.NewPublicClient(cfg.BitunixBaseURL)
	if err != nil {
		log.Fatal(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	tradeCfg := trader.New().Config
	if *once {
		if err := scanOnce(ctx, client, tradeCfg); err != nil {
			log.Fatal(err)
		}
		return
	}
	log.Printf("Bitunix paper signal runner started: symbols=%v frames=%v interval=%ds; ORDERS DISABLED", tradeCfg.Symbols, tradeCfg.Timeframes, tradeCfg.ScanIntervalSec)
	if err := run(ctx, client, tradeCfg); err != nil && err != context.Canceled {
		log.Fatal(err)
	}
}

func run(ctx context.Context, client *bitunix.Client, cfg trader.Config) error {
	if cfg.ScanIntervalSec < 1 || len(cfg.Symbols) == 0 || len(cfg.Timeframes) == 0 {
		return fmt.Errorf("invalid scan configuration")
	}
	if err := scanOnce(ctx, client, cfg); err != nil {
		log.Printf("scan error: %v", err)
	}
	ticker := time.NewTicker(time.Duration(cfg.ScanIntervalSec) * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := scanOnce(ctx, client, cfg); err != nil {
				log.Printf("scan error: %v", err)
			}
		}
	}
}

func scanOnce(ctx context.Context, client *bitunix.Client, cfg trader.Config) error {
	for _, symbol := range cfg.Symbols {
		frames := make(map[string]trader.FrameSignal, len(cfg.Timeframes))
		prices := make(map[string]string, len(cfg.Timeframes))
		var firstErr error
		for _, interval := range cfg.Timeframes {
			candlesRaw, err := client.GetKlines(ctx, symbol, interval, "LAST_PRICE", 200)
			if err != nil {
				firstErr = fmt.Errorf("%s %s candles: %w", symbol, interval, err)
				break
			}
			candles := make([]trader.Candle, 0, len(candlesRaw))
			for _, row := range candlesRaw {
				open, e1 := strconv.ParseFloat(row.Open, 64)
				high, e2 := strconv.ParseFloat(row.High, 64)
				low, e3 := strconv.ParseFloat(row.Low, 64)
				closePrice, e4 := strconv.ParseFloat(row.Close, 64)
				volume, e5 := strconv.ParseFloat(row.BaseVol, 64)
				if e1 != nil || e2 != nil || e3 != nil || e4 != nil || e5 != nil {
					continue
				}
				candles = append(candles, trader.Candle{Time: row.Time, Open: open, High: high, Low: low, Close: closePrice, BaseVolume: volume})
			}
			sort.Slice(candles, func(i, j int) bool { return candles[i].Time < candles[j].Time })
			signal, err := trader.EvaluateFrame(candles, cfg.MinAgreeingStrategies, float64(cfg.TFMinConfidence))
			if err != nil {
				firstErr = fmt.Errorf("%s %s signal: %w", symbol, interval, err)
				break
			}
			frames[interval] = signal
			if len(candles) > 0 {
				prices[interval] = strconv.FormatFloat(candles[len(candles)-1].Close, 'f', -1, 64)
			}
			// Stay comfortably below Bitunix's public REST rate limit across symbols.
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(120 * time.Millisecond):
			}
		}
		if firstErr != nil {
			return firstErr
		}
		consensus := trader.CombineFrames(frames, cfg.Timeframes, float64(cfg.MinConfidence))
		log.Printf("PAPER_SIGNAL symbol=%s direction=%s confidence=%.1f%% long_frames=%d short_frames=%d frame_signals=%v last_prices=%v orders=DISABLED",
			symbol, directionName(consensus.Direction), consensus.Confidence, consensus.LongFrames, consensus.ShortFrames, frames, prices)
	}
	return nil
}

func directionName(direction int) string {
	switch direction {
	case 1:
		return "LONG"
	case -1:
		return "SHORT"
	default:
		return "NEUTRAL"
	}
}
