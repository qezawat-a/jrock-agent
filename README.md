# J+rock Agent (Go starter)

A constrained, local Go CLI scaffold for a candid AI coding assistant and paper-trading configuration shell. It is **not** the full autonomous system described in the request and does not run a live strategy. A separately gated Bitunix REST client is included; its signed private requests have only been tested against local mocks, not your account. Read [`docs/SAFETY.md`](docs/SAFETY.md) before using it.

## Requirements and run

- Go 1.22+
- Optional OpenAI-compatible provider URL/API key for `/models`, `/check_ai`, and `/think`

```sh
cp .env.example .env
# edit .env locally; never commit keys
go run ./cmd/jrock
# For Railway/live public signals only (no orders):
go run ./cmd/bitunix-paper
```

Commands begin with `/`; use `/help`. `AI_BASE_URL`, `AI_API_KEY`, and `AI_MODEL` may be set in process environment or `.env`. If `AI_API_KEY` is configured and `AI_MODEL` is blank, startup automatically calls `GET {base}/models` and selects the first non-empty model ID; retry any time with `/auto_set_model_by_key` (alias `/auto_set_model`). This uses the key to query the configured provider, not to infer a model offline. It does not send a chat probe or incur chat-completion usage. Run `/check_ai` to verify the selected model supports chat completions. `/models` lists available IDs; manual selection remains available with `/set_model ID`. Provider error response bodies are not printed to avoid leaking secrets or private server details.

## Current implementation

- Model discovery, model selection, connectivity probe, and simple chat completions for OpenAI-compatible APIs.
- Long-term memory: `/memory NOTE` appends persistent notes to `MEMORY_PATH` (default `data/memory.md`), and `/think` includes the most recent 12,000 characters as reference context. For Railway persistence, attach a Volume mounted at `/app/data` and set `MEMORY_PATH=/app/data/memory.md`; without a Volume, Railway's service filesystem is ephemeral and notes may be lost on redeploy.
- Paper-trader settings and command flags with basic validation; default indicators are EMA, MA, RSI, MACD, VOLUME, MOMENTUM, ATR_BREAKOUT, SUPERTREND, BOLLINGER. Requested timeframes and basic thresholds are represented as config.
- `/check_system` and `/verify` run `go test ./...` and `go vet ./...`.
- A separate Bitunix REST package in `internal/bitunix`: SHA-256 request signing; public trading-pair metadata; signed account, position, history, leverage/mode, order, and TP/SL methods. Writes are blocked by default. `NewLiveClient` requires both `BITUNIX_LIVE_TRADING=I_ACKNOWLEDGE_REAL_TRADING` and an explicit call-site acknowledgement; this is an API-library gate, not an autonomous/live strategy loop.
- Bitunix order-unit conversion using the supplied Help Center article: Nominal Value → `qty = notional/price`; Cost Value → `qty = cost × leverage/price`; Quantity is base-asset `qty`. Quantity is rounded down to `basePrecision` and checked against the returned minimum/maximum volumes, symbol status, API support, and leverage limits. Trading fees are not included in Cost Value.
- Low-level Bitunix WebSocket transport in `internal/bitunix/websocket.go`: public connect, private signed login (Unix seconds), subscribe/unsubscribe, JSON heartbeat, private balance/position/order subscriptions, and incoming message reads. URLs are configurable as `BITUNIX_WS_PUBLIC` and `BITUNIX_WS_PRIVATE`; private TP/SL (`tpsl`) is automatically pushed after successful login per docs. Client writes are spaced to respect Bitunix's 5-message/second limit. Local mock tests cover the protocol; reconnect/reconciliation is not implemented.
- `go run ./cmd/bitunix-paper` is a continuous Bitunix **public market-data** signal runner: it fetches closed candles, calculates nine indicator votes and multi-timeframe consensus, and logs `PAPER_SIGNAL ... orders=DISABLED`. `go run ./cmd/bitunix-paper --once` performs one scan and exits. It never authenticates to the private API or sends orders. In Railway, this is the safe worker command to see live market signals after deploying the updated source.
- `go run ./cmd/bitunix-check` performs a signed **read-only** account check after API credentials are configured locally. It never changes account settings or sends orders.
- API documentation index in [`docs/BITUNIX_SOURCES.md`](docs/BITUNIX_SOURCES.md).
- Separate Bitunix futures historical-candle research prototype in [`backtesting/bitunix_backtest.py`](backtesting/bitunix_backtest.py), plus reports in [`backtesting/RESULTS.md`](backtesting/RESULTS.md). It is Python, public-data-only, and not connected to the Go agent or live trading. Requires `pandas`, `numpy`, and `requests`: `python -m pip install pandas numpy requests`. Run, for example: `python backtesting/bitunix_backtest.py --symbol BTCUSDT --days 30 --timeframes 15m`. The four-timeframe default is `--timeframes 1m,3m,5m,15m`; the API returned only 2.07 days of overlapping history in the current run.

Not implemented/wired into live trading: portfolio risk limits, order sizing policy for a real account, private-position reconciliation loop, live trailing/breakeven order updates, TP/SL confirmation, WebSocket reconnect/state reconciliation, and live PnL/reporting. The safe paper signal runner does not place orders and is not connected to the REPL trader flags. The Bitunix REST/WebSocket writers are not called by that runner. No authenticated private WebSocket or live order has been tested against a real account.

## Editing and approvals

`AUTO_ACCEPT_EDITS` is present in `.env.example` but deliberately does not grant permissions: this scaffold has no source-edit tool. To build that later, use reviewed unified diffs, path allowlists, snapshots/backups, tests, and rollback. Treat auto-accept as a narrowly scoped policy, not permission for arbitrary shell/device access. Never expose a general-purpose terminal or filesystem to a model with exchange credentials.

## Go tests

```sh
go test ./...
go test -race ./...
go vet ./...
```

Test audit and coverage notes are in [`docs/GO_TESTING_AUDIT.md`](docs/GO_TESTING_AUDIT.md). Bitunix private REST/WS behavior is tested with local mocks; opt-in real-exchange tests read public symbol metadata and public candles only. No private WebSocket session or live order has been tested.

## GitHub

The user supplied `https://github.com/qezawat-a/jrock-agent`. The current workspace changes have not been pushed to that repository. Never paste a PAT into chat; use a local GitHub credential manager/CLI to publish reviewed changes.

