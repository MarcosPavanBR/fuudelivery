package handlers

import (
    "context"
    "strconv"
    "time"

    "github.com/carloshomar/vercardapio/app/dto"
    "github.com/carloshomar/vercardapio/app/models"
    "github.com/gofiber/fiber/v2"
    "go.mongodb.org/mongo-driver/bson"
    "go.mongodb.org/mongo-driver/bson/primitive"
)

func CreateReview(c *fiber.Ctx) error {
    var req dto.CreateReviewRequest
    if err := c.BodyParser(&req); err != nil {
        return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
    }

    if req.Rating < 1 || req.Rating > 5 {
        return c.Status(400).JSON(fiber.Map{"error": "Rating must be between 1 and 5"})
    }

    orderID, err := primitive.ObjectIDFromHex(req.OrderID)
    if err != nil {
        return c.Status(400).JSON(fiber.Map{"error": "Invalid order ID"})
    }

    collection := models.MongoDabase.Collection("orders")
    filter := bson.M{"_id": orderID}

    var order bson.M
    if err := collection.FindOne(context.Background(), filter).Decode(&order); err != nil {
        return c.Status(404).JSON(fiber.Map{"error": "Order not found"})
    }

    status, ok := order["status"].(string)
    if !ok || status != "FINISHED" {
        return c.Status(400).JSON(fiber.Map{"error": "Order is not finished yet"})
    }

    var existing models.Review
    result := models.DB.Where("order_id = ?", req.OrderID).First(&existing)
    if result.Error == nil {
        return c.Status(400).JSON(fiber.Map{"error": "You have already reviewed this order"})
    }

    establishmentID := uint(0)
    if estID, ok := order["establishmentid"]; ok {
        switch v := estID.(type) {
        case int64:
            establishmentID = uint(v)
        case float64:
            establishmentID = uint(v)
        }
    }

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
            models.DB.Create(&loyalty)
        }

        loyalty.Points += 5
        loyalty.UpdatedAt = time.Now()
        models.DB.Save(&loyalty)

        transaction := models.LoyaltyTransaction{
            UserPhone:   req.UserPhone,
            Points:      5,
            Type:        "earn",
            Description: "Pontos ganhos por avaliação",
            OrderID:     req.OrderID,
            CreatedAt:   time.Now(),
        }
        models.DB.Create(&transaction)
    }

    return c.JSON(fiber.Map{
        "message":    "Review created successfully",
        "review_id":  review.ID,
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
        "reviews":       responses,
        "total":         total,
        "page":          page,
        "limit":         limit,
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
