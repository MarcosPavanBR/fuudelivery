package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/carloshomar/fuudelivery/pkg/queue"
	"github.com/go-redis/redis/v8"
	"github.com/gofiber/fiber/v2"
	"golang.org/x/time/rate"
)

// ---------------------------------------------------------------------------
// Rate limiter compartilhado via Redis.
//
// O limiter em memoria (getIPLimiter) e por-instancia: com multiplas
// instancias no Render, cada uma tem seu proprio token bucket e o limite
// nunca estoura (um atacante distribui as tentativas entre as instancias).
// Este limiter usa uma janela fixa atomica no Redis (INCR + EXPIRE), entao
// o contador e global — o limite de N/min passa a valer de verdade.
//
// Fallback: se REDIS_URL nao estiver configurado (ou o Redis estiver fora),
// cai no limiter em memoria — comportamento identico ao anterior.
// ---------------------------------------------------------------------------

// rateLimitKeyPrefix identifica as chaves deste limiter no Redis.
const rateLimitKeyPrefix = "fuudelivery:ratelimit:v1:"

// rateLimitRedisTimeout limita o tempo de cada operacao no Redis.
// Se o Redis estiver lento/fora, fail-open (deixa passar) e o fallback
// em memoria assume — nunca bloqueia o request por culpa do Redis.
const rateLimitRedisTimeout = 300 * time.Millisecond

// redisLimiterOnce guarda o client Redis reutilizado entre requests.
// nil quando REDIS_URL nao esta configurado.
var (
	redisLimiterClient *redis.Client
	redisLimiterOnce   sync.Once

	// redisClientProvider permite aos testes injetar um client (miniredis)
	// sem tocar no singleton do queue. Em producao usa o queue singleton.
	redisClientProvider = func() *redis.Client {
		return queue.GetClient()
	}
)

// getRedisLimiterClient devolve o client Redis do rate limiter.
// Retorna nil se o Redis nao estiver em uso.
func getRedisLimiterClient() *redis.Client {
	redisLimiterOnce.Do(func() {
		redisLimiterClient = redisClientProvider()
	})
	return redisLimiterClient
}

// resetRedisLimiterClient existe para os testes recriarem o client
// (o singleton do queue so inicializa uma vez).
func resetRedisLimiterClient() {
	redisLimiterClient = nil
	redisLimiterOnce = sync.Once{}
}

// rateLimitKey monta a chave Redis para (ip, maxPerMinute).
// Incluir maxPerMinute evita colisao entre rotas com limites diferentes
// usando o mesmo IP.
func rateLimitKey(ip string, maxPerMinute int) string {
	return fmt.Sprintf("%s%d:%s", rateLimitKeyPrefix, maxPerMinute, ip)
}

// redisAllow registra a requisicao no Redis e retorna (permitido, usouRedis).
// Sem Redis configurado ou com erro de conexao, usouRedis=false para o
// chamador cair no fallback em memoria (nunca derruba o login por culpa
// do Redis, mas tambem nao abre a guarda quando o Redis falha).
func redisAllow(ip string, maxPerMinute int) (allowed, usedRedis bool) {
	client := getRedisLimiterClient()
	if client == nil {
		return true, false // sem Redis -> fallback em memoria
	}

	ctx, cancel := context.WithTimeout(context.Background(), rateLimitRedisTimeout)
	defer cancel()

	key := rateLimitKey(ip, maxPerMinute)
	count, err := client.Incr(ctx, key).Result()
	if err != nil {
		log.Printf("[RATELIMIT] redis INCR falhou (ip=%s): %v", ip, err)
		return true, false // erro -> fallback em memoria
	}

	if count == 1 {
		// Primeira requisicao da janela: define o TTL de 60s.
		client.Expire(ctx, key, time.Minute)
	}

	return count <= int64(maxPerMinute), true
}

// rateLimitMiddleware devolve um middleware que respeita o limite
// compartilhado via Redis quando disponivel, com fallback em memoria.
func rateLimitMiddleware(maxPerMinute int) fiber.Handler {
	rps := rate.Limit(float64(maxPerMinute) / 60.0)
	return func(c *fiber.Ctx) error {
		// Usa c.IP() que respeita a configuracao de TrustedProxies do Fiber,
		// evitando que o cliente forje X-Forwarded-For para burlar o rate limit.
		ip := c.IP()

		allowed, usedRedis := redisAllow(ip, maxPerMinute)
		if !allowed {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": "Muitas requisicoes. Tente novamente mais tarde.",
			})
		}
		if usedRedis {
			// Redis ativo: o contador e GLOBAL (compartilhado entre as
			// instancias do Render) — o limite N/min vale de verdade.
			return c.Next()
		}

		// Sem Redis (ou erro de conexao): fallback no token bucket em memoria.
		limiter := getIPLimiter(ip, rps, maxPerMinute)
		if !limiter.Allow() {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": "Muitas requisicoes. Tente novamente mais tarde.",
			})
		}
		return c.Next()
	}
}
