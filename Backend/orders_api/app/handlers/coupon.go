package handlers

import (
	"strings"
	"time"

	"github.com/carloshomar/vercardapio/orders_api/app/dto"
	"github.com/carloshomar/vercardapio/orders_api/app/models"
	"github.com/gofiber/fiber/v2"
)

func CreateCoupon(c *fiber.Ctx) error {
	var request dto.CreateCouponRequest
	if err := c.BodyParser(&request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Erro ao fazer parsing do corpo da requisição"})
	}

	request.Code = strings.ToUpper(strings.TrimSpace(request.Code))
	if request.Code == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Código do cupom é obrigatório"})
	}

	if request.DiscountType != "PERCENTAGE" && request.DiscountType != "FIXED" && request.DiscountType != "FREE_DELIVERY" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Tipo de desconto inválido. Use PERCENTAGE, FIXED ou FREE_DELIVERY"})
	}

	if request.DiscountType == "PERCENTAGE" && (request.DiscountValue <= 0 || request.DiscountValue > 100) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Percentual de desconto deve estar entre 1 e 100"})
	}

	if request.DiscountType == "FIXED" && request.DiscountValue <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Valor de desconto deve ser maior que zero"})
	}

	var existingCoupon models.Coupon
	if err := models.DB.Where("code = ?", request.Code).First(&existingCoupon).Error; err == nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "Código de cupom já existe"})
	}

	startDate, err := time.Parse(time.RFC3339, request.StartDate)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Data de início inválida. Use o formato RFC3339"})
	}

	expiryDate, err := time.Parse(time.RFC3339, request.ExpiryDate)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Data de expiração inválida. Use o formato RFC3339"})
	}

	coupon := models.Coupon{
		Code:            request.Code,
		Description:     request.Description,
		DiscountType:    request.DiscountType,
		DiscountValue:   request.DiscountValue,
		MinOrderValue:   request.MinOrderValue,
		MaxUses:         request.MaxUses,
		MaxUsesPerUser:  request.MaxUsesPerUser,
		StartDate:       startDate,
		ExpiryDate:      expiryDate,
		EstablishmentID: request.EstablishmentID,
	}

	if err := models.DB.Create(&coupon).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Erro ao criar cupom"})
	}

	return c.Status(fiber.StatusCreated).JSON(coupon)
}

func ValidateCoupon(c *fiber.Ctx) error {
	var request dto.ValidateCouponRequest
	if err := c.BodyParser(&request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Erro ao fazer parsing do corpo da requisição"})
	}

	request.Code = strings.ToUpper(strings.TrimSpace(request.Code))

	var coupon models.Coupon
	if err := models.DB.Where("code = ?", request.Code).First(&coupon).Error; err != nil {
		return c.JSON(dto.ValidateCouponResponse{
			Valid:   false,
			Message: "Cupom não encontrado",
		})
	}

	if !coupon.IsActive {
		return c.JSON(dto.ValidateCouponResponse{
			Valid:   false,
			Message: "Cupom está inativo",
		})
	}

	now := time.Now()
	if now.Before(coupon.StartDate) {
		return c.JSON(dto.ValidateCouponResponse{
			Valid:   false,
			Message: "Cupom ainda não está válido",
		})
	}

	if now.After(coupon.ExpiryDate) {
		return c.JSON(dto.ValidateCouponResponse{
			Valid:   false,
			Message: "Cupom expirado",
		})
	}

	if coupon.MaxUses > 0 && coupon.UsedCount >= coupon.MaxUses {
		return c.JSON(dto.ValidateCouponResponse{
			Valid:   false,
			Message: "Cupom atingiu o limite máximo de usos",
		})
	}

	if request.OrderValue < coupon.MinOrderValue {
		return c.JSON(dto.ValidateCouponResponse{
			Valid:   false,
			Message: "Valor mínimo do pedido não atingido",
		})
	}

	if request.EstablishmentID != 0 && coupon.EstablishmentID != 0 && coupon.EstablishmentID != request.EstablishmentID {
		return c.JSON(dto.ValidateCouponResponse{
			Valid:   false,
			Message: "Cupom não é válido para este estabelecimento",
		})
	}

	if coupon.MaxUsesPerUser > 0 && request.UserPhone != "" {
		var userUsageCount int64
		models.DB.Model(&models.CouponUsage{}).Where("coupon_id = ? AND user_phone = ?", coupon.ID, request.UserPhone).Count(&userUsageCount)
		if int(userUsageCount) >= coupon.MaxUsesPerUser {
			return c.JSON(dto.ValidateCouponResponse{
				Valid:   false,
				Message: "Você já atingiu o limite de usos deste cupom",
			})
		}
	}

	var discountAmount float64
	switch coupon.DiscountType {
	case "PERCENTAGE":
		discountAmount = request.OrderValue * (coupon.DiscountValue / 100)
	case "FIXED":
		discountAmount = coupon.DiscountValue
	case "FREE_DELIVERY":
		discountAmount = 0
	}

	finalValue := request.OrderValue - discountAmount
	if finalValue < 0 {
		finalValue = 0
	}

	return c.JSON(dto.ValidateCouponResponse{
		Valid:          true,
		DiscountType:   coupon.DiscountType,
		DiscountValue:  coupon.DiscountValue,
		DiscountAmount: discountAmount,
		FinalValue:     finalValue,
	})
}

func ApplyCoupon(c *fiber.Ctx) error {
	var request dto.ApplyCouponRequest
	if err := c.BodyParser(&request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Erro ao fazer parsing do corpo da requisição"})
	}

	request.Code = strings.ToUpper(strings.TrimSpace(request.Code))

	validateReq := dto.ValidateCouponRequest{
		Code:      request.Code,
		UserPhone: request.UserPhone,
	}
	validateResp := ValidateCouponInternal(validateReq)
	if !validateResp.Valid {
		return c.JSON(validateResp)
	}

	var coupon models.Coupon
	models.DB.Where("code = ?", request.Code).First(&coupon)

	usage := models.CouponUsage{
		CouponID:       coupon.ID,
		UserPhone:      request.UserPhone,
		OrderID:        request.OrderID,
		DiscountAmount: validateResp.DiscountAmount,
		UsedAt:         time.Now(),
	}

	if err := models.DB.Create(&usage).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Erro ao registrar uso do cupom"})
	}

	models.DB.Model(&coupon).UpdateColumn("used_count", coupon.UsedCount+1)

	return c.JSON(fiber.Map{
		"success":        true,
		"message":        "Cupom aplicado com sucesso",
		"discount_type":  validateResp.DiscountType,
		"discount_value": validateResp.DiscountValue,
		"discount_amount": validateResp.DiscountAmount,
		"final_value":    validateResp.FinalValue,
	})
}

func ValidateCouponInternal(req dto.ValidateCouponRequest) dto.ValidateCouponResponse {
	var coupon models.Coupon
	if err := models.DB.Where("code = ?", req.Code).First(&coupon).Error; err != nil {
		return dto.ValidateCouponResponse{Valid: false, Message: "Cupom não encontrado"}
	}

	if !coupon.IsActive {
		return dto.ValidateCouponResponse{Valid: false, Message: "Cupom está inativo"}
	}

	now := time.Now()
	if now.Before(coupon.StartDate) {
		return dto.ValidateCouponResponse{Valid: false, Message: "Cupom ainda não está válido"}
	}

	if now.After(coupon.ExpiryDate) {
		return dto.ValidateCouponResponse{Valid: false, Message: "Cupom expirado"}
	}

	if coupon.MaxUses > 0 && coupon.UsedCount >= coupon.MaxUses {
		return dto.ValidateCouponResponse{Valid: false, Message: "Cupom atingiu o limite máximo de usos"}
	}

	if req.OrderValue < coupon.MinOrderValue {
		return dto.ValidateCouponResponse{Valid: false, Message: "Valor mínimo do pedido não atingido"}
	}

	if req.EstablishmentID != 0 && coupon.EstablishmentID != 0 && coupon.EstablishmentID != req.EstablishmentID {
		return dto.ValidateCouponResponse{Valid: false, Message: "Cupom não é válido para este estabelecimento"}
	}

	if coupon.MaxUsesPerUser > 0 && req.UserPhone != "" {
		var userUsageCount int64
		models.DB.Model(&models.CouponUsage{}).Where("coupon_id = ? AND user_phone = ?", coupon.ID, req.UserPhone).Count(&userUsageCount)
		if int(userUsageCount) >= coupon.MaxUsesPerUser {
			return dto.ValidateCouponResponse{Valid: false, Message: "Você já atingiu o limite de usos deste cupom"}
		}
	}

	var discountAmount float64
	switch coupon.DiscountType {
	case "PERCENTAGE":
		discountAmount = req.OrderValue * (coupon.DiscountValue / 100)
	case "FIXED":
		discountAmount = coupon.DiscountValue
	case "FREE_DELIVERY":
		discountAmount = 0
	}

	finalValue := req.OrderValue - discountAmount
	if finalValue < 0 {
		finalValue = 0
	}

	return dto.ValidateCouponResponse{
		Valid:          true,
		DiscountType:   coupon.DiscountType,
		DiscountValue:  coupon.DiscountValue,
		DiscountAmount: discountAmount,
		FinalValue:     finalValue,
	}
}

func ListCoupons(c *fiber.Ctx) error {
	establishmentID := c.Query("establishment_id")

	var coupons []models.Coupon
	query := models.DB
	if establishmentID != "" {
		query = query.Where("establishment_id = ? OR establishment_id = 0", establishmentID)
	}
	query.Order("created_at DESC").Find(&coupons)

	return c.JSON(coupons)
}

func GetCoupon(c *fiber.Ctx) error {
	id := c.Params("id")

	var coupon models.Coupon
	if err := models.DB.First(&coupon, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Cupom não encontrado"})
	}

	return c.JSON(coupon)
}

func DeleteCoupon(c *fiber.Ctx) error {
	id := c.Params("id")

	var coupon models.Coupon
	if err := models.DB.First(&coupon, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Cupom não encontrado"})
	}

	coupon.IsActive = false
	if err := models.DB.Save(&coupon).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Erro ao desativar cupom"})
	}

	return c.JSON(fiber.Map{"message": "Cupom desativado com sucesso"})
}

func GenerateReferralCoupon(c *fiber.Ctx) error {
	var request dto.ReferralCouponRequest
	if err := c.BodyParser(&request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Erro ao fazer parsing do corpo da requisição"})
	}

	now := time.Now()
	referrerExpiry := now.AddDate(0, 3, 0)
	newUserExpiry := now.AddDate(0, 3, 0)

	referrerCode := "INDICOU-" + strings.ToUpper(request.ReferrerPhone)
	newUserCode := "GANHOU-" + strings.ToUpper(request.NewUserPhone)

	referrerCoupon := models.Coupon{
		Code:          referrerCode,
		Description:   "Cupom de indicação - Você indicou um amigo!",
		DiscountType:  "PERCENTAGE",
		DiscountValue: 10,
		MinOrderValue: 0,
		MaxUses:       1,
		MaxUsesPerUser: 1,
		StartDate:     now,
		ExpiryDate:    referrerExpiry,
		IsActive:      true,
	}

	if err := models.DB.Create(&referrerCoupon).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Erro ao criar cupom do indicador"})
	}

	newUserCoupon := models.Coupon{
		Code:          newUserCode,
		Description:   "Cupom de indicação - Seja bem-vindo!",
		DiscountType:  "FIXED",
		DiscountValue: 10,
		MinOrderValue: 0,
		MaxUses:       1,
		MaxUsesPerUser: 1,
		StartDate:     now,
		ExpiryDate:    newUserExpiry,
		IsActive:      true,
	}

	if err := models.DB.Create(&newUserCoupon).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Erro ao criar cupom do novo usuário"})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message":               "Cupons de indicação criados com sucesso",
		"referrer_coupon_code":  referrerCoupon.Code,
		"new_user_coupon_code":  newUserCoupon.Code,
		"referrer_coupon":       referrerCoupon,
		"new_user_coupon":       newUserCoupon,
	})
}

func CalculateDiscount(c *fiber.Ctx) error {
	var request dto.ValidateCouponRequest
	if err := c.BodyParser(&request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Erro ao fazer parsing do corpo da requisição"})
	}

	request.Code = strings.ToUpper(strings.TrimSpace(request.Code))

	var coupon models.Coupon
	if err := models.DB.Where("code = ?", request.Code).First(&coupon).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Cupom não encontrado"})
	}

	var discountAmount float64
	switch coupon.DiscountType {
	case "PERCENTAGE":
		discountAmount = request.OrderValue * (coupon.DiscountValue / 100)
	case "FIXED":
		discountAmount = coupon.DiscountValue
	case "FREE_DELIVERY":
		discountAmount = 0
	}

	finalValue := request.OrderValue - discountAmount
	if finalValue < 0 {
		finalValue = 0
	}

	return c.JSON(dto.ValidateCouponResponse{
		Valid:          true,
		DiscountType:   coupon.DiscountType,
		DiscountValue:  coupon.DiscountValue,
		DiscountAmount: discountAmount,
		FinalValue:     finalValue,
	})
}
