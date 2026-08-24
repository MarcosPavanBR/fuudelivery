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
	"github.com/valyala/fasthttp"
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
	return reqComIPUA(ip, "")
}

// reqComIPUA monta um POST /login com X-Forwarded-For = ip e User-Agent = ua
// (ua vazio usa o default do httptest, "Go-http-client/1.1").
func reqComIPUA(ip, ua string) *http.Request {
	req := httptest.NewRequest("POST", "/login", nil)
	req.Header.Set("X-Forwarded-For", ip)
	if ua != "" {
		req.Header.Set("User-Agent", ua)
	}
	return req
}

// ---- Chave composta: primeiro IP do XFF + User-Agent ----

// TestFirstForwardedIP extrai o PRIMEIRO IP valido da cadeia XFF, ignorando
// elementos invalidos e IPs anexados pelos proxies.
func TestFirstForwardedIP(t *testing.T) {
	app := testApp()

	// helperIP monta um request com o XFF dado e devolve firstForwardedIP
	// lendo o header do request (mesma fonte que o middleware usa).
	helperIP := func(xff string) string {
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(c)
		c.Request().Header.Set("X-Forwarded-For", xff)
		return firstForwardedIP(c)
	}

	cases := []struct {
		name string
		xff  string
		want string
	}{
		{"cadeia completa", "203.0.113.42, 10.0.0.1", "203.0.113.42"},
		{"um elemento", "198.51.100.7", "198.51.100.7"},
		{"com espacos", "  203.0.113.42 , 10.0.0.1 ", "203.0.113.42"},
		{"vazio", "", ""},
		{"elemento invalido primeiro", "not-an-ip, 8.8.8.8", "8.8.8.8"},
		{"ipv6 com porta", "[2001:db8::1]:443, 10.0.0.1", "2001:db8::1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, helperIP(tc.xff), "firstForwardedIP(%q)", tc.xff)
		})
	}
}

func TestNormalizeUserAgent(t *testing.T) {
	require.Equal(t, "go-http-client/1.1", normalizeUserAgent("Go-http-client/1.1"))
	require.Equal(t, "", normalizeUserAgent("   "))
	// Trunca em 48 chars (UA longo nao vira chave Redis gigante)
	require.Equal(t, "mozilla/5.0 (windows nt 10.0; win64; x64) applew", normalizeUserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0 Safari/537.36"))
}

// TestRateLimitKey_DistintaPorIdentidade garante que a chave Redis varia com
// IP e com User-Agent, mas NAO com o tamanho da cadeia XFF (mesmo cliente
// com proxy que anexa IPs diferentes continua no mesmo bucket).
func TestRateLimitKey_DistintaPorIdentidade(t *testing.T) {
	// Mesma identidade (mesmo primeiro IP + mesmo UA) => mesma chave,
	// mesmo quando o proxy anexa IPs diferentes a cadeia XFF.
	k1 := rateLimitKey("203.0.113.42|mozilla", 10)
	k2 := rateLimitKey("203.0.113.42|mozilla", 10)
	require.Equal(t, k1, k2, "mesma identidade => mesma chave")

	// IP diferente => chave diferente
	require.NotEqual(t, k1, rateLimitKey("198.51.100.7|mozilla", 10), "IP diferente => chave diferente")

	// UA diferente (mesmo IP) => chave diferente — usuarios no mesmo proxy
	// nao colidem
	require.NotEqual(t, k1, rateLimitKey("203.0.113.42|curl", 10), "UA diferente => chave diferente")

	// maxPerMinute diferente => chave diferente (sem colisao entre rotas)
	require.NotEqual(t, k1, rateLimitKey("203.0.113.42|mozilla", 20), "limite diferente => chave diferente")
}

// TestRateLimitMiddleware_ProxyVariavelNaoBurla: o Render pode anexar IPs
// diferentes a cadeia XFF entre requests (ex.: healthcheck ou proxy interno),
// mas como a chave usa o PRIMEIRO IP + UA, o limite continua valendo.
func TestRateLimitMiddleware_ProxyVariavelNaoBurla(t *testing.T) {
	newRateLimitRedis(t)

	app := testApp()
	app.Post("/login", rateLimitMiddleware(3), func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	// 3 requests com o MESMO cliente (primeiro IP + UA), mas cadeias XFF de
	// tamanhos diferentes (como se proxies internos anexassem IPs variados).
	xffs := []string{
		"10.0.0.1",
		"10.0.0.1, 192.168.1.5",
		"10.0.0.1, 192.168.1.5, 172.16.0.9",
	}
	for i, xff := range xffs {
		req := reqComIPUA(xff, "Mozilla/5.0 RateLimitTest")
		resp, err := app.Test(req, 500)
		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode, "req %d com XFF %q deveria passar", i, xff)
	}

	// 4a com a mesma identidade -> 429, mesmo com outra variacao de cadeia
	req := reqComIPUA("10.0.0.1, 203.0.113.99", "Mozilla/5.0 RateLimitTest")
	resp, err := app.Test(req, 500)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusTooManyRequests, resp.StatusCode, "4a requisicao da mesma identidade deveria ser 429")
}

// TestRateLimitMiddleware_UA_DistintoNaoColide: usuarios atras do mesmo proxy
// (mesmo primeiro IP do XFF) com User-Agents diferentes tem buckets separados.
func TestRateLimitMiddleware_UADistintoNaoColide(t *testing.T) {
	newRateLimitRedis(t)

	app := testApp()
	app.Post("/login", rateLimitMiddleware(3), func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	// Esgota o bucket do IP 10.0.0.1 com UA "curl/8"
	for i := 1; i <= 3; i++ {
		resp, err := app.Test(reqComIPUA("10.0.0.1", "curl/8.0"), 500)
		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode, "req %d (curl)", i)
	}
	resp, err := app.Test(reqComIPUA("10.0.0.1", "curl/8.0"), 500)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusTooManyRequests, resp.StatusCode, "curl esgotado -> 429")

	// Mesmo IP, UA diferente (navegador) => bucket proprio, nao bloqueado
	resp, err = app.Test(reqComIPUA("10.0.0.1", "Mozilla/5.0 Chrome"), 500)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode, "UA diferente nao deve herdar o bloqueio do curl")
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
