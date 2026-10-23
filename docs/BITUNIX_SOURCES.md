# Bitunix futures documentation and implementation notes

Official API documentation links supplied by the user and consulted for the REST/WebSocket client. Validate current schemas and signing rules before production use. The project has selected guarded REST methods and a low-level WebSocket transport, but the strategy loop is not wired to either client and private WebSockets have not been tested with user credentials.

- Demo examples: https://github.com/qezawat-a/open-api/Demo
- Liquidation mechanism and tiered risk limits: https://support.bitunix.com/hc/en-us/articles/32152530856601-Bitunix-Futures-Liquidation-Mechanism-and-Tiered-Risk-Limit
- Tickers: https://www.bitunix.com/api-docs/futures/market/get_tickers.html
- Changelog: https://www.bitunix.com/api-docs/futures/log/change_log.html
- Adjust position margin: https://www.bitunix.com/api-docs/futures/account/adjust_position_margin.html
- Change leverage: https://www.bitunix.com/api-docs/futures/account/change_leverage.html
- Change margin mode: https://www.bitunix.com/api-docs/futures/account/change_margin_mode.html
- Change position mode: https://www.bitunix.com/api-docs/futures/account/change_position_mode.html
- Get leverage and margin mode: https://www.bitunix.com/api-docs/futures/account/get_leverage_and_margin_mode.html
- Get single account: https://www.bitunix.com/api-docs/futures/account/get_single_account.html
- Asset query: https://www.bitunix.com/api-docs/futures/copyTrading/asset/asset_query.html
- Error codes: https://www.bitunix.com/api-docs/futures/ErrorCode/error_code.html
- Market depth: https://www.bitunix.com/api-docs/futures/market/get_depth.html
- Funding-rate batch: https://www.bitunix.com/api-docs/futures/market/get_funding_rate_batch.html
- Funding-rate history: https://www.bitunix.com/api-docs/futures/market/get_funding_rate_history.html
- Funding rate: https://www.bitunix.com/api-docs/futures/market/get_funding_rate.html
- Klines: https://www.bitunix.com/api-docs/futures/market/get_kline.html
- Trading pairs: https://www.bitunix.com/api-docs/futures/market/get_trading_pairs.html
- Position history: https://www.bitunix.com/api-docs/futures/position/get_history_positions.html
- Pending positions: https://www.bitunix.com/api-docs/futures/position/get_pending_positions.html
- Position tiers: https://www.bitunix.com/api-docs/futures/position/get_position_tiers.html
- Cancel TP/SL order: https://www.bitunix.com/api-docs/futures/tp_sl/cancel_tp_sl_order.html
- TP/SL history: https://www.bitunix.com/api-docs/futures/tp_sl/get_history_tp_sl_order.html
- Pending TP/SL: https://www.bitunix.com/api-docs/futures/tp_sl/get_pending_tp_sl_order.html
- Modify position TP/SL: https://www.bitunix.com/api-docs/futures/tp_sl/modify_position_tp_sl_order.html
- Modify TP/SL order: https://www.bitunix.com/api-docs/futures/tp_sl/modify_tp_sl_order.html
- Place position TP/SL: https://www.bitunix.com/api-docs/futures/tp_sl/place_position_tp_sl_order.html
- Place TP/SL: https://www.bitunix.com/api-docs/futures/tp_sl/place_tp_sl_order.html
- Batch orders: https://www.bitunix.com/api-docs/futures/trade/batch_order.html
- Cancel all orders: https://www.bitunix.com/api-docs/futures/trade/cancel_all_orders.html
- Cancel orders: https://www.bitunix.com/api-docs/futures/trade/cancel_orders.html
- Close all positions: https://www.bitunix.com/api-docs/futures/trade/close_all_position.html
- Flash close position: https://www.bitunix.com/api-docs/futures/trade/flash_close_position.html
- Order history: https://www.bitunix.com/api-docs/futures/trade/get_history_orders.html
- Trade history: https://www.bitunix.com/api-docs/futures/trade/get_history_trades.html
- Order detail: https://www.bitunix.com/api-docs/futures/trade/get_order_detail.html
- Pending orders: https://www.bitunix.com/api-docs/futures/trade/get_pending_orders.html
- Modify order: https://www.bitunix.com/api-docs/futures/trade/modify_order.html
- Place order: https://www.bitunix.com/api-docs/futures/trade/place_order.html
- WebSocket overview: https://www.bitunix.com/api-docs/futures/websocket/prepare/WebSocket.html
- Private balance channel: https://www.bitunix.com/api-docs/futures/websocket/private/Balance%20Channel.html
- Private order channel: https://www.bitunix.com/api-docs/futures/websocket/private/Order%20Channel.html
- Private position channel: https://www.bitunix.com/api-docs/futures/websocket/private/Position%20Channel.html
- Private TP/SL channel: https://www.bitunix.com/api-docs/futures/websocket/private/Tp%20Sl%20Channel.html
- Public depth channel: https://www.bitunix.com/api-docs/futures/websocket/public/depth%20channel.html
- Public kline channel: https://www.bitunix.com/api-docs/futures/websocket/public/kline%20channel.html
- Public market price channel: https://www.bitunix.com/api-docs/futures/websocket/public/MarketPrice%20Channel.html
- Public ticker channel: https://www.bitunix.com/api-docs/futures/websocket/public/Ticker%20Channel.html
- Public tickers channel: https://www.bitunix.com/api-docs/futures/websocket/public/Tickers%20Channel.html
- Public trade channel: https://www.bitunix.com/api-docs/futures/websocket/public/Trade%20Channel.html
- **Order units explainer (important for order sizing):** https://www.bitunix.com/hub/helpcenter/article/explanation-of-the-order-units-in-futures-trading?id=170

## Verified snippets during scaffold setup

Official pages returned these parameters: tickers use `GET /api/v1/futures/market/tickers`; K-lines use `GET /api/v1/futures/market/kline`; market orders use `POST /api/v1/futures/trade/place_order`; leverage uses `POST /api/v1/futures/account/change_leverage`; position TP/SL uses `POST /api/v1/futures/tpsl/position/place_order`; and symbol rules are available from `GET /api/v1/futures/market/trading_pairs`.

### WebSocket behavior implemented in the Go transport

The documented futures endpoints are `wss://fapi.bitunix.com/public/` and `wss://fapi.bitunix.com/private/`; these are now the `.env.example` defaults (`BITUNIX_WS_PUBLIC`, `BITUNIX_WS_PRIVATE`). Public subscriptions use `{"op":"subscribe","args":[{"symbol":"BTCUSDT","ch":"price"}]}`. Private login uses a cryptographic nonce, Unix-seconds timestamp, and two-pass SHA-256 `sign = SHA256(hex(SHA256(nonce + timestamp + apiKey)) + secretKey)`. Read and validate the login acknowledgement before calling `SubscribePrivateCore()`; that helper subscribes to balance, position, and order. The documentation says TP/SL (`tpsl`) is pushed automatically after private login succeeds.

The transport spaces client messages at least 200ms apart (five/second maximum) and validates the documented 300-subscription connection limit. It exposes message reads and JSON heartbeat writes. Mock WebSocket tests validate URL/config handling, login signature construction, subscriptions, heartbeat, and received pushes. It has no reconnect/reconciliation loop and is not integrated with `cmd/jrock` or the trading engine. No authenticated private Bitunix WebSocket session has been run.

### Order-unit calculation (required when sizing from a UI notional/cost amount)

Bitunix's Help Center article above distinguishes **Nominal Value** (position notional in quote currency), **Cost Value** (margin/cost), and **Quantity Unit** (base-asset quantity). Its examples, excluding fees, imply:

- `nominalValue = qty × price`; `qty = nominalValue / price`
- `costValue ≈ nominalValue / leverage`; `qty = costValue × leverage / price`
- Quantity mode passes the underlying base-asset quantity directly

The REST `place_order` field `qty` is documented as the amount in base coin. Therefore UI notionals or cost values must be converted to base quantity before sending. Use `trading_pairs` metadata to enforce `basePrecision`, `minTradeVolume`, market/limit max volumes, leverage limits, and `isApiSupported`. The Go `internal/bitunix.CalculateOrderSize` now performs this conversion and rounds quantity down to the symbol precision; its calculations exclude transaction fees, so fee/margin checks must be applied separately. See also [`internal/bitunix/units.go`](../internal/bitunix/units.go).

The client signs private REST requests with Bitunix's documented two-pass SHA-256 scheme; writes remain blocked unless explicitly opted in. These local tests do not constitute a real authenticated exchange test.
