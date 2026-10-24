package config

import (
	"bufio"
	"os"
	"strings"
)

type Config struct {
	BaseURL, APIKey, Model, Provider             string
	MemoryPath                                   string
	AutoAcceptEdits                              bool
	TradingMode                                  string
	BitunixBaseURL, BitunixAPIKey, BitunixSecret string
	BitunixWSPublic, BitunixWSPrivate            string
}

// Load reads environment variables and a deliberately small .env subset. Existing
// process environment always wins; values are never logged.
func Load() Config {
	loadDotEnv(".env")
	c := Config{
		BaseURL: get("AI_BASE_URL", "https://api.openai.com/v1"),
		APIKey:  os.Getenv("AI_API_KEY"), Model: os.Getenv("AI_MODEL"),
		Provider:        get("AI_PROVIDER", "openai-compatible"),
		MemoryPath:      get("MEMORY_PATH", "data/memory.md"),
		AutoAcceptEdits: strings.EqualFold(os.Getenv("AUTO_ACCEPT_EDITS"), "true"),
		TradingMode:     get("TRADING_MODE", "paper"),
		BitunixBaseURL:  get("BITUNIX_BASE_URL", "https://fapi.bitunix.com"),
		BitunixAPIKey:   os.Getenv("BITUNIX_API_KEY"), BitunixSecret: os.Getenv("BITUNIX_SECRET_KEY"),
		BitunixWSPublic:  get("BITUNIX_WS_PUBLIC", "wss://fapi.bitunix.com/public/"),
		BitunixWSPrivate: get("BITUNIX_WS_PRIVATE", "wss://fapi.bitunix.com/private/"),
	}
	if c.TradingMode != "paper" {
		c.TradingMode = "paper"
	} // live trading intentionally unavailable in this scaffold
	return c
}
func get(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.Trim(strings.TrimSpace(v), "\"'")
		if _, exists := os.LookupEnv(k); !exists {
			_ = os.Setenv(k, v)
		}
	}
}
