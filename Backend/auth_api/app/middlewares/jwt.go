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

func GenerateJWT(user *models.User, establishment *models.Establishment) (string, error) { // GenerateJWT gera um token JWT para um usuario autenticado.
	// O token contem: id, name, email, role, establishment_id (se aplicavel),
	// e expira em 15 minutos. Assinado com HS256 usando JWT_SECRET.
	// Expiração curta reduz o risco de vazamento em query strings / logs.
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

// GenerateJWTDeliveryMan gera um token JWT para um entregador.
// Similar ao GenerateJWT, mas sem campo establishment_id.
func GenerateJWTDeliveryMan(user *models.DeliveryMan) (string, error) {
	expirationTime := time.Now().UTC().Add(time.Hour * 24 * 7).Unix()

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
