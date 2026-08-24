package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"strings"
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
// v2: chave composta (IP+User-Agent) — ver clientRateLimitID.
const rateLimitKeyPrefix = "fuudelivery:ratelimit:v2:"

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

// clientRateLimitID devolve a identidade usada como chave do rate limit:
// o IP real do cliente (PRIMEIRO endereco do X-Forwarded-For — a convencao
// que o Render segue: o primeiro elemento e o cliente original) combinado
// com o User-Agent normalizado.
//
// Por que IP+UA em vez de c.IP() sozinho:
//  1. Com TrustedProxies aberto, c.IP() devolve o XFF CRU (a cadeia inteira,
//     ex.: "203.0.113.42, 10.0.0.1") — se o Render anexa IPs a cadeia varia
//     por request e a chave Redis muda, nunca disparando o limite.
//  2. O primeiro elemento do XFF e estavel (e o cliente original), entao
//     extrai-lo normaliza a chave independente do tamanho da cadeia.
//  3. User-Agent adiciona entropia: usuarios distintos atras do mesmo proxy
//     (mesmo IP de saida) nao colidem no mesmo bucket; e burlar o limite
//     passaria a exigir variar IP E UA ao mesmo tempo.
//
// Fallback: sem XFF valido, usa o IP do socket (RemoteIP) — quem chega
// direto (sem proxy) continua sendo identificado de forma estavel.
func clientRateLimitID(c *fiber.Ctx) string {
	ip := firstForwardedIP(c)
	if ip == "" {
		ip = c.Context().RemoteIP().String()
	}
	ua := normalizeUserAgent(c.Get("User-Agent"))
	return ip + "|" + ua
}

// firstForwardedIP extrai o PRIMEIRO endereco IP valido do X-Forwarded-For.
// A cadeia vem como "cliente, proxy1, proxy2" — o primeiro e o cliente
// original (convencao XFF, seguida pelo Render). Retorna "" se nao houver
// header ou nenhum elemento parseavel.
func firstForwardedIP(c *fiber.Ctx) string {
	xff := c.Get("X-Forwarded-For")
	if xff == "" {
		return ""
	}
	for _, part := range strings.Split(xff, ",") {
		ip := strings.TrimSpace(part)
		if host, _, err := net.SplitHostPort(ip); err == nil {
			ip = host
		}
		if net.ParseIP(ip) != nil {
			return ip
		}
	}
	return ""
}

// normalizeUserAgent limita o User-Agent a um identificador estavel e curto
// para a chave: trim, lowercase e truncado em 48 chars (evita chaves Redis
// gigantes com UAs longos; case/whitespace diferentes nao criam buckets novos).
func normalizeUserAgent(ua string) string {
	ua = strings.ToLower(strings.TrimSpace(ua))
	if len(ua) > 48 {
		ua = ua[:48]
	}
	return ua
}

// rateLimitKey monta a chave Redis para (identidade, maxPerMinute).
// Incluir maxPerMinute evita colisao entre rotas com limites diferentes
// usando a mesma identidade. A identidade (IP+UA) e hasheada com SHA-256
// para a chave caber no Redis sem caracteres estranhos do User-Agent.
func rateLimitKey(id string, maxPerMinute int) string {
	sum := sha256.Sum256([]byte(id))
	hash := hex.EncodeToString(sum[:])[:16]
	return fmt.Sprintf("%s%d:%s", rateLimitKeyPrefix, maxPerMinute, hash)
}

// redisAllow registra a requisicao no Redis e retorna (permitido, usouRedis).
// Sem Redis configurado ou com erro de conexao, usouRedis=false para o
// chamador cair no fallback em memoria (nunca derruba o login por culpa
// do Redis, mas tambem nao abre a guarda quando o Redis falha).
func redisAllow(id string, maxPerMinute int) (allowed, usedRedis bool) {
	client := getRedisLimiterClient()
	if client == nil {
		return true, false // sem Redis -> fallback em memoria
	}

	ctx, cancel := context.WithTimeout(context.Background(), rateLimitRedisTimeout)
	defer cancel()

	key := rateLimitKey(id, maxPerMinute)
	count, err := client.Incr(ctx, key).Result()
	if err != nil {
		log.Printf("[RATELIMIT] redis INCR falhou (id=%s): %v", id, err)
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
		// Chave composta IP+User-Agent (primeiro IP do XFF, nao a cadeia crua
		// que c.IP() devolveria) — estavel mesmo com proxy variavel e mais
		// dificil de burlar (exige variar IP e UA ao mesmo tempo).
		id := clientRateLimitID(c)

		allowed, usedRedis := redisAllow(id, maxPerMinute)
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
		limiter := getIPLimiter(id, rps, maxPerMinute)
		if !limiter.Allow() {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": "Muitas requisicoes. Tente novamente mais tarde.",
			})
		}
		return c.Next()
	}
}
