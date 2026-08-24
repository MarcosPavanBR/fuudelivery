package handlers

import (
	"crypto/rand"
	"log"
	"math"
	"math/big"
	"strconv"
	"time"

	"github.com/carloshomar/fuudelivery/auth_api/app/middlewares"
	"github.com/carloshomar/fuudelivery/orders_api/app/models"
	"github.com/gofiber/fiber/v2"
)

// lookupServerOrderTotal lê o total recalculado no servidor no momento da
// criação do pedido (payload JSONB em order_documents). Fonte única para
// pontos de fidelidade — o valor vindo do cliente nunca é usado.
func lookupServerOrderTotal(orderID string) (float64, bool) {
	if models.DB == nil || orderID == "" {
		return 0, false
	}
	var row struct {
		Total *float64
	}
	err := models.DB.Raw(
		`SELECT NULLIF(payload->>'order_total', '')::float8 AS total
		 FROM order_documents WHERE legacy_id = ? LIMIT 1`, orderID).Scan(&row).Error
	if err != nil {
		log.Printf("[LOYALTY] lookupServerOrderTotal(%s): %v", orderID, err)
		return 0, false
	}
	if row.Total == nil || *row.Total <= 0 {
		return 0, false
	}
	return *row.Total, true
}

// hasLoyaltyEarnForOrder evita crédito duplicado de pontos para o mesmo
// pedido (webhook + chamada manual concorrentes).
func hasLoyaltyEarnForOrder(orderID string) bool {
	var count int64
	models.DB.Model(&models.LoyaltyTransaction{}).
		Where("order_id = ? AND type = ?", orderID, "earn").
		Count(&count)
	return count > 0
}

func getTier(points int) string {
	switch {
	case points >= 1500:
		return "ouro"
	case points >= 500:
		return "prata"
	default:
		return "bronze"
	}
}

func getPointsMultiplier(tier string) int {
	switch tier {
	case "ouro":
		return 2
	default:
		return 1
	}
}

func generateCashbackCode() (string, error) {
	const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 6)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		if err != nil {
			return "", err
		}
		b[i] = chars[n.Int64()]
	}
	return "CASHBACK-" + string(b), nil
}

func EarnPoints(c *fiber.Ctx) error {
	// Identidade e valor vêm do servidor: o phone do token JWT identifica o
	// beneficiário e o order_total é o recalculado no banco na criação do
	// pedido. O body não pode mintar pontos para outro telefone nem com
	// valor inventado.
	tokenPhone, err := middlewares.GetUserPhoneFromToken(c)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{"error": "Invalid token"})
	}

	var req struct {
		UserPhone string `json:"user_phone"`
		OrderID   string `json:"order_id"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}

	if req.UserPhone == "" {
		req.UserPhone = tokenPhone
	}
	if req.UserPhone != tokenPhone {
		return c.Status(403).JSON(fiber.Map{"error": "Cannot earn points for another user"})
	}

	orderValue, ok := lookupServerOrderTotal(req.OrderID)
	if !ok {
		return c.Status(400).JSON(fiber.Map{"error": "Pedido inválido ou sem total calculado"})
	}
	if alreadyEarned := hasLoyaltyEarnForOrder(req.OrderID); alreadyEarned {
		return c.Status(409).JSON(fiber.Map{"error": "Pontos já creditados para este pedido"})
	}

	var loyalty models.LoyaltyPoints
	result := models.DB.Where("user_phone = ?", req.UserPhone).First(&loyalty)

	if result.Error != nil {
		loyalty = models.LoyaltyPoints{
			UserPhone: req.UserPhone,
			Points:    0,
			Tier:      "bronze",
		}
		models.DB.Create(&loyalty)
	}

	multiplier := getPointsMultiplier(loyalty.Tier)
	pointsEarned := int(math.Floor(orderValue)) * multiplier

	loyalty.Points += pointsEarned
	loyalty.TotalOrders++
	loyalty.TotalSpent += orderValue
	loyalty.Tier = getTier(loyalty.Points)
	loyalty.UpdatedAt = time.Now()

	models.DB.Save(&loyalty)

	transaction := models.LoyaltyTransaction{
		UserPhone:   req.UserPhone,
		Points:      pointsEarned,
		Type:        "earn",
		Description: "Pontos ganhos com pedido",
		OrderID:     req.OrderID,
		CreatedAt:   time.Now(),
	}
	models.DB.Create(&transaction)

	return c.JSON(fiber.Map{
		"message": "Pontos ganhos com sucesso",
		"points":  pointsEarned,
		"total":   loyalty.Points,
		"tier":    loyalty.Tier,
	})
}

func EarnPointsForOrder(userPhone, orderID string, orderValue float64) error {
	if userPhone == "" || orderValue <= 0 {
		return nil
	}

	var loyalty models.LoyaltyPoints
	result := models.DB.Where("user_phone = ?", userPhone).First(&loyalty)

	if result.Error != nil {
		loyalty = models.LoyaltyPoints{
			UserPhone: userPhone,
			Points:    0,
			Tier:      "bronze",
		}
		models.DB.Create(&loyalty)
	}

	multiplier := getPointsMultiplier(loyalty.Tier)
	pointsEarned := int(math.Floor(orderValue)) * multiplier

	loyalty.Points += pointsEarned
	loyalty.TotalOrders++
	loyalty.TotalSpent += orderValue
	loyalty.Tier = getTier(loyalty.Points)
	loyalty.UpdatedAt = time.Now()

	models.DB.Save(&loyalty)

	transaction := models.LoyaltyTransaction{
		UserPhone:   userPhone,
		Points:      pointsEarned,
		Type:        "earn",
		Description: "Pontos ganhos com pedido",
		OrderID:     orderID,
		CreatedAt:   time.Now(),
	}
	models.DB.Create(&transaction)

	log.Printf("[LOYALTY] %s ganhou %d pontos (pedido %s, valor %.2f)", userPhone, pointsEarned, orderID, orderValue)
	return nil
}

func RedeemPoints(c *fiber.Ctx) error {
	var req struct {
		UserPhone string `json:"user_phone"`
		Points    int    `json:"points"`
		OrderID   string `json:"order_id"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}

	var loyalty models.LoyaltyPoints
	result := models.DB.Where("user_phone = ?", req.UserPhone).First(&loyalty)
	if result.Error != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Usuário não encontrado"})
	}

	if loyalty.Points < req.Points {
		return c.Status(400).JSON(fiber.Map{"error": "Pontos insuficientes"})
	}

	if req.Points%10 != 0 {
		return c.Status(400).JSON(fiber.Map{"error": "Os pontos devem ser múltiplos de 10"})
	}

	discountValue := float64(req.Points / 10)

	loyalty.Points -= req.Points
	loyalty.Tier = getTier(loyalty.Points)
	loyalty.UpdatedAt = time.Now()
	models.DB.Save(&loyalty)

	transaction := models.LoyaltyTransaction{
		UserPhone:   req.UserPhone,
		Points:      -req.Points,
		Type:        "redeem",
		Description: "Pontos resgatados para desconto",
		OrderID:     req.OrderID,
		CreatedAt:   time.Now(),
	}
	models.DB.Create(&transaction)

	cashbackCode, err := generateCashbackCode()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Erro ao gerar cupom de cashback"})
	}

	now := time.Now()
	cashbackCoupon := models.Coupon{
		Code:           cashbackCode,
		Description:    "Cupom de cashback - resgate de pontos",
		DiscountType:   "FIXED",
		DiscountValue:  discountValue,
		MinOrderValue:  0,
		MaxUses:        1,
		MaxUsesPerUser: 1,
		StartDate:      now,
		ExpiryDate:     now.AddDate(0, 0, 30),
		IsActive:       true,
		OwnerPhone:     req.UserPhone,
	}

	if err := models.DB.Create(&cashbackCoupon).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Erro ao criar cupom de cashback"})
	}

	return c.JSON(fiber.Map{
		"message":          "Pontos resgatados com sucesso! Use o cupom no próximo pedido.",
		"points_redeemed":  req.Points,
		"discount_value":   discountValue,
		"remaining_points": loyalty.Points,
		"coupon_code":      cashbackCode,
		"coupon_expires":   cashbackCoupon.ExpiryDate.Format("02/01/2006"),
	})
}

func GetLoyaltyBalance(c *fiber.Ctx) error {
	phone := c.Params("phone")
	if phone == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Phone is required"})
	}

	var loyalty models.LoyaltyPoints
	result := models.DB.Where("user_phone = ?", phone).First(&loyalty)
	if result.Error != nil {
		return c.JSON(fiber.Map{
			"phone":        phone,
			"points":       0,
			"tier":         "bronze",
			"total_orders": 0,
			"total_spent":  0,
		})
	}

	return c.JSON(fiber.Map{
		"phone":        loyalty.UserPhone,
		"points":       loyalty.Points,
		"tier":         loyalty.Tier,
		"total_orders": loyalty.TotalOrders,
		"total_spent":  loyalty.TotalSpent,
	})
}

func GetLoyaltyHistory(c *fiber.Ctx) error {
	phone := c.Params("phone")
	if phone == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Phone is required"})
	}

	var transactions []models.LoyaltyTransaction
	models.DB.Where("user_phone = ?", phone).Order("created_at desc").Find(&transactions)

	if transactions == nil {
		transactions = []models.LoyaltyTransaction{}
	}

	return c.JSON(transactions)
}

func CalculateLoyaltyDiscount(c *fiber.Ctx) error {
	pointsStr := c.Query("points", "0")
	points, err := strconv.Atoi(pointsStr)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid points"})
	}

	maxDiscount := points / 10
	usedPoints := maxDiscount * 10

	return c.JSON(fiber.Map{
		"points_required": usedPoints,
		"discount_value":  float64(maxDiscount),
	})
}
