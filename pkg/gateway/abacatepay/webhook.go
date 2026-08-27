package abacatepay

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"log"
)

// ValidateHMAC valida a assinatura HMAC-SHA256 do webhook AbacatePay.
func ValidateHMAC(body []byte, signature, secret string) bool {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expectedMAC := hex.EncodeToString(mac.Sum(nil))

	isValid := hmac.Equal([]byte(signature), []byte(expectedMAC))

	if !isValid {
		log.Printf("[ABACATEPAY] Webhook HMAC mismatch")
	}

	return isValid
}
