package handlers

import (
	"errors"
	"strconv"
	"strings"

	"github.com/carloshomar/fuudelivery/auth_api/app/dto"
	"github.com/carloshomar/fuudelivery/auth_api/app/middlewares"
	"github.com/carloshomar/fuudelivery/auth_api/app/models"
	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func ListAllDeliveryMen(c *fiber.Ctx) error {
	role, err := middlewares.GetUserRoleFromToken(c)
	if err != nil || role != "admin" {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Admin access required"})
	}

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

	// E-mail vazio não pode casar: Where com struct ignora o campo vazio e
	// devolvia o primeiro entregador da tabela. Senha vazia também nunca
	// entra (contas antigas criadas pelo painel sem senha).
	email := strings.TrimSpace(request.Email)
	if email == "" || request.Password == "" {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Incorrect credentials"})
	}
	var user models.DeliveryMan
	err := models.DB.Where("email = ?", email).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = models.DB.Where("LOWER(email) = ?", strings.ToLower(email)).Order("id").First(&user).Error
	}
	if err != nil {
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
	request.Name = strings.TrimSpace(request.Name)
	request.Email = strings.ToLower(strings.TrimSpace(request.Email))
	if request.Name == "" || request.Email == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Nome e e-mail são obrigatórios"})
	}
	if len(request.Password) < 6 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "A senha precisa de pelo menos 6 caracteres"})
	}
	// A mesma função atende o cadastro público (/delivery-man/register) e o
	// painel admin (/delivery-man). Status, zona e limite de pedidos só o
	// admin define: pelo cadastro público o entregador entrava já
	// "available" na fila de despacho.
	if role, rErr := middlewares.GetUserRoleFromToken(c); rErr != nil || role != "admin" {
		request.Status, request.ZoneID, request.MaxOrders = "", nil, 0
	}
	if request.Status != "" && !validCourierStatus(request.Status) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Status inválido: available, busy ou offline"})
	}

	var dup int64
	models.DB.Model(&models.DeliveryMan{}).Where("LOWER(email) = ?", request.Email).Count(&dup)
	if dup > 0 {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "Este e-mail já está cadastrado"})
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
	if !validID(deliveryManID) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "ID inválido"})
	}
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
	if err := models.DB.First(&deliveryMan, "id = ?", deliveryManID).Error; err != nil {
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
		if !validCourierStatus(request.Status) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Status inválido: available, busy ou offline"})
		}
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

// validCourierStatus: os valores que o motor de despacho entende
// (models.DeliveryMan.Status). O painel mandava "online"/"on_delivery", que
// o despacho não reconhece — o entregador sumia da fila.
func validCourierStatus(s string) bool {
	switch s {
	case "available", "busy", "offline":
		return true
	}
	return false
}

// DeleteDeliveryMan remove um entregador (DELETE /delivery-man/:id). Admin.
func DeleteDeliveryMan(c *fiber.Ctx) error {
	deliveryManID := c.Params("id")
	if !validID(deliveryManID) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "ID inválido"})
	}
	if deliveryManID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid delivery man ID"})
	}

	var deliveryMan models.DeliveryMan
	if err := models.DB.First(&deliveryMan, "id = ?", deliveryManID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Delivery man not found"})
	}

	if err := models.DB.Delete(&deliveryMan).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to delete delivery man"})
	}

	return c.JSON(fiber.Map{"message": "Delivery man deleted successfully", "id": deliveryMan.ID})
}

func UpdateDeliveryManWallet(c *fiber.Ctx) error {
	deliveryManID := c.Params("id")
	if !validID(deliveryManID) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "ID inválido"})
	}
	if deliveryManID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid delivery man ID"})
	}

	role, roleErr := middlewares.GetUserRoleFromToken(c)
	if roleErr != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid token"})
	}

	_, tokenErr := middlewares.GetUserIDFromToken(c)
	if role != "admin" {
		if tokenErr != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid token"})
		}
		dmID, _ := strconv.ParseInt(deliveryManID, 10, 64)
		// Tipo E id: o cliente/usuário 5 não pode apontar a carteira de
		// recebimento do entregador 5 (tabelas com ids independentes).
		if !middlewares.IsOwnAccount(c, middlewares.AccountDeliveryMan, dmID) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Forbidden"})
		}
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
	if err := models.DB.First(&deliveryMan, "id = ?", deliveryManID).Error; err != nil {
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
