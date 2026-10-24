package bitunix

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

func (c *Client) PublicGet(ctx context.Context, path string, query url.Values, out any) error {
	if !strings.HasPrefix(path, "/api/v1/futures/market/") && path != "/api/v1/futures/position/get_position_tiers" {
		return errors.New("refusing non-public Bitunix path")
	}
	if query == nil {
		query = url.Values{}
	}
	u := c.baseURL + path
	if q := query.Encode(); q != "" {
		u += "?" + q
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("Bitunix public request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Bitunix returned HTTP %d", resp.StatusCode)
	}
	var env apiEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return fmt.Errorf("decode Bitunix response: %w", err)
	}
	if env.Code != 0 {
		return fmt.Errorf("Bitunix API error code %d", env.Code)
	}
	if out == nil || len(env.Data) == 0 || string(env.Data) == "null" {
		return nil
	}
	return json.Unmarshal(env.Data, out)
}

// GetTradingPairs fetches public symbol rules needed to convert UI order units
// and validate API qty, precision, min/max volume, leverage range, and API status.
func (c *Client) GetTradingPairs(ctx context.Context, symbols []string) ([]TradingPair, error) {
	q := url.Values{}
	if len(symbols) > 0 {
		q.Set("symbols", strings.Join(symbols, ","))
	}
	var out []TradingPair
	err := c.PublicGet(ctx, "/api/v1/futures/market/trading_pairs", q, &out)
	return out, err
}

type Kline struct {
	Open     string `json:"open"`
	High     string `json:"high"`
	Low      string `json:"low"`
	Close    string `json:"close"`
	QuoteVol string `json:"quoteVol"`
	BaseVol  string `json:"baseVol"`
	Time     int64  `json:"time"`
	Type     string `json:"type"`
}

// UnmarshalJSON accepts both numeric and quoted Unix-millisecond candle times;
// the docs specify an integer, but public responses may serialize it as a string.
func (k *Kline) UnmarshalJSON(data []byte) error {
	var wire struct {
		Open     string          `json:"open"`
		High     string          `json:"high"`
		Low      string          `json:"low"`
		Close    string          `json:"close"`
		QuoteVol string          `json:"quoteVol"`
		BaseVol  string          `json:"baseVol"`
		Time     json.RawMessage `json:"time"`
		Type     string          `json:"type"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	timeRaw := strings.TrimSpace(string(wire.Time))
	if strings.HasPrefix(timeRaw, "\"") {
		var text string
		if err := json.Unmarshal(wire.Time, &text); err != nil {
			return err
		}
		timeRaw = strings.TrimSpace(text)
	}
	timestamp, err := strconv.ParseInt(timeRaw, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid kline time %q: %w", timeRaw, err)
	}
	*k = Kline{Open: wire.Open, High: wire.High, Low: wire.Low, Close: wire.Close, QuoteVol: wire.QuoteVol, BaseVol: wire.BaseVol, Time: timestamp, Type: wire.Type}
	return nil
}

// GetKlines fetches closed futures candles from Bitunix's public market endpoint.
// Bitunix documents a maximum limit of 200 candles per request.
func (c *Client) GetKlines(ctx context.Context, symbol, interval, priceType string, limit int) ([]Kline, error) {
	if strings.TrimSpace(symbol) == "" {
		return nil, errors.New("symbol is required")
	}
	switch interval {
	case "1m", "3m", "5m", "15m", "30m", "1h", "2h", "4h", "6h", "8h", "12h", "1d", "3d", "1w", "1M":
	default:
		return nil, fmt.Errorf("unsupported kline interval %q", interval)
	}
	if priceType == "" {
		priceType = "LAST_PRICE"
	}
	if priceType != "LAST_PRICE" && priceType != "MARK_PRICE" {
		return nil, errors.New("kline type must be LAST_PRICE or MARK_PRICE")
	}
	if limit < 1 || limit > 200 {
		return nil, errors.New("kline limit must be between 1 and 200")
	}
	q := url.Values{
		"symbol":   []string{symbol},
		"interval": []string{interval},
		"limit":    []string{fmt.Sprint(limit)},
		"type":     []string{priceType},
	}
	var out []Kline
	err := c.PublicGet(ctx, "/api/v1/futures/market/kline", q, &out)
	return out, err
}

func (c *Client) GetAccount(ctx context.Context, marginCoin string) (json.RawMessage, error) {
	q := url.Values{"marginCoin": []string{marginCoin}}
	var out json.RawMessage
	err := c.doPrivate(ctx, http.MethodGet, "/api/v1/futures/account", q, nil, &out)
	return out, err
}

func (c *Client) GetPendingPositions(ctx context.Context, symbol, positionID string) (json.RawMessage, error) {
	q := url.Values{}
	if symbol != "" {
		q.Set("symbol", symbol)
	}
	if positionID != "" {
		q.Set("positionId", positionID)
	}
	var out json.RawMessage
	err := c.doPrivate(ctx, http.MethodGet, "/api/v1/futures/position/get_pending_positions", q, nil, &out)
	return out, err
}

func (c *Client) GetLeverageAndMarginMode(ctx context.Context, symbol, marginCoin string) (json.RawMessage, error) {
	q := url.Values{"symbol": []string{symbol}, "marginCoin": []string{marginCoin}}
	var out json.RawMessage
	err := c.doPrivate(ctx, http.MethodGet, "/api/v1/futures/account/get_leverage_margin_mode", q, nil, &out)
	return out, err
}

func (c *Client) GetPositionTiers(ctx context.Context, symbol string) (json.RawMessage, error) {
	q := url.Values{"symbol": []string{symbol}}
	var out json.RawMessage
	err := c.PublicGet(ctx, "/api/v1/futures/position/get_position_tiers", q, &out)
	return out, err
}

func (c *Client) GetHistoryPositions(ctx context.Context, query url.Values) (json.RawMessage, error) {
	var out json.RawMessage
	err := c.doPrivate(ctx, http.MethodGet, "/api/v1/futures/position/get_history_positions", query, nil, &out)
	return out, err
}
func (c *Client) GetHistoryOrders(ctx context.Context, query url.Values) (json.RawMessage, error) {
	var out json.RawMessage
	err := c.doPrivate(ctx, http.MethodGet, "/api/v1/futures/trade/get_history_orders", query, nil, &out)
	return out, err
}
func (c *Client) GetHistoryTrades(ctx context.Context, query url.Values) (json.RawMessage, error) {
	var out json.RawMessage
	err := c.doPrivate(ctx, http.MethodGet, "/api/v1/futures/trade/get_history_trades", query, nil, &out)
	return out, err
}
func (c *Client) GetPendingOrders(ctx context.Context, query url.Values) (json.RawMessage, error) {
	var out json.RawMessage
	err := c.doPrivate(ctx, http.MethodGet, "/api/v1/futures/trade/get_pending_orders", query, nil, &out)
	return out, err
}
func (c *Client) GetPendingTPSL(ctx context.Context, query url.Values) (json.RawMessage, error) {
	var out json.RawMessage
	err := c.doPrivate(ctx, http.MethodGet, "/api/v1/futures/tpsl/get_pending_orders", query, nil, &out)
	return out, err
}
func (c *Client) GetHistoryTPSL(ctx context.Context, query url.Values) (json.RawMessage, error) {
	var out json.RawMessage
	err := c.doPrivate(ctx, http.MethodGet, "/api/v1/futures/tpsl/get_history_orders", query, nil, &out)
	return out, err
}

type LeverageRequest struct {
	Symbol     string `json:"symbol"`
	MarginCoin string `json:"marginCoin"`
	Leverage   int    `json:"leverage"`
}

func (c *Client) ChangeLeverage(ctx context.Context, req LeverageRequest) (json.RawMessage, error) {
	if req.Symbol == "" || req.MarginCoin == "" || req.Leverage < 1 {
		return nil, errors.New("symbol, marginCoin, and positive leverage are required")
	}
	var out json.RawMessage
	err := c.doPrivate(ctx, http.MethodPost, "/api/v1/futures/account/change_leverage", nil, req, &out)
	return out, err
}

type MarginModeRequest struct {
	Symbol     string `json:"symbol"`
	MarginCoin string `json:"marginCoin"`
	MarginMode string `json:"marginMode"`
}

func (c *Client) ChangeMarginMode(ctx context.Context, req MarginModeRequest) (json.RawMessage, error) {
	if req.Symbol == "" || req.MarginCoin == "" || (req.MarginMode != "ISOLATION" && req.MarginMode != "CROSS") {
		return nil, errors.New("valid symbol, marginCoin and ISOLATION/CROSS marginMode are required")
	}
	var out json.RawMessage
	err := c.doPrivate(ctx, http.MethodPost, "/api/v1/futures/account/change_margin_mode", nil, req, &out)
	return out, err
}

type PositionModeRequest struct {
	PositionMode string `json:"positionMode"`
}

func (c *Client) ChangePositionMode(ctx context.Context, req PositionModeRequest) (json.RawMessage, error) {
	if req.PositionMode != "HEDGE" && req.PositionMode != "ONE_WAY" {
		return nil, errors.New("positionMode must be HEDGE or ONE_WAY")
	}
	var out json.RawMessage
	err := c.doPrivate(ctx, http.MethodPost, "/api/v1/futures/account/change_position_mode", nil, req, &out)
	return out, err
}

type OrderRequest struct {
	Symbol       string `json:"symbol"`
	Qty          string `json:"qty"`
	Price        string `json:"price,omitempty"`
	Side         string `json:"side"`
	TradeSide    string `json:"tradeSide,omitempty"`
	PositionID   string `json:"positionId,omitempty"`
	OrderType    string `json:"orderType"`
	Effect       string `json:"effect,omitempty"`
	ClientID     string `json:"clientId,omitempty"`
	ReduceOnly   bool   `json:"reduceOnly,omitempty"`
	TPPrice      string `json:"tpPrice,omitempty"`
	TPStopType   string `json:"tpStopType,omitempty"`
	TPOrderType  string `json:"tpOrderType,omitempty"`
	TPOrderPrice string `json:"tpOrderPrice,omitempty"`
	SLPrice      string `json:"slPrice,omitempty"`
	SLStopType   string `json:"slStopType,omitempty"`
	SLOrderType  string `json:"slOrderType,omitempty"`
	SLOrderPrice string `json:"slOrderPrice,omitempty"`
}

type OrderResult struct {
	OrderID  string `json:"orderId"`
	ClientID string `json:"clientId"`
}

func (c *Client) PlaceOrder(ctx context.Context, req OrderRequest) (OrderResult, error) {
	qty, validQty := decimal(req.Qty)
	if req.Symbol == "" || !validQty || qty.Sign() <= 0 || (req.Side != "BUY" && req.Side != "SELL") || (req.OrderType != "MARKET" && req.OrderType != "LIMIT") {
		return OrderResult{}, errors.New("symbol, positive base-asset qty, BUY/SELL side and MARKET/LIMIT orderType are required")
	}
	if req.OrderType == "LIMIT" && req.Price == "" {
		return OrderResult{}, errors.New("price is required for LIMIT order")
	}
	if req.TradeSide == "CLOSE" && req.PositionID == "" {
		return OrderResult{}, errors.New("positionId is required to close a position")
	}
	if req.TradeSide != "" && req.TradeSide != "OPEN" && req.TradeSide != "CLOSE" {
		return OrderResult{}, errors.New("tradeSide must be OPEN or CLOSE")
	}
	var out OrderResult
	err := c.doPrivate(ctx, http.MethodPost, "/api/v1/futures/trade/place_order", nil, req, &out)
	return out, err
}

// PlaceOrderByUnit converts a UI amount (quantity, nominal value, or cost value)
// into the base-asset qty accepted by the REST API, then places the order. Pair
// metadata must come from the current trading_pairs endpoint response.
func (c *Client) PlaceOrderByUnit(ctx context.Context, req OrderRequest, amount, sizingPrice string, leverage int, unit OrderUnit, pair TradingPair) (OrderResult, OrderSizing, error) {
	if req.Symbol != pair.Symbol {
		return OrderResult{}, OrderSizing{}, errors.New("order symbol does not match the trading-pair metadata")
	}
	sizing, err := CalculateOrderSize(amount, sizingPrice, leverage, unit, OrderKind(req.OrderType), pair)
	if err != nil {
		return OrderResult{}, OrderSizing{}, err
	}
	req.Qty = sizing.Qty
	result, err := c.PlaceOrder(ctx, req)
	return result, sizing, err
}

type PositionTPSLRequest struct {
	Symbol     string `json:"symbol"`
	PositionID string `json:"positionId"`
	TPPrice    string `json:"tpPrice,omitempty"`
	TPStopType string `json:"tpStopType,omitempty"`
	SLPrice    string `json:"slPrice,omitempty"`
	SLStopType string `json:"slStopType,omitempty"`
}

func validatePositionTPSL(req PositionTPSLRequest) error {
	if req.Symbol == "" || req.PositionID == "" || (req.TPPrice == "" && req.SLPrice == "") {
		return errors.New("symbol, positionId, and at least one TP/SL trigger price are required")
	}
	if req.TPStopType != "" && req.TPStopType != "LAST_PRICE" && req.TPStopType != "MARK_PRICE" {
		return errors.New("tpStopType must be LAST_PRICE or MARK_PRICE")
	}
	if req.SLStopType != "" && req.SLStopType != "LAST_PRICE" && req.SLStopType != "MARK_PRICE" {
		return errors.New("slStopType must be LAST_PRICE or MARK_PRICE")
	}
	return nil
}
func (c *Client) PlacePositionTPSL(ctx context.Context, req PositionTPSLRequest) (json.RawMessage, error) {
	if err := validatePositionTPSL(req); err != nil {
		return nil, err
	}
	var out json.RawMessage
	err := c.doPrivate(ctx, http.MethodPost, "/api/v1/futures/tpsl/position/place_order", nil, req, &out)
	return out, err
}
func (c *Client) ModifyPositionTPSL(ctx context.Context, req PositionTPSLRequest) (json.RawMessage, error) {
	if err := validatePositionTPSL(req); err != nil {
		return nil, err
	}
	var out json.RawMessage
	err := c.doPrivate(ctx, http.MethodPost, "/api/v1/futures/tpsl/position/modify_order", nil, req, &out)
	return out, err
}
func (c *Client) CancelTPSL(ctx context.Context, symbol, orderID string) (json.RawMessage, error) {
	if symbol == "" || orderID == "" {
		return nil, errors.New("symbol and TP/SL orderId are required")
	}
	var out json.RawMessage
	err := c.doPrivate(ctx, http.MethodPost, "/api/v1/futures/tpsl/cancel_order", nil, map[string]string{"symbol": symbol, "orderId": orderID}, &out)
	return out, err
}
func (c *Client) FlashClosePosition(ctx context.Context, positionID string) (json.RawMessage, error) {
	if positionID == "" {
		return nil, errors.New("positionId is required")
	}
	var out json.RawMessage
	err := c.doPrivate(ctx, http.MethodPost, "/api/v1/futures/trade/flash_close_position", nil, map[string]string{"positionId": positionID}, &out)
	return out, err
}
