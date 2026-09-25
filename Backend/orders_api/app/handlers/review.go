package handlers

import (
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/carloshomar/fuudelivery/auth_api/app/middlewares"
	"github.com/carloshomar/fuudelivery/orders_api/app/dto"
	"github.com/carloshomar/fuudelivery/orders_api/app/models"
	"github.com/gofiber/fiber/v2"
)

// createReview lê o pedido via camada Postgres-first (orders_pg.go) e grava
// a avaliação em Postgres, creditando pontos de fidelidade.
func CreateReview(c *fiber.Ctx) error {
	var req dto.CreateReviewRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}

	if req.Rating < 1 || req.Rating > 5 {
		return c.Status(400).JSON(fiber.Map{"error": "Rating must be between 1 and 5"})
	}

	// Corte 5: existência/status/estabelecimento vêm do Postgres-first
	// (findOrderByLegacyId faz lazy import do Mongo para pedidos antigos).
	doc, err := findOrderByLegacyID(req.OrderID)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Order not found"})
	}

	if doc.Status != "FINISHED" {
		return c.Status(400).JSON(fiber.Map{"error": "Order is not finished yet"})
	}

	// Só quem fez o pedido avalia, e o telefone vem do TOKEN. Antes bastava
	// o telefone do corpo bater com o do token: qualquer cliente avaliava
	// pedido de qualquer outra pessoa (nota falsa na concorrência) e ainda
	// ganhava os pontos; o admin creditava pontos a um telefone qualquer.
	tokenPhone, phoneErr := middlewares.GetUserPhoneFromToken(c)
	if phoneErr != nil || tokenPhone == "" || !samePhone(tokenPhone, doc.UserPhone) {
		return c.Status(403).JSON(fiber.Map{"error": "Só quem fez o pedido pode avaliar"})
	}
	req.UserPhone = doc.UserPhone

	var existing models.Review
	if models.DB.Where("order_id = ?", req.OrderID).First(&existing).Error == nil {
		return c.Status(409).JSON(fiber.Map{"error": "Este pedido já foi avaliado"})
	}

	review := models.Review{
		OrderID:         req.OrderID,
		EstablishmentID: uint(doc.EstablishmentID),
		UserPhone:       req.UserPhone,
		UserName:        clipText(req.UserName, 60),
		ProductID:       req.ProductID,
		Rating:          req.Rating,
		Comment:         clipText(req.Comment, maxReviewText),
		// Foto: não há upload no app e um link qualquer vira pixel de
		// rastreio na tela da loja. Fica de fora até existir upload próprio.
		ImageURL:    "",
		IsAnonymous: req.IsAnonymous,
	}

	if err := models.DB.Create(&review).Error; err != nil {
		// Duas avaliações simultâneas do mesmo pedido: o índice único de
		// order_id barra a segunda.
		return c.Status(409).JSON(fiber.Map{"error": "Este pedido já foi avaliado"})
	}

	if req.UserPhone != "" {
		var loyalty models.LoyaltyPoints
		res := models.DB.Where("user_phone = ?", req.UserPhone).First(&loyalty)
		if res.Error != nil {
			loyalty = models.LoyaltyPoints{
				UserPhone: req.UserPhone,
				Points:    0,
				Tier:      "bronze",
			}
			if err := models.DB.Create(&loyalty).Error; err != nil {
				log.Printf("[REVIEW] Erro ao criar loyalty para %s: %v", maskPhone(req.UserPhone), err)
			}
		}

		loyalty.Points += 5
		loyalty.UpdatedAt = time.Now()
		if err := models.DB.Save(&loyalty).Error; err != nil {
			log.Printf("[REVIEW] Erro ao atualizar loyalty para %s: %v", maskPhone(req.UserPhone), err)
		}

		transaction := models.LoyaltyTransaction{
			UserPhone:   req.UserPhone,
			Points:      5,
			Type:        "earn",
			Description: "Pontos ganhos por avaliação",
			OrderID:     req.OrderID,
			CreatedAt:   time.Now(),
		}
		if err := models.DB.Create(&transaction).Error; err != nil {
			log.Printf("[REVIEW] Erro ao registrar transação para %s: %v", maskPhone(req.UserPhone), err)
		}
	}

	return c.JSON(fiber.Map{
		"message":        "Review created successfully",
		"review_id":      review.ID,
		"points_awarded": 5,
	})
}

// maxReviewText limita comentário e resposta da loja.
const maxReviewText = 500

// clipText apara e corta em n caracteres (runas, não bytes).
func clipText(s string, n int) string {
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > n {
		return strings.TrimSpace(string(r[:n]))
	}
	return s
}

// samePhone compara telefones ignorando formatação ("+55 11 9..." vs
// "5511 9...").
func samePhone(a, b string) bool {
	digits := func(s string) string {
		return strings.Map(func(r rune) rune {
			if r >= '0' && r <= '9' {
				return r
			}
			return -1
		}, s)
	}
	da, db := digits(a), digits(b)
	return da != "" && da == db
}

func toReviewResponse(r models.Review) dto.ReviewResponse {
	userName := r.UserName
	if r.IsAnonymous {
		userName = ""
	}
	out := dto.ReviewResponse{
		ID:           r.ID,
		Rating:       r.Rating,
		Comment:      r.Comment,
		UserName:     userName,
		ImageURL:     r.ImageURL,
		CreatedAt:    r.CreatedAt.Format(time.RFC3339),
		ResponseText: r.ResponseText,
	}
	if r.ResponseAt != nil {
		out.ResponseAt = r.ResponseAt.Format(time.RFC3339)
	}
	return out
}

func GetEstablishmentReviews(c *fiber.Ctx) error {
	establishmentID := c.Params("id")
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	offset := (page - 1) * limit

	var reviews []models.Review
	models.DB.Where("establishment_id = ?", establishmentID).
		Order("created_at desc").
		Offset(offset).
		Limit(limit).
		Find(&reviews)

	var total int64
	models.DB.Model(&models.Review{}).Where("establishment_id = ?", establishmentID).Count(&total)

	var avgRating struct {
		Average float64
	}
	models.DB.Model(&models.Review{}).
		Select("COALESCE(AVG(rating), 0) as average").
		Where("establishment_id = ?", establishmentID).
		Scan(&avgRating)

	responses := []dto.ReviewResponse{}
	for _, r := range reviews {
		responses = append(responses, toReviewResponse(r))
	}

	return c.JSON(fiber.Map{
		"reviews":        responses,
		"total":          total,
		"page":           page,
		"limit":          limit,
		"average_rating": avgRating.Average,
	})
}

func GetProductReviews(c *fiber.Ctx) error {
	productID := c.Params("id")

	var reviews []models.Review
	models.DB.Where("product_id = ?", productID).
		Order("created_at desc").
		Find(&reviews)

	responses := []dto.ReviewResponse{}
	for _, r := range reviews {
		responses = append(responses, toReviewResponse(r))
	}

	return c.JSON(fiber.Map{
		"reviews": responses,
		"total":   len(responses),
	})
}

func RespondToReview(c *fiber.Ctx) error {
	reviewID := c.Params("id")
	if !validID(reviewID) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "ID inválido"})
	}

	var req dto.RespondReviewRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}

	req.ResponseText = clipText(req.ResponseText, maxReviewText)
	if req.ResponseText == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Response text is required"})
	}

	var review models.Review
	if err := models.DB.First(&review, "id = ?", reviewID).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Review not found"})
	}

	if !canActOnEstablishment(c, int64(review.EstablishmentID)) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Forbidden"})
	}

	now := time.Now()
	review.ResponseText = req.ResponseText
	review.ResponseAt = &now

	if err := models.DB.Save(&review).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to save response"})
	}

	return c.JSON(fiber.Map{
		"message": "Response saved successfully",
	})
}

func GetUserReviews(c *fiber.Ctx) error {
	phone := c.Params("phone")

	tokenPhone, phoneErr := middlewares.GetUserPhoneFromToken(c)
	role, roleErr := middlewares.GetUserRoleFromToken(c)
	if phoneErr != nil || roleErr != nil || (role != "admin" && tokenPhone != phone) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Forbidden"})
	}

	var reviews []models.Review
	models.DB.Where("user_phone = ?", phone).
		Order("created_at desc").
		Find(&reviews)

	responses := []dto.ReviewResponse{}
	for _, r := range reviews {
		item := toReviewResponse(r)
		item.OrderID = r.OrderID
		responses = append(responses, item)
	}

	return c.JSON(fiber.Map{
		"reviews": responses,
		"total":   len(responses),
	})
}

func GetEstablishmentRating(c *fiber.Ctx) error {
	establishmentID := c.Params("establishmentId")

	id, err := strconv.ParseUint(establishmentID, 10, 64)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid establishment ID"})
	}

	var totalReviews int64
	models.DB.Model(&models.Review{}).
		Where("establishment_id = ?", id).
		Count(&totalReviews)

	var avgRating struct {
		Average float64
	}
	models.DB.Model(&models.Review{}).
		Select("COALESCE(AVG(rating), 0) as average").
		Where("establishment_id = ?", id).
		Scan(&avgRating)

	ratingCounts := make(map[int]int)
	for i := 1; i <= 5; i++ {
		var count int64
		models.DB.Model(&models.Review{}).
			Where("establishment_id = ? AND rating = ?", id, i).
			Count(&count)
		ratingCounts[i] = int(count)
	}

	return c.JSON(dto.EstablishmentRating{
		EstablishmentID: uint(id),
		AverageRating:   avgRating.Average,
		TotalReviews:    int(totalReviews),
		RatingCounts:    ratingCounts,
	})
}
