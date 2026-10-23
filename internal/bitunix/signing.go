package bitunix

import (
	"crypto/sha256"
	"encoding/hex"
)

// signature matches Bitunix REST signing docs: SHA256 twice, not HMAC.
// digest = SHA256(nonce + timestamp + apiKey + canonicalQuery + exactJSONBody)
// sign   = SHA256(hex(digest) + secretKey)
func signature(nonce, timestamp, apiKey, secret, query, body string) string {
	first := sha256.Sum256([]byte(nonce + timestamp + apiKey + query + body))
	firstHex := hex.EncodeToString(first[:])
	second := sha256.Sum256([]byte(firstHex + secret))
	return hex.EncodeToString(second[:])
}
