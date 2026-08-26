package handlers

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"log"
	"strings"
	"time"

	"github.com/carloshomar/fuudelivery/auth_api/app/dto"
	"github.com/carloshomar/fuudelivery/auth_api/app/middlewares"
	"github.com/carloshomar/fuudelivery/auth_api/app/models"
	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// ============================================================================
// Reset de senha assistido ("esqueci minha senha")
//
// Fluxo: usuário travado fala com o suporte → admin gera um código de uso
// único no WebAdmin (POST /admin/password-reset/code) e informa o código por
// telefone/WhatsApp → usuário define a nova senha na página pública
// /resetar-senha do WebRestaurant (POST /auth/reset-password).
//
// Não há serviço de email no projeto e a tabela clients NÃO tem email — o
// canal do código é humano mesmo. Por isso o código é curto, sem caracteres
// ambíguos, com TTL baixo e teto de tentativas.
// ============================================================================

const (
	passwordResetCodeLength  = 8
	passwordResetTTL         = 15 * time.Minute
	maxPasswordResetAttempts = 5
	// Sem 0/O e 1/I para não confundir ditado por telefone.
	passwordResetAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
)

// Mensagem GENÉRICA para falhas do endpoint público: nunca revela se o
// identificador existe ou se o código está errado/expirado (anti-enumeration).
const errInvalidCode = "Código inválido ou expirado. Confira os dados com o suporte."

// generateResetCode gera um código alfanumérico aleatório (crypto/rand).
// O alfabeto tem exatamente 32 caracteres e rand.Read entrega bytes 0–255:
// 256 % 32 == 0, então o módulo NÃO introduz viés.
func generateResetCode() string {
	raw := make([]byte, passwordResetCodeLength)
	if _, err := rand.Read(raw); err != nil {
		log.Printf("[PASSWORD_RESET] ERRO gerando código: %v", err)
		return ""
	}
	for i, b := range raw {
		raw[i] = passwordResetAlphabet[int(b)%len(passwordResetAlphabet)]
	}
	return string(raw)
}

// hashResetCode — só o hash vai para o banco; o código em claro existe uma
// única vez, na resposta do endpoint admin.
func hashResetCode(code string) string {
	sum := sha256.Sum256([]byte(code))
	return hex.EncodeToString(sum[:])
}

// findAccountID resolve o ID da conta a partir do tipo + identificador.
// Retorna (0, false) quando o tipo é desconhecido ou nada foi encontrado —
// o chamador decide como responder (admin pode saber que não achou; o
// endpoint público trata tudo igual).
func findAccountID(userType, identifier string) (uint, bool) {
	ident := strings.TrimSpace(identifier)
	if ident == "" {
		return 0, false
	}

	switch userType {
	case "client":
		var m models.Client
		if err := models.DB.Where("phone = ?", ident).First(&m).Error; err != nil {
			return 0, false
		}
		return m.ID, true
	case "delivery_man":
		var m models.DeliveryMan
		if err := models.DB.Where("phone = ? OR LOWER(email) = ?", ident, strings.ToLower(ident)).First(&m).Error; err != nil {
			return 0, false
		}
		return m.ID, true
	case "user":
		var m models.User
		if err := models.DB.Where("phone = ? OR LOWER(email) = ?", ident, strings.ToLower(ident)).First(&m).Error; err != nil {
			return 0, false
		}
		return m.ID, true
	default:
		return 0, false
	}
}

func isValidResetUserType(t string) bool {
	switch t {
	case "client", "user", "delivery_man":
		return true
	}
	return false
}

// maskIdentifier ofusca o identificador para logs (não vazar PII completa).
func maskIdentifier(s string) string {
	s = strings.TrimSpace(s)
	if len(s) <= 4 {
		return "***"
	}
	return s[:2] + "****" + s[len(s)-2:]
}

// GenerateAdminResetCode — POST /admin/password-reset/code (adminRequired +
// rate limit na rota). Invalida códigos ativos anteriores da mesma conta:
// só o último código gerado vale.
func GenerateAdminResetCode(c *fiber.Ctx) error {
	var req dto.GeneratePasswordResetCodeRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Failed to parse request body"})
	}

	req.UserType = strings.TrimSpace(req.UserType)
	req.Identifier = strings.TrimSpace(req.Identifier)
	if !isValidResetUserType(req.UserType) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "user_type deve ser client, user ou delivery_man"})
	}
	if req.Identifier == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "identifier (telefone ou email) é obrigatório"})
	}

	accountID, ok := findAccountID(req.UserType, req.Identifier)
	if !ok {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Cadastro não encontrado para este tipo/identificador"})
	}

	// Invalida códigos anteriores ainda válidos desta conta.
	now := time.Now().UTC()
	if err := models.DB.Model(&models.PasswordResetToken{}).
		Where("user_type = ? AND user_id = ? AND used_at IS NULL", req.UserType, accountID).
		Update("used_at", now).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to invalidate previous codes"})
	}

	code := generateResetCode()
	if code == "" {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to generate code"})
	}

	tok := &models.PasswordResetToken{
		UserType:  req.UserType,
		UserID:    accountID,
		CodeHash:  hashResetCode(code),
		ExpiresAt: now.Add(passwordResetTTL),
	}
	if err := models.DB.Create(tok).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to store reset code"})
	}

	adminID, _ := middlewares.GetUserIDFromToken(c)
	log.Printf("[PASSWORD_RESET] admin %d gerou código para %s %s (id=%d, expira %s)",
		adminID, req.UserType, maskIdentifier(req.Identifier), accountID, tok.ExpiresAt.Format(time.RFC3339))

	// O código EM CLARO sai aqui UMA única vez — depois só existe o hash.
	return c.JSON(fiber.Map{
		"code":             code,
		"user_type":        req.UserType,
		"expires_at":       tok.ExpiresAt,
		"attempts_allowed": maxPasswordResetAttempts,
		"message":          "Informe o código ao usuário por telefone/WhatsApp. Ele expira em 15 minutos.",
	})
}

// ResetPassword — POST /auth/reset-password (público + rate limit na rota).
// Valida código contra o hash mais recente ATIVO da conta; teto de
// maxPasswordResetAttempts tentativas por código (excedeu → código morto).
// No sucesso: troca a senha (bcrypt), marca o código como usado e revoga os
// refresh tokens quando o tipo é "user" (clientes/entregadores usam JWT puro,
// sem sessão server-side para revogar).
func ResetPassword(c *fiber.Ctx) error {
	var req dto.PasswordResetRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Failed to parse request body"})
	}

	req.UserType = strings.TrimSpace(req.UserType)
	req.Identifier = strings.TrimSpace(req.Identifier)
	req.Code = strings.ToUpper(strings.TrimSpace(req.Code))

	if len(req.NewPassword) < 6 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Nova senha deve ter pelo menos 6 caracteres"})
	}
	if !isValidResetUserType(req.UserType) || req.Code == "" || req.Identifier == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": errInvalidCode})
	}

	accountID, ok := findAccountID(req.UserType, req.Identifier)
	if !ok {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": errInvalidCode})
	}

	var tok models.PasswordResetToken
	err := models.DB.Where(
		"user_type = ? AND user_id = ? AND used_at IS NULL AND expires_at > ?",
		req.UserType, accountID, time.Now().UTC(),
	).Order("id DESC").First(&tok).Error
	if err != nil {
		// Mesma resposta de código errado: não diferencia "nunca existiu".
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": errInvalidCode})
	}

	if subtle.ConstantTimeCompare([]byte(hashResetCode(req.Code)), []byte(tok.CodeHash)) != 1 {
		// Incremento atômico com guarda: quando chega ao teto, mata o código.
		res := models.DB.Model(&tok).
			Where("id = ? AND used_at IS NULL AND attempts < ?", tok.ID, maxPasswordResetAttempts).
			Updates(map[string]interface{}{"attempts": gorm.Expr("attempts + 1")})
		if res.Error != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to process code attempt"})
		}
		if res.RowsAffected == 0 {
			// Teto atingido (ou já usado concorrentemente) — invalida por garantia.
			models.DB.Model(&tok).Where("id = ? AND used_at IS NULL", tok.ID).Update("used_at", time.Now().UTC())
			log.Printf("[PASSWORD_RESET] código %d bloqueado por tentativas (%s %s)", tok.ID, req.UserType, maskIdentifier(req.Identifier))
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Código bloqueado por tentativas excessivas. Solicite um novo ao suporte."})
		}
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": errInvalidCode})
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to hash password"})
	}

	switch req.UserType {
	case "client":
		err = models.DB.Model(&models.Client{}).Where("id = ?", accountID).Update("password", string(hashed)).Error
	case "delivery_man":
		err = models.DB.Model(&models.DeliveryMan{}).Where("id = ?", accountID).Update("password", string(hashed)).Error
	default:
		err = models.DB.Model(&models.User{}).Where("id = ?", accountID).Update("password", string(hashed)).Error
	}
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update password"})
	}

	// Uso único — guarda atômica: duas requisições com o mesmo código não
	// passam as duas pelo WHERE used_at IS NULL.
	models.DB.Model(&tok).Where("id = ? AND used_at IS NULL", tok.ID).Update("used_at", time.Now().UTC())

	// Sessões do painel/usuário: revoga toda a família de refresh tokens.
	// (clients e delivery_men não têm refresh token server-side.)
	if req.UserType == "user" {
		if rErr := models.DB.Model(&models.RefreshToken{}).
			Where("user_id = ? AND revoked = false", accountID).
			Update("revoked", true).Error; rErr != nil {
			log.Printf("[PASSWORD_RESET] AVISO: falha revogando refresh tokens do user %d: %v", accountID, rErr)
		}
	}

	log.Printf("[PASSWORD_RESET] senha redefinida para %s %s (token %d usado)", req.UserType, maskIdentifier(req.Identifier), tok.ID)
	return c.JSON(fiber.Map{"message": "Senha redefinida com sucesso. Faça login com a nova senha."})
}

// CleanupExpiredPasswordResets remove códigos expirados/usados. O histórico
// real fica no audit_log (trigger do sql/13) — esta tabela é só fila quente.
func CleanupExpiredPasswordResets() {
	cutoff := time.Now().UTC().Add(-24 * time.Hour)
	models.DB.Where("expires_at < ? OR used_at IS NOT NULL", cutoff).Delete(&models.PasswordResetToken{})
}
