// Package middleware fornece middlewares HTTP para autenticacao e autorizacao.
package middleware

import (
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/carloshomar/vercardapio/payment/config"
	"github.com/golang-jwt/jwt/v5"
)

// Claims representa as informacoes contidas no token JWT.
type Claims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

// AuthRequired verifica se a requisicao possui um token JWT valido.
// Valida o algoritmo de assinatura para prevenir algorithm confusion attacks.
func AuthRequired() fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Status(401).JSON(fiber.Map{"error": "Authorization header required"})
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			return c.Status(401).JSON(fiber.Map{"error": "Invalid authorization format"})
		}

		tokenString := parts[1]
		claims := &Claims{}

		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			// Previne algorithm confusion: rejeita tokens que nao usam HMAC
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(config.AppConfig.JWTSecret), nil
		})

		if err != nil || !token.Valid {
			return c.Status(401).JSON(fiber.Map{"error": "Invalid token"})
		}

		c.Locals("user_id", claims.UserID)
		c.Locals("email", claims.Email)
		c.Locals("role", claims.Role)

		return c.Next()
	}
}

// AdminRequired verifica se o usuario autenticado tem papel de administrador.
func AdminRequired() fiber.Handler {
	return func(c *fiber.Ctx) error {
		role := c.Locals("role")
		if role != "admin" {
			return c.Status(403).JSON(fiber.Map{"error": "Admin access required"})
		}
		return c.Next()
	}
}
