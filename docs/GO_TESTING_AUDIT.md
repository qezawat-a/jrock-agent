# Go testing skill run — 2026-10-23

Applied the `golang-testing` guidance. Tests are named, isolated, and use standard-library tools; HTTP behavior is exercised with `httptest`. This codebase remains small, so no multi-agent audit was needed.

## Added/expanded tests

- `internal/trader/engine_test.go`: defaults; valid settings; invalid values; no mutation on rejection; status output. Tests caught/fixed partial float parsing (`5oops`), non-finite values, and empty/unsupported settings.
- `internal/provider/provider_test.go`: model-list request/auth/decode, chat payload/response, chat probe, empty choices, invalid base URL, malformed responses, and redaction of HTTP error bodies.
- `internal/config/config_test.go`, `websocket_test.go`, `memory_test.go`: `.env` parsing, comments/quotes, environment precedence, WebSocket URL defaults/overrides, and configurable memory path.
- `internal/agent/repl_test.go`, `memory_test.go`:  paper-engine/report flags, refusal to activate auto-trading, valid settings, invalid/unknown commands, append/read memory persistence, bounded memory context, and injection of saved notes into chat requests.
- `internal/bitunix/client_test.go`, `units_test.go`, `endpoints_test.go`, `websocket_test.go`, `order_unit_request_test.go`, `kline_test.go`: official signature test vector, query canonicalization, public/private client restrictions, live-write opt-in, local mocked signed REST account/order requests, selected route mapping, order-unit conversion, precision/min/max/leverage/symbol validation, safe API errors, public/private WebSocket protocol mocks, and candle endpoint/query/schema tests.
- `internal/trader/signals_test.go` and `cmd/bitunix-paper/main_test.go`: nine-indicator frame evaluation, multi-timeframe consensus, validation, and mocked public-candle-to-paper-signal pipeline.
- `internal/config/websocket_test.go`: public/private Bitunix WebSocket defaults and environment overrides.
- `internal/bitunix/integration_test.go` is build-tagged `integration`. It was run against Bitunix's **public** `trading_pairs` endpoint and passed; it does not use keys or mutate an account.
- `.github/workflows/go-test.yml`: CI runs `go test -race ./...` and `go vet ./...`.

## Verification performed

Using Go 1.24.12 downloaded temporarily to `/tmp`:

- `go test ./...` — passed
- `go test -race ./...` — passed
- `go vet ./...` — passed
- `go test -tags=integration ./internal/bitunix -run TestBitunixPublicTradingPairMetadata -count=1 -v` — passed against Bitunix public metadata endpoint
- `go test -coverprofile=... ./...` — passed; total statement coverage **60.2%** (Bitunix 77.6%, provider 88.2%, trader 79.0%, config 75.0%, agent 21.0%).

Coverage is a gap finder, not a target. `cmd/bitunix-check` intentionally has no unit test: its behavior is a thin read-only wrapper around the tested client.

## Live account testing limits

The signed private client and write methods were tested against local HTTP mocks, **not** an authenticated Bitunix account. No live order or setting change was sent. The public market metadata integration test passed. To verify private connectivity locally, configure API credentials with read-only permissions in `.env` and run `go run ./cmd/bitunix-check`. Actual order writes require deliberate `NewLiveClient` opt-in and should first be tested against Bitunix demo trading. The agent's autonomous trader loop is still not wired to this client.

## Addendum — WebSockets, paper signals, and persistent memory (2026-10-24)

- Added local mock tests for public WS subscribe/ping/push and private login signing/subscriptions; config tests cover the two WS URL defaults/overrides.
- Added a safe `cmd/bitunix-paper` worker that polls Bitunix public closed candles, computes the backtested nine-indicator votes and timeframe consensus, and logs signals with orders hard-disabled. Added a mock pipeline test and an opt-in public Bitunix candle-to-signal integration test.
- Added memory tests proving append/read at `MEMORY_PATH` and inclusion of saved notes in `/think` context (capped to the latest 12,000 characters).
- Re-ran `go test ./... -count=1`, `go test -race ./... -count=1`, `go vet ./...`, and public `trading_pairs` and K-line integration tests — all passed. Also ran `go run ./cmd/bitunix-paper --once` against live public candles; it emitted a neutral 2-vs-2 timeframe signal and `orders=DISABLED`.
- WebSocket tests use local mocks; no private WS session or live order was sent. The paper runner does not place orders. Railway persistence still requires a mounted Volume.
