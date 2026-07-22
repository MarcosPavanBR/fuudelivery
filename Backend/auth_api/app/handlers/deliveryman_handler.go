package handlers

import (
	"github.com/carloshomar/vercardapio/auth_api/app/dto"
	"github.com/carloshomar/vercardapio/auth_api/app/middlewares"
	"github.com/carloshomar/vercardapio/auth_api/app/models"
	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"
)

func ListAllDeliveryMen(c *fiber.Ctx) error {
	var deliveryMen []models.DeliveryMan
	if err := models.DB.Find(&deliveryMen).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to query delivery men"})
	}
	return c.JSON(deliveryMen)
}

func LoginDeliveryMan(c *fiber.Ctx) error {
	var request dto.LoginRequest
	if err := c.BodyParser(&request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Failed to parse request body"})
	}

	var user models.DeliveryMan
	if err := models.DB.Where(&models.DeliveryMan{Email: request.Email}).First(&user).Error; err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Incorrect credentials"})
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(request.Password)); err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Incorrect credentials"})
	}

	tokenString, jwtError := middlewares.GenerateJWTDeliveryMan(&user)
	if jwtError != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to generate token"})
	}

	return c.JSON(fiber.Map{"token": tokenString})
}

func CreateDeliveryMan(c *fiber.Ctx) error {
	var request dto.CreateDeliveryManRequest
	if err := c.BodyParser(&request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Failed to parse request body"})
	}

	var existingUser models.DeliveryMan
	if err := models.DB.Where(&models.DeliveryMan{Email: request.Email}).First(&existingUser).Error; err == nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "Email already exists"})
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(request.Password), bcrypt.DefaultCost)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to hash password"})
	}

	user := models.DeliveryMan{
		Name:     request.Name,
		Email:    request.Email,
		Phone:    request.Phone,
		Password: string(hashedPassword),
	}

	if err := models.DB.Create(&user).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create user"})
	}

	tokenString, jwtError := middlewares.GenerateJWTDeliveryMan(&user)
	if jwtError != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to generate JWT token"})
	}

	request.Password = ""
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"user": request, "token": tokenString})
}

func UpdateDeliveryManWallet(c *fiber.Ctx) error {
	deliveryManID := c.Params("id")
	if deliveryManID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid delivery man ID"})
	}

	var req struct {
		PaymentWalletID string `json:"payment_wallet_id"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if req.PaymentWalletID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "payment_wallet_id is required"})
	}

	var deliveryMan models.DeliveryMan
	if err := models.DB.First(&deliveryMan, deliveryManID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Delivery man not found"})
	}

	deliveryMan.PaymentWalletID = req.PaymentWalletID
	if err := models.DB.Save(&deliveryMan).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update wallet"})
	}

	return c.JSON(fiber.Map{
		"message": "Wallet ID updated successfully",
		"payment_wallet_id": deliveryMan.PaymentWalletID,
	})
}
