package handlers

import (
	"errors"
	"log"
	"os"
	"time"

	"github.com/carloshomar/fuudelivery/auth_api/app/dto"
	"github.com/carloshomar/fuudelivery/auth_api/app/middlewares"
	"github.com/carloshomar/fuudelivery/auth_api/app/models"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// setAuthCookies define cookies HttpOnly para access e refresh tokens.
// Usado por Login, RefreshToken e LoginDeliveryMan.
func setAuthCookies(c *fiber.Ctx, accessToken, refreshToken string) {
	accessMaxAge := int(15 * time.Minute.Seconds())
	refreshMaxAge := int(30 * 24 * time.Hour.Seconds())

	c.Cookie(&fiber.Cookie{
		Name:     "access_token",
		Value:    accessToken,
		HTTPOnly: true,
		Secure:   true,
		SameSite: "None",
		MaxAge:   accessMaxAge,
		Path:     "/",
	})

	c.Cookie(&fiber.Cookie{
		Name:     "refresh_token",
		Value:    refreshToken,
		HTTPOnly: true,
		Secure:   true,
		SameSite: "None",
		MaxAge:   refreshMaxAge,
		Path:     "/",
	})
}

// clearAuthCookies limpa os cookies de autenticação.
// Usado por Logout e LogoutDeliveryMan.
func clearAuthCookies(c *fiber.Ctx) {
	c.Cookie(&fiber.Cookie{
		Name:     "access_token",
		Value:    "",
		HTTPOnly: true,
		Secure:   true,
		SameSite: "None",
		MaxAge:   -1,
		Path:     "/",
	})

	c.Cookie(&fiber.Cookie{
		Name:     "refresh_token",
		Value:    "",
		HTTPOnly: true,
		Secure:   true,
		SameSite: "None",
		MaxAge:   -1,
		Path:     "/",
	})
}

// SessionLogin faz login e retorna tokens via cookies HttpOnly.
// POST /auth/session {email, password}
func SessionLogin(c *fiber.Ctx) error {
	var request dto.LoginRequest
	if err := c.BodyParser(&request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Failed to parse request body"})
	}

	var user models.User
	if err := models.DB.Where(&models.User{Email: request.Email}).First(&user).Error; err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Incorrect credentials"})
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(request.Password)); err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Incorrect credentials"})
	}

	var establishmentPtr *models.Establishment
	if user.EstablishmentID != 0 {
		var establishment models.Establishment
		if err := models.DB.Where("id = ?", user.EstablishmentID).First(&establishment).Error; err != nil {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Incorrect credentials"})
		}
		establishmentPtr = &establishment
	}

	accessToken, refreshToken, jwtError := createTokenPair(&user, establishmentPtr)
	if jwtError != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Incorrect credentials"})
	}

	setAuthCookies(c, accessToken, refreshToken)

	return c.JSON(fiber.Map{
		"message": "Login successful",
		"user": fiber.Map{
			"id":               user.ID,
			"name":             user.Name,
			"email":            user.Email,
			"phone":            user.Phone,
			"role":             user.Role,
			"establishment_id": user.EstablishmentID,
		},
	})
}

// SessionLogout limpa os cookies de autenticação e revoga o refresh token.
// POST /auth/session/logout
func SessionLogout(c *fiber.Ctx) error {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := c.BodyParser(&req); err != nil {
		clearAuthCookies(c)
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": "Logged out successfully"})
	}

	if req.RefreshToken != "" {
		if err := middlewares.RevokeRefreshToken(req.RefreshToken); err != nil {
			log.Printf("[WARN] Failed to revoke refresh token on logout: %v", err)
		}
	}

	clearAuthCookies(c)
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": "Logged out successfully"})
}

// SessionRefresh renova o access token via cookie e retorna novo cookie.
// POST /auth/session/refresh
func SessionRefresh(c *fiber.Ctx) error {
	refreshToken := c.Cookies("refresh_token")
	if refreshToken == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Refresh token required"})
	}

	userID, newRefreshToken, rErr := middlewares.RotateRefreshToken(refreshToken)
	if errors.Is(rErr, middlewares.ErrRefreshReuse) {
		clearAuthCookies(c)
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Sessão encerrada por segurança. Faça login novamente.",
		})
	}
	if rErr != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid or expired refresh token"})
	}

	var user models.User
	if err := models.DB.First(&user, userID).Error; err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "User not found"})
	}

	var establishmentPtr *models.Establishment
	if user.EstablishmentID != 0 {
		var establishment models.Establishment
		if err := models.DB.Where("id = ?", user.EstablishmentID).First(&establishment).Error; err == nil {
			establishmentPtr = &establishment
		}
	}

	accessToken, err := middlewares.GenerateJWT(&user, establishmentPtr)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to generate token"})
	}

	setAuthCookies(c, accessToken, newRefreshToken)

	return c.JSON(fiber.Map{
		"message": "Token refreshed successfully",
		"user": fiber.Map{
			"id":               user.ID,
			"name":             user.Name,
			"email":            user.Email,
			"phone":            user.Phone,
			"role":             user.Role,
			"establishment_id": user.EstablishmentID,
		},
	})
}

// SessionMe retorna os dados do usuário logado a partir do cookie access_token.
// GET /auth/session/me — usado pelo frontend para restaurar o estado após
// page refresh (cookies HttpOnly não são legíveis via JS).
func SessionMe(c *fiber.Ctx) error {
	accessToken := c.Cookies("access_token")
	if accessToken == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Not authenticated"})
	}

	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "JWT secret not configured"})
	}

	token, err := jwt.Parse(accessToken, func(token *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	if err != nil || !token.Valid {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid session"})
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid session claims"})
	}

	idFloat, ok := claims["id"].(float64)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "User ID not found in session"})
	}

	var user models.User
	if err := models.DB.First(&user, int64(idFloat)).Error; err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "User not found"})
	}

	return c.JSON(fiber.Map{
		"user": fiber.Map{
			"id":               user.ID,
			"name":             user.Name,
			"email":            user.Email,
			"phone":            user.Phone,
			"role":             user.Role,
			"establishment_id": user.EstablishmentID,
		},
	})
}
