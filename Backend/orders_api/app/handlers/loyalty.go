package handlers

import (
	"math"
	"strconv"
	"time"

	"github.com/carloshomar/vercardapio/orders_api/app/models"
	"github.com/gofiber/fiber/v2"
)

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

func EarnPoints(c *fiber.Ctx) error {
	var req struct {
		UserPhone string  `json:"user_phone"`
		OrderID   string  `json:"order_id"`
		OrderValue float64 `json:"order_value"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
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
	pointsEarned := int(math.Floor(req.OrderValue)) * multiplier

	loyalty.Points += pointsEarned
	loyalty.TotalOrders++
	loyalty.TotalSpent += req.OrderValue
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
		"message":  "Pontos ganhos com sucesso",
		"points":   pointsEarned,
		"total":    loyalty.Points,
		"tier":     loyalty.Tier,
	})
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

	return c.JSON(fiber.Map{
		"message":       "Pontos resgatados com sucesso",
		"points_redeemed": req.Points,
		"discount_value": discountValue,
		"remaining_points": loyalty.Points,
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
			"phone":  phone,
			"points": 0,
			"tier":   "bronze",
			"total_orders": 0,
			"total_spent": 0,
		})
	}

	return c.JSON(fiber.Map{
		"phone":         loyalty.UserPhone,
		"points":        loyalty.Points,
		"tier":          loyalty.Tier,
		"total_orders":  loyalty.TotalOrders,
		"total_spent":   loyalty.TotalSpent,
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
