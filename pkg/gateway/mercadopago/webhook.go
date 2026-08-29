package mercadopago

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"log"
)

// ═══════════════════════════════════════════════════════════════
// VALIDAÇÃO DE WEBHOOK
// ═══════════════════════════════════════════════════════════════

// ValidateHMAC valida a assinatura HMAC-SHA256 de um webhook do Mercado Pago.
//
// O Mercado Pago envia a assinatura no header "x-signature".
// Algoritmo:
//  1. Calcular HMAC-SHA256 do body usando o secret como chave
//  2. Converter o resultado para hex
//  3. Comparar com a assinatura recebida usando hmac.Equal
//     (comparação em tempo constante para prevenir timing attacks)
//
// Parameters:
//   - body: corpo bruto da requisição HTTP
//   - signature: valor do header x-signature
//   - secret: MERCADOPAGO_WEBHOOK_SECRET
//
// Retorna true se a assinatura for válida.
func ValidateHMAC(body []byte, signature, secret string) bool {
	if secret == "" {
		log.Println("[MERCADOPAGO] WARNING: Webhook secret not configured. Skipping validation.")
		return true
	}

	if signature == "" {
		log.Println("[MERCADOPAGO] Webhook validation failed: missing signature")
		return false
	}

	// Calcular HMAC-SHA256 do body
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expectedMAC := hex.EncodeToString(mac.Sum(nil))

	// Comparação em tempo constante (previne timing attack)
	isValid := hmac.Equal([]byte(signature), []byte(expectedMAC))

	if !isValid {
		log.Printf("[MERCADOPAGO] Webhook HMAC mismatch: expected=%s got=%s", expectedMAC[:8]+"...", signature[:8]+"...")
	}

	return isValid
}

// ComputeHMAC calcula o HMAC-SHA256 de um body (para testes e debug).
func ComputeHMAC(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}
