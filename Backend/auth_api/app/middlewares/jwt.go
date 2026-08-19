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

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(os.Getenv("JWT_SECRET")), nil
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

	tokenString, err := token.SignedString([]byte(os.Getenv("JWT_SECRET")))
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

// CleanupExpiredRefreshTokens remove refresh tokens expirados do banco.
// Pode ser chamado periodicamente para manter a tabela limpa.
func CleanupExpiredRefreshTokens() {
	models.DB.Where("expires_at < ? OR revoked = true", time.Now().UTC()).Delete(&models.RefreshToken{})
}

// GenerateJWTDeliveryMan gera um token JWT para um entregador.
// Similar ao GenerateJWT, mas sem campo establishment_id.
func GenerateJWTDeliveryMan(user *models.DeliveryMan) (string, error) {
	expirationTime := time.Now().UTC().Add(ACCESS_TOKEN_DURATION).Unix()

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"id":    user.ID,
		"name":  user.Name,
		"email": user.Email,
		"phone": user.Phone,
		"exp":   expirationTime,
	})

	tokenString, err := token.SignedString([]byte(os.Getenv("JWT_SECRET")))
	if err != nil {
		return "", err
	}

	return tokenString, nil
}
