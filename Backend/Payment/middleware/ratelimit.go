// Package middleware - ratelimit.go
// Rate limiting por IP e por usuario usando token bucket.
// Previne brute force em login e abuso em endpoints de pagamento.
package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/time/rate"
)

// ipLimiter armazena limiters por endereco IP.
type ipLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

var (
	ipLimiters   = make(map[string]*ipLimiter)
	ipMu         sync.Mutex
	userLimiters = make(map[string]*ipLimiter)
	userMu       sync.Mutex
)

// getIPLimiter retorna (ou cria) um rate limiter para o IP informado.
// Limite: max Burst requests por periodo.
func getIPLimiter(ip string, rps rate.Limit, burst int) *rate.Limiter {
	ipMu.Lock()
	defer ipMu.Unlock()

	li, exists := ipLimiters[ip]
	if !exists {
		li = &ipLimiter{limiter: rate.NewLimiter(rps, burst)}
		ipLimiters[ip] = li
	}
	li.lastSeen = time.Now()
	return li.limiter
}

// getUserLimiter retorna (ou cria) um rate limiter para o usuario informado.
func getUserLimiter(userID string, rps rate.Limit, burst int) *rate.Limiter {
	userMu.Lock()
	defer userMu.Unlock()

	li, exists := userLimiters[userID]
	if !exists {
		li = &ipLimiter{limiter: rate.NewLimiter(rps, burst)}
		userLimiters[userID] = li
	}
	li.lastSeen = time.Now()
	return li.limiter
}

// cleanupStaleLimiters remove entries de IPs/usuarios inativos (a cada 10 min).
func init() {
	go func() {
		for {
			time.Sleep(10 * time.Minute)
			cutoff := time.Now().Add(-10 * time.Minute)

			ipMu.Lock()
			for ip, li := range ipLimiters {
				if li.lastSeen.Before(cutoff) {
					delete(ipLimiters, ip)
				}
			}
			ipMu.Unlock()

			userMu.Lock()
			for uid, li := range userLimiters {
				if li.lastSeen.Before(cutoff) {
					delete(userLimiters, uid)
				}
			}
			userMu.Unlock()
		}
	}()
}

// RateLimitByIP cria middleware que limita requests por IP.
// rps = requests por segundo, burst = maximo instantaneo.
func RateLimitByIP(rps rate.Limit, burst int) fiber.Handler {
	return func(c *fiber.Ctx) error {
		limiter := getIPLimiter(c.IP(), rps, burst)
		if !limiter.Allow() {
			return c.Status(http.StatusTooManyRequests).JSON(fiber.Map{
				"error":   "rate_limit_exceeded",
				"message": "Muitas requisições. Tente novamente em alguns segundos.",
			})
		}
		return c.Next()
	}
}

// RateLimitByUser cria middleware que limita requests por user_id do JWT.
// Usado em endpoints autenticados (pagamento, carteira).
func RateLimitByUser(rps rate.Limit, burst int) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID, _ := c.Locals("user_id").(string)
		if userID == "" {
			userID = c.IP() // fallback para IP se não autenticado
		}
		limiter := getUserLimiter(userID, rps, burst)
		if !limiter.Allow() {
			return c.Status(http.StatusTooManyRequests).JSON(fiber.Map{
				"error":   "rate_limit_exceeded",
				"message": "Limite de operações excedido. Tente novamente em alguns segundos.",
			})
		}
		return c.Next()
	}
}

// LoginRateLimit é um RateLimitByIP pré-configurado para login (5 req/min burst 5).
func LoginRateLimit() fiber.Handler {
	return RateLimitByIP(rate.Every(12*time.Second), 5) // 5 req/min, burst 5
}

// PaymentRateLimit é um RateLimitByUser pré-configurado para pagamento (10 req/min).
func PaymentRateLimit() fiber.Handler {
	return RateLimitByUser(rate.Every(6*time.Second), 10) // 10 req/min, burst 10
}

// WalletRateLimit é um RateLimitByUser pré-configurado para carteira (20 req/min).
func WalletRateLimit() fiber.Handler {
	return RateLimitByUser(rate.Every(3*time.Second), 20) // 20 req/min, burst 20
}
