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
//   - Aplica-se a toda mutação que chega com COOKIE DE SESSÃO
//     (access_token/refresh_token) — o alvo real de CSRF.
//   - Requisições autenticadas só por `Authorization: Bearer`
//     (apps mobile via SecureStore) não têm cookie para forjar:
//     um atacante cross-site não consegue definir headers em
//     requisições cross-origin, então não há CSRF a proteger.
//     Elas continuam exigindo o Bearer e passam direto.
//
// O gatilho é o cookie de SESSÃO, não o cookie de CSRF. Isso é
// deliberado e corrige um furo real: quando o teste era
// `if csrf_token == "" { libera }`, uma sessão de browser sem o cookie
// de CSRF passava direto. E isso acontecia sozinho — o csrf_token
// expirava antes da sessão, então a proteção se desligava com o tempo.
// Enquanto os cookies eram SameSite=Strict o navegador nem os mandava
// cross-site e o furo era teórico; com SameSite=None (necessário porque
// frontend e API vivem em subdomínios .onrender.com distintos) este
// passou a ser o gate lógico principal, e precisa fechar.
//
// Não há fallback por query string (?_csrf=): token em URL vaza para
// log de acesso, Referer e histórico — quem lê o log ganha um token
// válido, que é justamente o que o double-submit tenta impedir.

const (
	csrfCookieName    = "csrf_token"
	csrfHeaderName    = "X-CSRF-Token"
	accessCookieName  = "access_token"
	refreshCookieName = "refresh_token"
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

	// Webhooks de gateway autenticam por assinatura própria e não têm
	// cookies do nosso domínio — isentos explicitamente.
	if IsCSRFExemptPath(c.Path()) {
		return nil
	}

	cookieToken := c.Cookies(csrfCookieName)
	headerToken := c.Get(csrfHeaderName)

	hasSession := c.Cookies(accessCookieName) != "" || c.Cookies(refreshCookieName) != ""

	// Sem NENHUM cookie nosso → não é requisição de browser com credencial
	// ambiente; não há o que um site terceiro possa forjar.
	// (Bearer-only: mobile/S2S.)
	if !hasSession && cookieToken == "" {
		return nil
	}

	// A partir daqui exige-se o double-submit. Note que basta haver sessão:
	// cookie de CSRF ausente agora REJEITA em vez de liberar — era esse o
	// furo, porque o csrf_token expirava antes da sessão e a proteção se
	// desligava sozinha com o tempo.
	if cookieToken == "" || headerToken == "" {
		return &fiber.Error{
			Code:    fiber.StatusForbidden,
			Message: "CSRF token missing or invalid",
		}
	}

	// constant-time para não vazar prefixo por timing.
	if !constantTimeEquals(headerToken, cookieToken) {
		return &fiber.Error{
			Code:    fiber.StatusForbidden,
			Message: "CSRF token missing or invalid",
		}
	}

	return nil
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
