package handlers

import (
	"fmt"

	"github.com/carloshomar/fuudelivery/auth_api/app/audit"
	"github.com/carloshomar/fuudelivery/auth_api/app/dto"
	"github.com/carloshomar/fuudelivery/auth_api/app/middlewares"
	"github.com/carloshomar/fuudelivery/auth_api/app/models"
	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"
)

// ListAllDeliveryMen lista entregadores com paginacao server-side (admin).
//
//	GET /delivery-man?page=1&limit=20&q=nome
//
// Resposta: {data, total, page, limit, total_pages}.
func ListAllDeliveryMen(c *fiber.Ctx) error {
	tx := models.DB.Model(&models.DeliveryMan{})
	if q := c.Query("q"); q != "" {
		like := "%" + q + "%"
		tx = tx.Where("(name ILIKE ? OR email ILIKE ? OR phone ILIKE ?)", like, like, like)
	}

	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to count delivery men"})
	}

	page := c.QueryInt("page", 1)
	if page < 1 {
		page = 1
	}
	limit := c.QueryInt("limit", 20)
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	var deliveryMen []models.DeliveryMan
	if err := tx.Order("id DESC").Offset((page - 1) * limit).Limit(limit).Find(&deliveryMen).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to query delivery men"})
	}
	if deliveryMen == nil {
		deliveryMen = []models.DeliveryMan{}
	}

	totalPages := (total + int64(limit) - 1) / int64(limit)
	return c.JSON(fiber.Map{
		"data":        deliveryMen,
		"total":       total,
		"page":        page,
		"limit":       limit,
		"total_pages": totalPages,
	})
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

	audit.Record(c, models.DB, "DELIVERY_MAN_DELETED", "delivery_man", fmt.Sprintf("%d", deliveryMan.ID), map[string]interface{}{
		"name":  deliveryMan.Name,
		"email": deliveryMan.Email,
	})

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
