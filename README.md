# J+rock Agent (Go starter)

A constrained, local Go CLI scaffold for a candid AI coding assistant and paper-trading configuration shell. It is **not** the full autonomous system described in the request and does not run a live strategy. A separately gated Bitunix REST client is included; its signed private requests have only been tested against local mocks, not your account. Read [`docs/SAFETY.md`](docs/SAFETY.md) before using it.

## Requirements and run

- Go 1.22+
- Optional OpenAI-compatible provider URL/API key for `/models`, `/check_ai`, and `/think`

```sh
cp .env.example .env
# edit .env locally; never commit keys
go run ./cmd/jrock
```

Commands begin with `/`; use `/help`. `AI_BASE_URL`, `AI_API_KEY`, and `AI_MODEL` may be set in process environment or `.env`. Model discovery calls `GET {base}/models`; choose an ID via `/set_model ID`, then run `/check_ai`. `/models` listing alone does not prove the endpoint supports chat completions. Provider error response bodies are not printed to avoid leaking secrets or private server details.

## Current implementation

- Model discovery, model selection, connectivity probe, and simple chat completions for OpenAI-compatible APIs.
- Local SOUL.md and append-only text memory.
- Paper-trader settings and command flags with basic validation; default indicators are EMA, MA, RSI, MACD, VOLUME, MOMENTUM, ATR_BREAKOUT, SUPERTREND, BOLLINGER. Requested timeframes and basic thresholds are represented as config.
- `/check_system` and `/verify` run `go test ./...` and `go vet ./...`.
- A separate Bitunix REST package in `internal/bitunix`: SHA-256 request signing; public trading-pair metadata; signed account, position, history, leverage/mode, order, and TP/SL methods. Writes are blocked by default. `NewLiveClient` requires both `BITUNIX_LIVE_TRADING=I_ACKNOWLEDGE_REAL_TRADING` and an explicit call-site acknowledgement; this is an API-library gate, not an autonomous/live strategy loop.
- Bitunix order-unit conversion using the supplied Help Center article: Nominal Value → `qty = notional/price`; Cost Value → `qty = cost × leverage/price`; Quantity is base-asset `qty`. Quantity is rounded down to `basePrecision` and checked against the returned minimum/maximum volumes, symbol status, API support, and leverage limits. Trading fees are not included in Cost Value.
- `go run ./cmd/bitunix-check` performs a signed **read-only** account check after `BITUNIX_API_KEY` and `BITUNIX_SECRET_KEY` are configured locally in `.env`. No order is sent.
- API documentation index in [`docs/BITUNIX_SOURCES.md`](docs/BITUNIX_SOURCES.md).
- Separate Bitunix futures historical-candle research prototype in [`backtesting/bitunix_backtest.py`](backtesting/bitunix_backtest.py), plus reports in [`backtesting/RESULTS.md`](backtesting/RESULTS.md). It is Python, public-data-only, and not connected to the Go agent or live trading. Requires `pandas`, `numpy`, and `requests`: `python -m pip install pandas numpy requests`. Run, for example: `python backtesting/bitunix_backtest.py --symbol BTCUSDT --days 30 --timeframes 15m`. The four-timeframe default is `--timeframes 1m,3m,5m,15m`; the API returned only 2.07 days of overlapping history in the current run.

Not implemented/wired into the agent loop: autonomous code changes, auto-accept, broad device/terminal access, executable MCP/plugins/skills, real subagents, strategy/indicator execution, market scanner, portfolio risk manager, live position management, trailing/breakeven automation, TP/SL verification/reconciliation, WebSockets, and live PnL/reporting. The Bitunix client has selected REST methods, but these are not called by trader commands. No authenticated private request or live order has been tested against a real account.

## Editing and approvals

`AUTO_ACCEPT_EDITS` is present in `.env.example` but deliberately does not grant permissions: this scaffold has no source-edit tool. To build that later, use reviewed unified diffs, path allowlists, snapshots/backups, tests, and rollback. Treat auto-accept as a narrowly scoped policy, not permission for arbitrary shell/device access. Never expose a general-purpose terminal or filesystem to a model with exchange credentials.

## Go tests

```sh
go test ./...
go test -race ./...
go vet ./...
```

Test audit and coverage notes are in [`docs/GO_TESTING_AUDIT.md`](docs/GO_TESTING_AUDIT.md). Bitunix private requests are tested with local mocks; the only real exchange integration test is a public symbol-metadata read. No private account request or order has been sent.

## GitHub

The project is local here. No GitHub repo URL or PAT was included yet, and I have not pushed anything. After you create a private repository and provide its URL, use a fine-grained PAT limited to that one repository with Contents: Read and write (and Metadata: read). Prefer entering the token into your own credential manager/CLI rather than pasting it into chat. Then:

```sh
git init
git add .
git commit -m 'Initial J+rock Go scaffold'
git branch -M main
git remote add origin YOUR_REPO_URL
git push -u origin main
```

Rotate/revoke the PAT after use if it was exposed in chat. Never store it in `.env`, source, or a commit.
