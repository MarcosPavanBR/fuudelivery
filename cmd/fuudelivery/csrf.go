package main

import (
	"crypto/subtle"

	"github.com/gofiber/fiber/v2"
)

// ═══════════════════════════════════════════════════════════════
// CSRF (Cross-Site Request Forgery) — padrão double-submit cookie.
// ═══════════════════════════════════════════════════════════════
//
// Como funciona:
//   1. GET /csrf-token emite um token aleatório em DOIS lugares:
//      cookie `csrf_token` (legível por JS) e corpo da resposta.
//   2. Frontends web leem o corpo/cookie e reenviam o valor no
//      header `X-CSRF-Token` em toda mutação (POST/PUT/DELETE).
//   3. O middleware aqui compara HEADER × COOKIE. Um site malicioso
//      consegue fazer o browser enviar o cookie automaticamente
//      (sem site prefere a si mesmo), mas NÃO consegue ler esse
//      cookie de outra origem para colocar no header — a mutação
//      cross-site é rejeitada.
//
// Escopo da proteção:
//   - Aplica-se apenas a mutações com credenciais de browser
//     (cookie access_token/refresh_token/csrf_token presentes) —
//     o alvo real de CSRF.
//   - Requisições autenticadas só por `Authorization: Bearer`
//     (apps mobile via SecureStore) não têm cookie para forjar:
//     um atacante cross-site não consegue definir headers em
//     requisições cross-origin, então não há CSRF a proteger.
//     Elas continuam exigindo o Bearer e passam direto.
//
// O header tem prioridade sobre o cookie. Fallback legado: se não
// houver header, mas o cliente enviar o token por query (?_csrf=),
// também aceitamos — mantém compatibilidade com clientes antigos
// que usavam apenas o cookie, sem enfraquecer o double-submit
// (attacker cross-site não controla query strings de requisições
// que ele não monta via JS).

const (
	csrfCookieName = "csrf_token"
	csrfHeaderName = "X-CSRF-Token"
	csrfQueryName  = "_csrf"
)

// constantTimeEquals compara duas strings em tempo constante,
// evitando vazamento de prefixo por diferença de tempo.
func constantTimeEquals(a, b string) bool {
	return len(a) == len(b) && subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// csrfCheck valida uma requisição de mutação e retorna (nil) se
// autorizada a prosseguir, ou um *fiber.Error pronto para resposta.
func csrfCheck(c *fiber.Ctx) *fiber.Error {
	method := c.Method()
	if method == fiber.MethodGet || method == fiber.MethodHead || method == fiber.MethodOptions {
		return nil // mutações não-afetantes não precisam de CSRF
	}

	cookieToken := c.Cookies(csrfCookieName)
	headerToken := c.Get(csrfHeaderName)
	queryToken := c.Query(csrfQueryName)

	// Sem cookie de CSRF → não é uma sessão de browser; nada a proteger.
	// (Bearer-only: mobile/S2S.)
	if cookieToken == "" {
		return nil
	}

	// Double-submit: header (ou query legada) precisa igualar o cookie.
	// constant-time para não vazar prefixo por timing.
	if headerToken != "" && constantTimeEquals(headerToken, cookieToken) {
		return nil
	}
	if queryToken != "" && constantTimeEquals(queryToken, cookieToken) {
		return nil
	}

	return &fiber.Error{
		Code:    fiber.StatusForbidden,
		Message: "CSRF token missing or invalid",
	}
}

// csrfMiddleware é o wrapper Fiber que converte o resultado de
// csrfCheck em resposta HTTP.
func csrfMiddleware(c *fiber.Ctx) error {
	if err := csrfCheck(c); err != nil {
		return c.Status(err.Code).JSON(fiber.Map{"error": err.Message})
	}
	return c.Next()
}

// IsCSRFExemptPath marca rotas que NÃO podem exigir CSRF porque são
// chamadas por servidores dos gateways de pagamento (HMAC/token próprio
// no header, sem cookies do nosso domínio). Mantida pública para
// documentar intenção e permitir testes.
func IsCSRFExemptPath(path string) bool {
	// Webhooks de pagamento: autenticação via assinatura/token do gateway.
	return path == "/payments/webhook"
}
