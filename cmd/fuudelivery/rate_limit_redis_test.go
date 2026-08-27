package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
)

// newRateLimitRedis sobe um miniredis e injeta o provider do limiter
// para apontar para ele (sem tocar no singleton do queue).
func newRateLimitRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	s, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(s.Close)

	client := redis.NewClient(&redis.Options{Addr: s.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	orig := redisClientProvider
	redisClientProvider = func() *redis.Client { return client }
	resetRedisLimiterClient()
	t.Cleanup(func() {
		redisClientProvider = orig
		resetRedisLimiterClient()
	})
	return s, client
}

func TestRedisAllow_FixedWindow(t *testing.T) {
	s, _ := newRateLimitRedis(t)

	// 10 permitidos no minuto
	for i := 1; i <= 10; i++ {
		allowed, used := redisAllow("1.2.3.4", 10)
		require.True(t, allowed, "requisicao %d deveria passar", i)
		require.True(t, used, "deveria usar o Redis")
	}

	// 11a bloqueada (limite 10/min)
	allowed, used := redisAllow("1.2.3.4", 10)
	require.False(t, allowed)
	require.True(t, used)

	// IP diferente tem janela propria
	allowed, _ = redisAllow("5.6.7.8", 10)
	require.True(t, allowed)

	// Chave inclui maxPerMinute: limite 20 do mesmo IP nao conflita com o de 10
	allowed, _ = redisAllow("1.2.3.4", 20)
	require.True(t, allowed)

	// Apos 60s a janela reseta
	s.FastForward(61 * time.Second)
	allowed, _ = redisAllow("1.2.3.4", 10)
	require.True(t, allowed, "janela deveria ter resetado")
}

func TestRedisAllow_FallbackSemRedis(t *testing.T) {
	resetRedisLimiterClient()

	allowed, used := redisAllow("9.9.9.9", 10)
	require.True(t, allowed, "sem Redis deve ser fail-open para o fallback decidir")
	require.False(t, used, "sem Redis nao deve marcar usado")
}

// testApp devolve um app Fiber com o mesmo TrustedProxies do monolith
// (lê o IP real do X-Forwarded-For), para os testes conseguirem variar o IP.
func testApp() *fiber.App {
	return fiber.New(fiber.Config{
		EnableTrustedProxyCheck: true,
		TrustedProxies:          []string{"0.0.0.0/0", "::/0"},
		ProxyHeader:             "X-Forwarded-For",
	})
}

// reqComIP monta um POST /login com X-Forwarded-For = ip.
func reqComIP(ip string) *http.Request {
	req := httptest.NewRequest("POST", "/login", nil)
	req.Header.Set("X-Forwarded-For", ip)
	return req
}

func TestRateLimitMiddleware_UsaRedisQuandoDisponivel(t *testing.T) {
	newRateLimitRedis(t)

	app := testApp()
	app.Post("/login", rateLimitMiddleware(3), func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	// 3 permitidas
	for i := 1; i <= 3; i++ {
		resp, err := app.Test(reqComIP("10.0.0.1"), 500)
		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode, "req %d", i)
	}

	// 4a -> 429
	resp, err := app.Test(reqComIP("10.0.0.1"), 500)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusTooManyRequests, resp.StatusCode, "4a requisicao deveria ser 429")

	// IP diferente nao e afetado
	resp2, err := app.Test(reqComIP("10.0.0.2"), 500)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp2.StatusCode)
}

func TestRateLimitMiddleware_FallbackMemoria(t *testing.T) {
	resetRedisLimiterClient()

	app := testApp()
	app.Post("/login", rateLimitMiddleware(2), func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	for i := 1; i <= 2; i++ {
		resp, err := app.Test(reqComIP("10.0.0.5"), 500)
		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode, "req %d", i)
	}

	resp, err := app.Test(reqComIP("10.0.0.5"), 500)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusTooManyRequests, resp.StatusCode, "3a requisicao deveria ser 429 no fallback")
}

// TestRedisAllow_RedisIndisponivelCaiNoFallback verifica que, quando o Redis
// configurado fica fora do ar, o middleware nao bloqueia o request (o fallback
// em memoria assume) — nunca derruba o login por culpa do Redis.
func TestRedisAllow_RedisIndisponivelCaiNoFallback(t *testing.T) {
	_, srv := newRateLimitRedis(t)

	app := testApp()
	app.Post("/login", rateLimitMiddleware(1), func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	// Redis no ar: 1 permitida, 2a bloqueada
	resp, _ := app.Test(reqComIP("10.0.0.9"), 500)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	resp, _ = app.Test(reqComIP("10.0.0.9"), 500)
	require.Equal(t, fiber.StatusTooManyRequests, resp.StatusCode)

	// Derruba o Redis: proxima requisicao nao pode travar nem bloquear falso
	_ = srv.Close()
	time.Sleep(50 * time.Millisecond)

	resp, err := app.Test(reqComIP("10.0.0.9"), 2000)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode, "com Redis fora, fallback em memoria assume e permite")
}

// ========================================================================
// rateLimitByIdentifier — limite por conta (user_type:identifier)
// ========================================================================

func TestRateLimitByIdentifier_Redis_FixedWindow(t *testing.T) {
	s, _ := newRateLimitRedis(t)

	// Limite de 3 por minuto
	for i := 1; i <= 3; i++ {
		require.True(t, rateLimitByIdentifier("client", "+5511999990001", 3),
			"requisicao %d deveria passar", i)
	}

	// 4a bloqueada
	require.False(t, rateLimitByIdentifier("client", "+5511999990001", 3),
		"4a requisicao deveria ser bloqueada")

	// Identificador diferente tem janela propria
	require.True(t, rateLimitByIdentifier("client", "+5511999990002", 3),
		"identificador diferente nao deve ser afetado")

	// User type diferente tem janela propria
	require.True(t, rateLimitByIdentifier("user", "+5511999990001", 3),
		"user_type diferente nao deve ser afetado")

	// Apos 60s a janela reseta
	s.FastForward(61 * time.Second)
	require.True(t, rateLimitByIdentifier("client", "+5511999990001", 3),
		"janela deveria ter resetado")
}

func TestRateLimitByIdentifier_FallbackMemoria(t *testing.T) {
	// Limpa o Redis e os limiters em memoria
	resetRedisLimiterClient()
	identifierLimitersMu.Lock()
	identifierLimiters = make(map[string]*rate.Limiter)
	identifierLimitersMu.Unlock()
	defer func() {
		identifierLimitersMu.Lock()
		identifierLimiters = make(map[string]*rate.Limiter)
		identifierLimitersMu.Unlock()
	}()

	// Limite 2 por minuto
	require.True(t, rateLimitByIdentifier("client", "fallback-test", 2))
	require.True(t, rateLimitByIdentifier("client", "fallback-test", 2))
	require.False(t, rateLimitByIdentifier("client", "fallback-test", 2),
		"3a deveria bloquear no fallback em memoria")

	// Identificador diferente funciona
	require.True(t, rateLimitByIdentifier("client", "fallback-other", 2))
}

func TestRateLimitByIdentifier_RedisIndisponivel_FailOpen(t *testing.T) {
	_, srv := newRateLimitRedis(t)

	// Redis no ar: funciona normalmente
	require.True(t, rateLimitByIdentifier("user", "redis-test", 1))
	require.False(t, rateLimitByIdentifier("user", "redis-test", 1))

	// Derruba o Redis: deve ser fail-open (permitir)
	_ = srv.Close()
	time.Sleep(50 * time.Millisecond)

	require.True(t, rateLimitByIdentifier("user", "redis-test", 1),
		"com Redis fora, rateLimitByIdentifier deve ser fail-open")
}
