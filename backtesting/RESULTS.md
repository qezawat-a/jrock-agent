# Bitunix futures backtests — 2026-10-23

The earlier Yahoo Finance BTC-USD results were removed: they were not Bitunix futures data and should not be used to evaluate this project.

## Best available 30-day run: 15m signals

The baseline used Bitunix BTCUSDT `LAST_PRICE` futures candles and all nine requested indicator families, with execution decisions on 15m closed candles. Bitunix's public K-line endpoint documents a 200-candle maximum per request ([official K-line documentation](https://www.bitunix.com/api-docs/futures/market/get_kline.html)). In this run, 15m data covered 29.97 days. The exchange returned much shorter 1m/5m histories, so this run deliberately uses **15m only** and does **not** test the requested 1m/3m/5m/15m confirmation.

| Metric | Result |
|---|---:|
| Starting equity | $10,000 |
| Ending equity | $8,740.80 |
| Total return | **−12.59%** |
| Max drawdown | −15.75% |
| Trades | 295 |
| Win rate | 27.46% |
| Profit factor | 0.70 |
| Funding PnL | +$3.45 |
| Trading fees / slippage | 0 modeled (pre-cost result) |

This baseline lost money before trading fees and slippage. Adding actual Bitunix trading costs would likely worsen the result. The 30-day 15m result is more informative than the two-day multi-timeframe diagnostic below, but 30 days is still a short evaluation period.

## Four-timeframe pipeline diagnostic (not a strategy evaluation)

A separate run requested 30 days across 1m/3m/5m/15m. Actual overlapping history was only **2.07 days**: the API returned 3,800 1m candles (~2.64 days), 14,399 3m (~30 days), 600 5m (~2.08 days), and 2,879 15m (~30 days). The 2-day four-timeframe result (+1.11% across 12 trades) is too short to assess and annualized metrics were suppressed. It only demonstrates that the Bitunix candle/funding adapter and signal pipeline execute.

## Explicit strategy assumptions

- EMA 9/21; SMA 20/50; RSI 14 (long vote below 30, short above 70); MACD 12/26/9; volume above SMA20 with candle direction; 10-bar momentum; breakout above/below the preceding 20-bar range; Supertrend ATR10 ×3; Bollinger 20 ×2 (contrarian votes outside the bands).
- A timeframe needs at least 3 same-direction indicator votes and at least 60% agreement among non-neutral votes. Overall confidence is interpreted as aligned selected timeframes / number of selected timeframes, with an 80% threshold. This is an explicit baseline definition, not a Bitunix definition or AI confidence score.
- Closed candles only; signal enters at the next 15m open; one position at a time; 1× notional; reverse qualified signal closes/reverses. The four-frame test requires all four frames to align at the 80% threshold.
- Historical Bitunix funding settlements were included, using the returned mark price. Fees and slippage are 0 because account tier/fill assumptions were not supplied. Results are idealized and before fees; no leverage, liquidation/tiered margin, spread, partial fills, exchange precision, or stop-loss/TP execution is simulated.

These are research prototypes—not a reproduction of the live bot and not financial advice. To validate the actual multi-timeframe engine over a useful period, provide/export longer Bitunix 1m and 5m candle history, and provide your taker/maker fee tier and slippage assumption. The report, equity series, trades, and candles are in `reports/bitunix/`; runnable prototype: `bitunix_backtest.py`.
