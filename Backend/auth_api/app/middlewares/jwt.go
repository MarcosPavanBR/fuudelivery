package middlewares

import (
	"errors"
	"log"
	"os"
	"time"

	"github.com/carloshomar/vercardapio/auth_api/app/models"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

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

func GenerateJWT(user *models.User, establishment *models.Establishment) (string, error) {
	// Expiração para 7 dias a partir de agora (hora UTC)
	expirationTime := time.Now().UTC().Add(time.Hour * 24 * 7).Unix()

	claims := jwt.MapClaims{
		"id":    user.ID,
		"name":  user.Name,
		"email": user.Email,
		"role":  user.Role,
		"exp":   expirationTime,
	}

	if establishment != nil {
		claims["establishment_id"] = establishment.ID
		claims["establishment_name"] = establishment.Name
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	tokenString, err := token.SignedString([]byte(os.Getenv("JWT_SECRET")))
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

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
