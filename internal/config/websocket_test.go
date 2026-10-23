package config

import "testing"

func TestLoadBitunixWebSocketDefaultsAndOverrides(t *testing.T) {
	t.Setenv("BITUNIX_WS_PUBLIC", "")
	t.Setenv("BITUNIX_WS_PRIVATE", "")
	cfg := Load()
	if cfg.BitunixWSPublic != "wss://fapi.bitunix.com/public/" || cfg.BitunixWSPrivate != "wss://fapi.bitunix.com/private/" {
		t.Fatalf("websocket defaults = %q / %q", cfg.BitunixWSPublic, cfg.BitunixWSPrivate)
	}

	t.Setenv("BITUNIX_WS_PUBLIC", "wss://example.test/pub")
	t.Setenv("BITUNIX_WS_PRIVATE", "wss://example.test/priv")
	cfg = Load()
	if cfg.BitunixWSPublic != "wss://example.test/pub" || cfg.BitunixWSPrivate != "wss://example.test/priv" {
		t.Fatalf("websocket overrides = %q / %q", cfg.BitunixWSPublic, cfg.BitunixWSPrivate)
	}
}
