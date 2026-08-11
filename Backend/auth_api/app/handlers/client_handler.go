package handlers

import (
	"log"
	"os"
	"strings"
	"time"

	"github.com/carloshomar/fuudelivery/auth_api/app/dto"
	"github.com/carloshomar/fuudelivery/auth_api/app/models"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// RegisterClient cadastra um novo cliente no AppComida.
// Recebe name + phone + password, hasheia a senha com bcrypt e salva.
// Retorna JWT token para login imediato.
func RegisterClient(c *fiber.Ctx) error {
	var req dto.RegisterClientRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	// Valida campos obrigatorios
	req.Phone = strings.TrimSpace(req.Phone)
	req.Name = strings.TrimSpace(req.Name)

	if req.Phone == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Phone is required"})
	}
	if req.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Name is required"})
	}
	if len(req.Password) < 6 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Password must be at least 6 characters"})
	}

	// Verifica se o telefone ja esta cadastrado
	var existing models.Client
	if err := models.DB.Where("phone = ?", req.Phone).First(&existing).Error; err == nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "Phone number already registered"})
	}

	// Hash da senha
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("Failed to hash password: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Internal server error"})
	}

	client := models.Client{
		Name:     req.Name,
		Phone:    req.Phone,
		Password: string(hashedPassword),
	}

	if err := models.DB.Create(&client).Error; err != nil {
		log.Printf("Failed to create client: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create account"})
	}

	// Gera JWT para login imediato
	token, err := generateClientJWT(&client)
	if err != nil {
		log.Printf("Failed to generate JWT: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Account created but failed to generate token"})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"token": token,
		"user": fiber.Map{
			"id":    client.ID,
			"name":  client.Name,
			"phone": client.Phone,
		},
	})
}

// LoginClient autentica um cliente pelo telefone + senha.
// Retorna JWT token valido por 7 dias.
func LoginClient(c *fiber.Ctx) error {
	var req dto.LoginClientRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	req.Phone = strings.TrimSpace(req.Phone)
	if req.Phone == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Phone is required"})
	}
	if req.Password == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Password is required"})
	}

	// Busca cliente pelo telefone
	var client models.Client
	if err := models.DB.Where("phone = ?", req.Phone).First(&client).Error; err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid credentials"})
	}

	// Verifica senha
	if err := bcrypt.CompareHashAndPassword([]byte(client.Password), []byte(req.Password)); err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid credentials"})
	}

	// Gera JWT
	token, err := generateClientJWT(&client)
	if err != nil {
		log.Printf("Failed to generate JWT: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Internal server error"})
	}

	return c.JSON(fiber.Map{
		"token": token,
		"user": fiber.Map{
			"id":    client.ID,
			"name":  client.Name,
			"phone": client.Phone,
		},
	})
}

// generateClientJWT gera um token JWT para cliente com expiracao de 7 dias.
func generateClientJWT(client *models.Client) (string, error) {
	expirationTime := time.Now().UTC().Add(time.Hour * 24 * 7).Unix()

	claims := jwt.MapClaims{
		"id":    client.ID,
		"name":  client.Name,
		"phone": client.Phone,
		"role":  "client",
		"exp":   expirationTime,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	tokenString, err := token.SignedString([]byte(os.Getenv("JWT_SECRET")))
	if err != nil {
		return "", err
	}

	return tokenString, nil
}
