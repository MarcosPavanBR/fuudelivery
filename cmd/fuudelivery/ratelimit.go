package main

// Rate limiting por IP e por conta (user_type:identifier).

import (
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/time/rate"
	// Models (database initialization)
	// Handlers
	// Middleware
	// Dispatch engine
	// Batch expiry
	// Queue + Health + Upload + Metrics + Search
)

// Rate limiter por IP usando golang.org/x/time/rate (token bucket).
// Entries stale sao limpas periodicamente para evitar memory leak.
type ipLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

var (
	ipLimiters   = make(map[string]*ipLimiter)
	ipLimitersMu sync.Mutex
)

func getIPLimiter(ip string, rps rate.Limit, burst int) *rate.Limiter {
	ipLimitersMu.Lock()
	defer ipLimitersMu.Unlock()

	li, exists := ipLimiters[ip]
	if !exists {
		li = &ipLimiter{limiter: rate.NewLimiter(rps, burst)}
		ipLimiters[ip] = li
	}
	li.lastSeen = time.Now()
	return li.limiter
}

func startRateLimitCleanup() {
	go func() {
		for {
			time.Sleep(10 * time.Minute)
			cutoff := time.Now().Add(-10 * time.Minute)
			ipLimitersMu.Lock()
			for ip, li := range ipLimiters {
				if li.lastSeen.Before(cutoff) {
					delete(ipLimiters, ip)
				}
			}
			ipLimitersMu.Unlock()
		}
	}()
}

// rateLimitByIdentifierMiddleware cria um middleware que limita requests
// por identificador de conta (user_type:identifier), não por IP.
//
// Protege contra brute-force distribuído: um atacante com múltiplos IPs
// tentando a mesma conta atinge o teto rápido. O body JSON é lido aqui
// e armazenado em c.Locals("parsedBody") para o handler não precisar
// parseá-lo duas vezes.
//
// maxPerMinute é o limite de requests por minuto por identificador.
func rateLimitByIdentifierMiddleware(maxPerMinute int) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Lê o body para extrair user_type + identifier.
		var body struct {
			UserType   string `json:"user_type"`
			Identifier string `json:"identifier"`
		}
		if err := c.BodyParser(&body); err != nil {
			// Body malformado — deixa o handler retornar o erro 400.
			return c.Next()
		}

		userType := strings.TrimSpace(body.UserType)
		identifier := strings.TrimSpace(body.Identifier)
		if userType == "" || identifier == "" {
			// Campos faltando — handler vai retornar 400.
			return c.Next()
		}

		if !rateLimitByIdentifier(userType, identifier, maxPerMinute) {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": "Muitas tentativas para esta conta. Aguarde alguns minutos e tente novamente.",
			})
		}

		return c.Next()
	}
}
