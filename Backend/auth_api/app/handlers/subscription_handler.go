package handlers

import (
	"strconv"
	"time"

	"github.com/carloshomar/vercardapio/auth_api/app/middlewares"
	"github.com/carloshomar/vercardapio/auth_api/app/models"
	"github.com/gofiber/fiber/v2"
)

// ListSubscriptions lista todas as assinaturas (admin).
// GET /api/subscriptions
func ListSubscriptions(c *fiber.Ctx) error {
	var subs []models.Subscription
	if err := models.DB.Order("created_at desc").Find(&subs).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to list subscriptions"})
	}
	return c.JSON(subs)
}

// GetUserSubscription retorna a assinatura do usuario logado.
// GET /api/subscriptions/me
func GetUserSubscription(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid token"})
	}

	var sub models.Subscription
	if err := models.DB.Where("user_id = ?", userID).First(&sub).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "No active subscription found"})
	}
	return c.JSON(sub)
}

// CreateSubscription cria uma nova assinatura para o usuario logado.
// POST /api/subscriptions
func CreateSubscription(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid token"})
	}

	var req struct {
		Plan string `json:"plan"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	// Valida plano
	if req.Plan != models.PlanBasic && req.Plan != models.PlanPremium {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid plan. Use 'basic' or 'premium'",
		})
	}

	// Verifica se ja existe assinatura ativa
	var existing models.Subscription
	if err := models.DB.Where("user_id = ? AND status = ?", userID, models.SubscriptionActive).First(&existing).Error; err == nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "User already has an active subscription"})
	}

	amount := models.GetPlanAmount(req.Plan)
	freeDeliveryAbove, cashbackPct := models.GetPlanBenefits(req.Plan)
	now := time.Now()

	sub := models.Subscription{
		UserID:             uint(userID),
		Plan:               req.Plan,
		Status:             models.SubscriptionActive,
		Amount:             amount,
		FreeDeliveryAbove:  freeDeliveryAbove,
		CashbackPct:        cashbackPct,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 1, 0), // +1 mes
	}

	if err := models.DB.Create(&sub).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create subscription"})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message":      "Subscription created successfully",
		"subscription": sub,
	})
}

// CancelSubscription cancela a assinatura do usuario logado.
// POST /api/subscriptions/cancel
func CancelSubscription(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid token"})
	}

	var sub models.Subscription
	if err := models.DB.Where("user_id = ? AND status = ?", userID, models.SubscriptionActive).First(&sub).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "No active subscription found"})
	}

	now := time.Now()
	sub.Status = models.SubscriptionCancelled
	sub.CancelledAt = &now

	if err := models.DB.Save(&sub).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to cancel subscription"})
	}

	return c.JSON(fiber.Map{
		"message":      "Subscription cancelled. You have access until the end of the billing period.",
		"subscription": sub,
	})
}

// RenewSubscription renova a assinatura por mais um mes.
// POST /api/subscriptions/renew
func RenewSubscription(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid token"})
	}

	var sub models.Subscription
	if err := models.DB.Where("user_id = ?", userID).Order("created_at desc").First(&sub).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "No subscription found"})
	}

	if sub.Status == models.SubscriptionExpired {
		// Reativa se expirou: novo periodo
		sub.Status = models.SubscriptionActive
		now := time.Now()
		sub.CurrentPeriodStart = now
		sub.CurrentPeriodEnd = now.AddDate(0, 1, 0)
	} else if sub.Status == models.SubscriptionCancelled {
		// Reativa se cancelada
		sub.Status = models.SubscriptionActive
		sub.CancelledAt = nil
		now := time.Now()
		sub.CurrentPeriodStart = now
		sub.CurrentPeriodEnd = now.AddDate(0, 1, 0)
	} else if sub.Status == models.SubscriptionActive {
		// Estende o periodo atual por +1 mes
		sub.CurrentPeriodEnd = sub.CurrentPeriodEnd.AddDate(0, 1, 0)
	}

	if err := models.DB.Save(&sub).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to renew subscription"})
	}

	return c.JSON(fiber.Map{
		"message":      "Subscription renewed successfully",
		"subscription": sub,
	})
}

// AdminUpdateSubscription atualiza qualquer assinatura (admin).
// PUT /api/subscriptions/:id
func AdminUpdateSubscription(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid subscription ID"})
	}

	var sub models.Subscription
	if err := models.DB.First(&sub, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Subscription not found"})
	}

	var req struct {
		Plan               *string  `json:"plan"`
		Status             *string  `json:"status"`
		Amount             *float64 `json:"amount"`
		FreeDeliveryAbove  *float64 `json:"free_delivery_above"`
		CashbackPct        *float64 `json:"cashback_pct"`
		CurrentPeriodStart *string  `json:"current_period_start"`
		CurrentPeriodEnd   *string  `json:"current_period_end"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if req.Plan != nil {
		sub.Plan = *req.Plan
		// Recalcula beneficios ao mudar de plano
		amount := models.GetPlanAmount(sub.Plan)
		sub.Amount = amount
		freeAbove, cashback := models.GetPlanBenefits(sub.Plan)
		sub.FreeDeliveryAbove = freeAbove
		sub.CashbackPct = cashback
	}
	if req.Status != nil {
		sub.Status = *req.Status
	}
	if req.Amount != nil {
		sub.Amount = *req.Amount
	}
	if req.FreeDeliveryAbove != nil {
		sub.FreeDeliveryAbove = *req.FreeDeliveryAbove
	}
	if req.CashbackPct != nil {
		sub.CashbackPct = *req.CashbackPct
	}

	if err := models.DB.Save(&sub).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update subscription"})
	}

	return c.JSON(fiber.Map{
		"message":      "Subscription updated successfully",
		"subscription": sub,
	})
}

// getUserID extrai o ID do usuario do token JWT.
func getUserID(c *fiber.Ctx) (int64, error) {
	return middlewares.GetUserIDFromToken(c)
}
