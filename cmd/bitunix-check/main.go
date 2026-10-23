// bitunix-check performs a signed, read-only account request to verify local
// credentials. It never changes settings or sends orders.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/yourname/jrock-agent/internal/bitunix"
	"github.com/yourname/jrock-agent/internal/config"
)

func main() {
	cfg := config.Load()
	if cfg.BitunixAPIKey == "" || cfg.BitunixSecret == "" {
		fmt.Fprintln(os.Stderr, "Set BITUNIX_API_KEY and BITUNIX_SECRET_KEY locally in .env first; do not paste them into chat.")
		os.Exit(2)
	}
	client, err := bitunix.NewClient(cfg.BitunixBaseURL, cfg.BitunixAPIKey, cfg.BitunixSecret)
	if err != nil {
		fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	_, err = client.GetAccount(ctx, "USDT")
	if err != nil {
		fatal(err)
	}
	fmt.Println("Signed Bitunix account read succeeded. This check is read-only; no order or account setting was changed.")
}

func fatal(err error) { fmt.Fprintln(os.Stderr, "Bitunix check failed:", err); os.Exit(1) }
