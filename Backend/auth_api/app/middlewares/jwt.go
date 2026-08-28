// Package middlewares fornece middleware de autenticacao JWT.
// Inclui validacao, geracao e extracao de claims de tokens.
package middlewares

import (
	"errors"
	"log"
	"os"
	"time"

	"github.com/carloshomar/fuudelivery/auth_api/app/models"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

// ValidateJWT valida o token JWT da requisicao HTTP.
// Extrai o token do header Authorization (formato 'Bearer <token>'),
// valida a assinatura HMAC-SHA256 e retorna o token parsed.
// Retorna erro 401 se o token for invalido ou estiver expirado.
func ValidateJWT(c *fiber.Ctx) (*jwt.Token, error) {
	tokenString := c.Get("Authorization")
	if len(tokenString) > 7 {
		tokenString = tokenString[7:]
	}

	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return nil, fiber.NewError(fiber.StatusInternalServerError, "JWT secret not configured")
	}

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(secret), nil
	})

	if err != nil {
		log.Printf("Error parsing token: %v", err)
		return nil, fiber.NewError(fiber.StatusUnauthorized, "Invalid token")
	}

	if token.Valid {
		return token, nil
	}

	return nil, fiber.NewError(fiber.StatusUnauthorized, "Invalid token")
}

func GenerateJWT(user *models.User, establishment *models.Establishment) (string, error) { // GenerateJWT gera um token JWT de curta duração (15 min).
	// O refresh token (longa duração, 30 dias) é gerado e armazenado
	// separadamente no banco via CreateRefreshToken. O token contem:
	// id, name, email, role, establishment_id (se aplicavel).
	// Assinado com HS256 usando JWT_SECRET.
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return "", fmt.Errorf("JWT_SECRET not configured")
	}

	expirationTime := time.Now().UTC().Add(15 * time.Minute).Unix()

	claims := jwt.MapClaims{
		"id":         user.ID,
		"name":       user.Name,
		"email":      user.Email,
		"role":       user.Role,
		"phone":      user.Phone,
		"avatar_url": user.AvatarURL,
		"exp":        expirationTime,
	}

	if establishment != nil {
		claims["establishment_id"] = establishment.ID
		claims["establishment_name"] = establishment.Name
		// Claim aninhado para o WebRestaurant (user.establishment.id no frontend).
		claims["establishment"] = map[string]interface{}{
			"id":   establishment.ID,
			"name": establishment.Name,
		}
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

// GetUserIDFromToken extrai o ID do usuario autenticado do token JWT.
// Retorna erro 401 se o token for invalido ou o claim 'id' nao existir.
func GetUserIDFromToken(c *fiber.Ctx) (int64, error) {
	token, err := ValidateJWT(c)
	if err != nil {
		return 0, err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return 0, fiber.NewError(fiber.StatusUnauthorized, "Invalid token claims")
	}

	idFloat, ok := claims["id"].(float64)
	if !ok {
		return 0, fiber.NewError(fiber.StatusUnauthorized, "User ID not found in token")
	}

	return int64(idFloat), nil
}

// GetEstablishmentIDFromToken extrai o ID do estabelecimento do token JWT.
// Retorna erro 403 se o usuario nao pertencer a nenhum estabelecimento.
func GetEstablishmentIDFromToken(c *fiber.Ctx) (int64, error) {
	token, err := ValidateJWT(c)
	if err != nil {
		return 0, err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return 0, fiber.NewError(fiber.StatusUnauthorized, "Invalid token claims")
	}

	estIDFloat, ok := claims["establishment_id"].(float64)
	if !ok {
		return 0, fiber.NewError(fiber.StatusForbidden, "Establishment ID not found in token")
	}

	return int64(estIDFloat), nil
}

// GetUserRoleFromToken extrai o papel (role) do usuario do token JWT.
// Valores possiveis: 'admin', 'restaurant', 'deliverer', 'client'.
func GetUserRoleFromToken(c *fiber.Ctx) (string, error) {
	token, err := ValidateJWT(c)
	if err != nil {
		return "", err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", fiber.NewError(fiber.StatusUnauthorized, "Invalid token claims")
	}

	role, _ := claims["role"].(string)
	return role, nil
}

// GetUserPhoneFromToken extrai o telefone do usuario autenticado do token JWT.
// Usado para garantir que operações sensíveis (ex.: pontos de fidelidade)
// usem a identidade do token e não um phone arbitrário do body.
func GetUserPhoneFromToken(c *fiber.Ctx) (string, error) {
	token, err := ValidateJWT(c)
	if err != nil {
		return "", err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", fiber.NewError(fiber.StatusUnauthorized, "Invalid token claims")
	}

	phone, _ := claims["phone"].(string)
	if phone == "" {
		return "", fiber.NewError(fiber.StatusForbidden, "Phone not found in token")
	}
	return phone, nil
}

// ACCESS_TOKEN_DURATION é a validade do token de acesso (curta duração).
const ACCESS_TOKEN_DURATION = 15 * time.Minute

// REFRESH_TOKEN_DURATION é a validade do refresh token (longa duração).
const REFRESH_TOKEN_DURATION = 30 * 24 * time.Hour // 30 dias

// CreateRefreshToken cria e persiste um refresh token no banco.
// Retorna o token string (para enviar ao frontend) e erro.
func CreateRefreshToken(userID uint) (string, error) {
	tokenValue, err := models.GenerateRefreshTokenValue()
	if err != nil {
		return "", err
	}

	refreshToken := models.RefreshToken{
		UserID:    userID,
		Token:     tokenValue,
		ExpiresAt: time.Now().UTC().Add(REFRESH_TOKEN_DURATION),
	}

	if err := models.DB.Create(&refreshToken).Error; err != nil {
		return "", err
	}

	return tokenValue, nil
}

// RevokeRefreshToken marca um refresh token como revogado (logout).
func RevokeRefreshToken(token string) error {
	return models.DB.Model(&models.RefreshToken{}).
		Where("token = ? AND revoked = false", token).
		Update("revoked", true).Error
}

// ValidateRefreshToken valida um refresh token: existe, não expirado, não revogado.
// Retorna o user_id associado ao token.
func ValidateRefreshToken(token string) (uint, error) {
	var rt models.RefreshToken
	err := models.DB.Where("token = ? AND revoked = false", token).First(&rt).Error
	if err != nil {
		return 0, errors.New("refresh token not found or revoked")
	}

	if time.Now().UTC().After(rt.ExpiresAt) {
		return 0, errors.New("refresh token expired")
	}

	return rt.UserID, nil
}

// ErrRefreshReuse indica uso de um refresh token já rotacionado/revogado —
// padrão clássico de token roubado sendo reproduzido em paralelo ao dono.
var ErrRefreshReuse = errors.New("refresh token reutilizado")

// RotateRefreshToken faz a rotação ATÔMICA do par de sessão:
//
//  1. Claim único: UPDATE ... WHERE revoked = false — só UMA requisição
//     consegue revogar o token; duas renovações concorrentes com o mesmo
//     token não geram mais dois pares válidos.
//  2. Reuse detection: se o claim não afetou linhas, o token já estava
//     revogado (replay pós-rotação). Nesse caso REVOGA TODA a família do
//     usuário, contendo o atacante que copiou o token antes do dono.
//
// Retorna o user_id e o NOVO refresh token já persistido.
func RotateRefreshToken(token string) (uint, string, error) {
	res := models.DB.Model(&models.RefreshToken{}).
		Where("token = ? AND revoked = false", token).
		Update("revoked", true)
	if res.Error != nil {
		return 0, "", res.Error
	}
	if res.RowsAffected == 0 {
		var rt models.RefreshToken
		if err := models.DB.Where("token = ?", token).First(&rt).Error; err == nil {
			models.DB.Model(&models.RefreshToken{}).
				Where("user_id = ? AND revoked = false", rt.UserID).
				Update("revoked", true)
			log.Printf("[AUTH] REFRESH REUSE detectado (user %d): família inteira revogada", rt.UserID)
		}
		return 0, "", ErrRefreshReuse
	}

	var rt models.RefreshToken
	if err := models.DB.Where("token = ?", token).First(&rt).Error; err != nil {
		return 0, "", err
	}
	if time.Now().UTC().After(rt.ExpiresAt) {
		return 0, "", errors.New("refresh token expired")
	}

	newToken, err := CreateRefreshToken(rt.UserID)
	if err != nil {
		return 0, "", err
	}
	return rt.UserID, newToken, nil
}

// CleanupExpiredRefreshTokens remove refresh tokens expirados do banco.
// Pode ser chamado periodicamente para manter a tabela limpa.
func CleanupExpiredRefreshTokens() {
	models.DB.Where("expires_at < ? OR revoked = true", time.Now().UTC()).Delete(&models.RefreshToken{})
}

// GenerateJWTDeliveryMan gera um token JWT para um entregador.
// Similar ao GenerateJWT, mas sem campo establishment_id.
func GenerateJWTDeliveryMan(user *models.DeliveryMan) (string, error) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return "", fmt.Errorf("JWT_SECRET not configured")
	}

	expirationTime := time.Now().UTC().Add(ACCESS_TOKEN_DURATION).Unix()

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"id":    user.ID,
		"name":  user.Name,
		"email": user.Email,
		"phone": user.Phone,
		"exp":   expirationTime,
	})

	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", err
	}

	return tokenString, nil
}
