package handlers

import (
	"strings"

	"github.com/carloshomar/fuudelivery/auth_api/app/dto"
	"github.com/carloshomar/fuudelivery/auth_api/app/middlewares"
	"github.com/carloshomar/fuudelivery/auth_api/app/models"
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
	var request struct {
		dto.CreateDeliveryManRequest
		ZoneID    *uint  `json:"zone_id,omitempty"`
		Status    string `json:"status"`
		MaxOrders int    `json:"max_orders"`
	}
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
		Name:      request.Name,
		Email:     request.Email,
		Phone:     request.Phone,
		Password:  string(hashedPassword),
		ZoneID:    request.ZoneID,
		Status:    "offline",
		MaxOrders: 3,
	}

	if request.Status != "" {
		user.Status = request.Status
	}
	if request.MaxOrders > 0 {
		user.MaxOrders = request.MaxOrders
	}

	if err := models.DB.Create(&user).Error; err != nil {
		if strings.Contains(err.Error(), "duplicate key value violates unique constraint") {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "Email already registered"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create user"})
	}

	tokenString, jwtError := middlewares.GenerateJWTDeliveryMan(&user)
	if jwtError != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to generate JWT token"})
	}

	request.Password = ""
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"user": request, "token": tokenString})
}

// UpdateDeliveryMan atualiza os dados de um entregador (PUT /delivery-man/:id).
// Admin. Suporta nome, email, telefone, status, max_orders, zona e senha
// opcional. Campos nao enviados permanecem inalterados.
func UpdateDeliveryMan(c *fiber.Ctx) error {
	deliveryManID := c.Params("id")
	if deliveryManID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid delivery man ID"})
	}

	var request struct {
		Name      string `json:"name"`
		Email     string `json:"email"`
		Phone     string `json:"phone"`
		Status    string `json:"status"`
		ZoneID    *uint  `json:"zone_id"`
		MaxOrders int    `json:"max_orders"`
		Password  string `json:"password"`
	}
	if err := c.BodyParser(&request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Failed to parse request body"})
	}

	var deliveryMan models.DeliveryMan
	if err := models.DB.First(&deliveryMan, deliveryManID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Delivery man not found"})
	}

	updates := map[string]interface{}{}
	if request.Name != "" {
		updates["name"] = request.Name
	}
	if request.Email != "" {
		updates["email"] = request.Email
	}
	if request.Phone != "" {
		updates["phone"] = request.Phone
	}
	if request.Status != "" {
		updates["status"] = request.Status
	}
	if request.MaxOrders > 0 {
		updates["max_orders"] = request.MaxOrders
	}
	if request.ZoneID != nil {
		updates["zone_id"] = *request.ZoneID
	}
	if request.Password != "" {
		if len(request.Password) < 6 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Password must be at least 6 characters"})
		}
		hashedPassword, hashErr := bcrypt.GenerateFromPassword([]byte(request.Password), bcrypt.DefaultCost)
		if hashErr != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to hash password"})
		}
		updates["password"] = string(hashedPassword)
	}

	if len(updates) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "No fields to update"})
	}

	if err := models.DB.Model(&deliveryMan).Updates(updates).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update delivery man"})
	}

	return c.JSON(fiber.Map{"message": "Delivery man updated successfully", "id": deliveryMan.ID})
}

// DeleteDeliveryMan remove um entregador (DELETE /delivery-man/:id). Admin.
func DeleteDeliveryMan(c *fiber.Ctx) error {
	deliveryManID := c.Params("id")
	if deliveryManID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid delivery man ID"})
	}

	var deliveryMan models.DeliveryMan
	if err := models.DB.First(&deliveryMan, deliveryManID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Delivery man not found"})
	}

	if err := models.DB.Delete(&deliveryMan).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to delete delivery man"})
	}

	return c.JSON(fiber.Map{"message": "Delivery man deleted successfully", "id": deliveryMan.ID})
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
		"message":           "Wallet ID updated successfully",
		"payment_wallet_id": deliveryMan.PaymentWalletID,
	})
}
