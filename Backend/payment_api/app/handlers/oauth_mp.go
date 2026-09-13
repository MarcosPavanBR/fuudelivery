package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/carloshomar/fuudelivery/auth_api/app/middlewares"
	"github.com/carloshomar/fuudelivery/payment_api/app/models"
	"github.com/carloshomar/fuudelivery/pkg/gateway/mercadopago"
	"github.com/carloshomar/fuudelivery/pkg/secretbox"
	"github.com/gofiber/fiber/v2"
)

// OAuth do Mercado Pago para o split na origem.
//
// Dois endpoints:
//
//	GET /payment/gateways/mercadopago/connect   (autenticado — dono do estab.)
//	GET /payment/gateways/mercadopago/callback  (redirect do MP — SEM JWT)
//
// O callback é um redirect do browser vindo do mercadopago.com, então NÃO
// carrega o JWT do dono. A identidade do estabelecimento viaja no parâmetro
// `state`, ASSINADO com HMAC no connect e verificado no callback. Sem a
// assinatura, qualquer um poderia completar um callback apontando a conta MP
// dele para o estabelecimento de outro — ou o próprio para o de um terceiro.
// O state também expira, para um valor vazado não ser reusado depois.

// stateTTL é a janela em que o state assinado é válido. O code do MP dura ~10
// min; 15 min cobre o fluxo com folga sem deixar o state reusável por muito
// tempo.
const stateTTL = 15 * time.Minute

// mpOAuthEnv carrega a configuração de OAuth do ambiente. Falha fechada: sem as
// variáveis, os endpoints respondem 503 em vez de subir um fluxo quebrado.
func mpOAuthEnv() (mercadopago.OAuthConfig, *secretbox.Box, string, error) {
	clientID := os.Getenv("MERCADOPAGO_APP_ID")
	clientSecret := os.Getenv("MERCADOPAGO_CLIENT_SECRET")
	redirect := os.Getenv("MERCADOPAGO_OAUTH_REDIRECT")
	stateSecret := os.Getenv("MERCADOPAGO_OAUTH_STATE_SECRET")

	var faltando []string
	if clientID == "" {
		faltando = append(faltando, "MERCADOPAGO_APP_ID")
	}
	if clientSecret == "" {
		faltando = append(faltando, "MERCADOPAGO_CLIENT_SECRET")
	}
	if redirect == "" {
		faltando = append(faltando, "MERCADOPAGO_OAUTH_REDIRECT")
	}
	if stateSecret == "" {
		faltando = append(faltando, "MERCADOPAGO_OAUTH_STATE_SECRET")
	}
	if len(faltando) > 0 {
		return mercadopago.OAuthConfig{}, nil, "", fmt.Errorf("OAuth do Mercado Pago não configurado (faltam: %s)", strings.Join(faltando, ", "))
	}

	box, err := secretbox.FromEnv("SECRET_ENCRYPTION_KEY")
	if err != nil {
		return mercadopago.OAuthConfig{}, nil, "", fmt.Errorf("cofre de cifragem indisponível: %w", err)
	}

	cfg := mercadopago.OAuthConfig{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURI:  redirect,
		AuthBaseURL:  os.Getenv("MERCADOPAGO_OAUTH_AUTH_URL"), // vazio → default de produção
	}
	return cfg, box, stateSecret, nil
}

// signState monta um state assinado: base64url(estID.expiryUnix.nonce).hexHMAC.
// O HMAC cobre o payload inteiro; qualquer alteração invalida.
func signState(estID int64, secret string, now time.Time) string {
	nonce := make([]byte, 12)
	_, _ = rand.Read(nonce)
	payload := fmt.Sprintf("%d.%d.%s", estID, now.Add(stateTTL).Unix(), base64.RawURLEncoding.EncodeToString(nonce))
	sig := stateHMAC(payload, secret)
	return base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." + sig
}

// verifyState confere a assinatura e a validade, devolvendo o establishment_id.
func verifyState(state, secret string, now time.Time) (int64, error) {
	partes := strings.SplitN(state, ".", 2)
	if len(partes) != 2 {
		return 0, fmt.Errorf("state malformado")
	}
	payloadRaw, err := base64.RawURLEncoding.DecodeString(partes[0])
	if err != nil {
		return 0, fmt.Errorf("state malformado")
	}
	payload := string(payloadRaw)
	sigEsperada := stateHMAC(payload, secret)

	// Comparação em tempo constante: o sig é a trava, comparar com == vaza
	// tamanho por tempo. Mesmo cuidado dos webhooks (hmac.Equal).
	if !hmac.Equal([]byte(partes[1]), []byte(sigEsperada)) {
		return 0, fmt.Errorf("assinatura do state inválida")
	}

	campos := strings.Split(payload, ".")
	if len(campos) != 3 {
		return 0, fmt.Errorf("payload do state inválido")
	}
	estID, err := strconv.ParseInt(campos[0], 10, 64)
	if err != nil || estID <= 0 {
		return 0, fmt.Errorf("establishment_id inválido no state")
	}
	expiry, err := strconv.ParseInt(campos[1], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("expiry inválido no state")
	}
	if now.Unix() > expiry {
		return 0, fmt.Errorf("state expirado")
	}
	return estID, nil
}

func stateHMAC(payload, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

// ConnectMercadoPago devolve a URL de autorização para o dono do estabelecimento
// conectar a conta MP dele. Autenticado: o establishment_id vem do JWT.
//
// GET /payment/gateways/mercadopago/connect
func ConnectMercadoPago(c *fiber.Ctx) error {
	cfg, _, stateSecret, err := mpOAuthEnv()
	if err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": err.Error()})
	}

	// Autorização por recurso: o dono conecta a PRÓPRIA conta. Admin pode
	// conectar em nome de um estabelecimento passando ?establishment_id=.
	estID, err := middlewares.GetEstablishmentIDFromToken(c)
	if err != nil || estID <= 0 {
		role, _ := middlewares.GetUserRoleFromToken(c)
		if role == "admin" {
			if q := c.Query("establishment_id"); q != "" {
				estID, _ = strconv.ParseInt(q, 10, 64)
			}
		}
	}
	if estID <= 0 {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "sem establishment_id no token"})
	}

	state := signState(estID, stateSecret, time.Now())
	return c.JSON(fiber.Map{"authorize_url": cfg.AuthorizeURL(state)})
}

// MercadoPagoCallback recebe o redirect do MP, troca o code por tokens, cifra e
// grava o recebedor. SEM JWT — a identidade vem do state assinado.
//
// GET /payment/gateways/mercadopago/callback?code=...&state=...
func MercadoPagoCallback(c *fiber.Ctx) error {
	cfg, box, stateSecret, err := mpOAuthEnv()
	if err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": err.Error()})
	}

	// Erro devolvido pelo próprio MP (o lojista negou, por ex.).
	if mpErr := c.Query("error"); mpErr != "" {
		log.Printf("[MP-OAUTH] callback com erro do MP: %s", mpErr)
		return redirectResult(c, false)
	}

	code := c.Query("code")
	state := c.Query("state")
	if code == "" || state == "" {
		return redirectResult(c, false)
	}

	estID, err := verifyState(state, stateSecret, time.Now())
	if err != nil {
		// State inválido/expirado/forjado: nunca conectar. Não revela o motivo
		// ao browser.
		log.Printf("[MP-OAUTH] state rejeitado: %v", err)
		return redirectResult(c, false)
	}

	toks, err := cfg.Exchange(context.Background(), code)
	if err != nil {
		log.Printf("[MP-OAUTH] troca de code falhou (estab %d): %v", estID, err)
		return redirectResult(c, false)
	}

	r := &models.Recipient{
		UserType:           "establishment",
		UserID:             estID,
		Gateway:            "mercadopago",
		GatewayRecipientID: toks.MPUserIDString(),
		Status:             models.RecipientActive,
		MPUserID:           toks.MPUserIDString(),
	}
	if err := r.SetTokens(box, toks.AccessToken, toks.RefreshToken, toks.ExpiresAt()); err != nil {
		log.Printf("[MP-OAUTH] cifrar tokens falhou (estab %d): %v", estID, err)
		return redirectResult(c, false)
	}
	if err := models.UpsertRecipient(models.DB, r); err != nil {
		log.Printf("[MP-OAUTH] gravar recebedor falhou (estab %d): %v", estID, err)
		return redirectResult(c, false)
	}

	log.Printf("[MP-OAUTH] estabelecimento %d conectou a conta MP %s", estID, toks.MPUserIDString())
	return redirectResult(c, true)
}

// StatusMercadoPago informa se o estabelecimento tem a conta MP conectada, para
// a tela do painel mostrar o estado. Read-only, autorizado por recurso.
//
// GET /payment/gateways/mercadopago/status
func StatusMercadoPago(c *fiber.Ctx) error {
	estID, err := middlewares.GetEstablishmentIDFromToken(c)
	if err != nil || estID <= 0 {
		role, _ := middlewares.GetUserRoleFromToken(c)
		if role == "admin" {
			if q := c.Query("establishment_id"); q != "" {
				estID, _ = strconv.ParseInt(q, 10, 64)
			}
		}
	}
	if estID <= 0 {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "sem establishment_id no token"})
	}

	r, err := models.FindRecipient(models.DB, "mercadopago", "establishment", estID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "falha ao consultar recebedor"})
	}
	if r == nil {
		return c.JSON(fiber.Map{"connected": false})
	}

	expired := r.TokenExpired(time.Now())
	return c.JSON(fiber.Map{
		"connected":  r.Status == models.RecipientActive && !expired,
		"status":     r.Status,
		"expired":    expired,
		"mp_user_id": r.MPUserID,
		"expires_at": r.TokenExpiresAt,
	})
}

// redirectResult manda o browser de volta ao painel com um marcador de
// sucesso/erro. A URL base vem do env; sem ela, responde JSON (útil em teste).
func redirectResult(c *fiber.Ctx, ok bool) error {
	base := os.Getenv("MERCADOPAGO_OAUTH_RESULT_URL")
	status := "ok"
	if !ok {
		status = "erro"
	}
	if base == "" {
		code := fiber.StatusOK
		if !ok {
			code = fiber.StatusBadRequest
		}
		return c.Status(code).JSON(fiber.Map{"mercadopago_connect": status})
	}
	sep := "?"
	if strings.Contains(base, "?") {
		sep = "&"
	}
	return c.Redirect(base+sep+"mercadopago="+status, fiber.StatusFound)
}
