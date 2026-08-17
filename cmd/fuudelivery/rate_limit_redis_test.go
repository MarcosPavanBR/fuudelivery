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
