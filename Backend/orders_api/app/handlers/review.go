package handlers

import (
	"strconv"
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

	var existing models.Review
	result := models.DB.Where("order_id = ?", req.OrderID).First(&existing)
	if result.Error == nil {
		return c.Status(400).JSON(fiber.Map{"error": "You have already reviewed this order"})
	}

	// Verifica se o revisor é o dono do pedido ou o estabelecimento
	tokenPhone, phoneErr := middlewares.GetUserPhoneFromToken(c)
	isOwner := phoneErr == nil && tokenPhone == req.UserPhone
	isAdmin := false
	if !isOwner {
		role, roleErr := middlewares.GetUserRoleFromToken(c)
		if roleErr == nil && role == "admin" {
			isAdmin = true
		}
	}
	if !isOwner && !isAdmin {
		return c.Status(403).JSON(fiber.Map{"error": "Only the order owner or admin can review"})
	}

	establishmentID := uint(doc.EstablishmentID)

	review := models.Review{
		OrderID:         req.OrderID,
		EstablishmentID: establishmentID,
		UserPhone:       req.UserPhone,
		UserName:        req.UserName,
		ProductID:       req.ProductID,
		Rating:          req.Rating,
		Comment:         req.Comment,
		ImageURL:        req.ImageURL,
		IsAnonymous:     req.IsAnonymous,
	}

	if err := models.DB.Create(&review).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to save review"})
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
				log.Printf("[REVIEW] Erro ao criar loyalty para %s: %v", req.UserPhone, err)
			}
		}

		loyalty.Points += 5
		loyalty.UpdatedAt = time.Now()
		if err := models.DB.Save(&loyalty).Error; err != nil {
			log.Printf("[REVIEW] Erro ao atualizar loyalty para %s: %v", req.UserPhone, err)
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
			log.Printf("[REVIEW] Erro ao registrar transação para %s: %v", req.UserPhone, err)
		}
	}

	return c.JSON(fiber.Map{
		"message":        "Review created successfully",
		"review_id":      review.ID,
		"points_awarded": 5,
	})
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

	var responses []dto.ReviewResponse
	for _, r := range reviews {
		userName := r.UserName
		if r.IsAnonymous {
			userName = ""
		}
		responses = append(responses, dto.ReviewResponse{
			Rating:    r.Rating,
			Comment:   r.Comment,
			UserName:  userName,
			ImageURL:  r.ImageURL,
			CreatedAt: r.CreatedAt.Format(time.RFC3339),
		})
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

	var responses []dto.ReviewResponse
	for _, r := range reviews {
		userName := r.UserName
		if r.IsAnonymous {
			userName = ""
		}
		responses = append(responses, dto.ReviewResponse{
			Rating:    r.Rating,
			Comment:   r.Comment,
			UserName:  userName,
			ImageURL:  r.ImageURL,
			CreatedAt: r.CreatedAt.Format(time.RFC3339),
		})
	}

	return c.JSON(fiber.Map{
		"reviews": responses,
		"total":   len(responses),
	})
}

func RespondToReview(c *fiber.Ctx) error {
	reviewID := c.Params("id")

	var req dto.RespondReviewRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}

	if req.ResponseText == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Response text is required"})
	}

	var review models.Review
	if err := models.DB.First(&review, reviewID).Error; err != nil {
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

	var reviews []models.Review
	models.DB.Where("user_phone = ?", phone).
		Order("created_at desc").
		Find(&reviews)

	var responses []dto.ReviewResponse
	for _, r := range reviews {
		userName := r.UserName
		if r.IsAnonymous {
			userName = ""
		}
		responses = append(responses, dto.ReviewResponse{
			Rating:    r.Rating,
			Comment:   r.Comment,
			UserName:  userName,
			ImageURL:  r.ImageURL,
			CreatedAt: r.CreatedAt.Format(time.RFC3339),
		})
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
