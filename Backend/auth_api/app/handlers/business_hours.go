package handlers

import (
	"fmt"

	"github.com/carloshomar/fuudelivery/auth_api/app/middlewares"
	"github.com/carloshomar/fuudelivery/auth_api/app/models"
	"github.com/gofiber/fiber/v2"
)

type BusinessHoursRequest struct {
	EstablishmentID uint   `json:"establishment_id"`
	DayOfWeek       int    `json:"day_of_week"`
	IsOpen          bool   `json:"is_open"`
	OpenTime        string `json:"open_time"`
	CloseTime       string `json:"close_time"`
	BreakStartTime  string `json:"break_start_time,omitempty"`
	BreakEndTime    string `json:"break_end_time,omitempty"`
}

func verifyOwnership(c *fiber.Ctx, establishmentID uint) error {
	tokenUserID, err := middlewares.GetUserIDFromToken(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid token"})
	}
	role, _ := middlewares.GetUserRoleFromToken(c)
	if role == "admin" {
		return nil // admin can edit any
	}
	var authUser models.User
	if dbErr := models.DB.First(&authUser, tokenUserID).Error; dbErr != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Cannot edit another establishment's data"})
	}
	if authUser.EstablishmentID != establishmentID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Cannot edit another establishment's data"})
	}
	return nil
}

func UpsertBusinessHours(c *fiber.Ctx) error {
	var req BusinessHoursRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}

	// IDOR protection: verify ownership of the establishment.
	if err := verifyOwnership(c, req.EstablishmentID); err != nil {
		return err
	}

	hours := models.BusinessHours{
		EstablishmentID: req.EstablishmentID,
		DayOfWeek:       req.DayOfWeek,
		IsOpen:          req.IsOpen,
		OpenTime:        req.OpenTime,
		CloseTime:       req.CloseTime,
		BreakStartTime:  req.BreakStartTime,
		BreakEndTime:    req.BreakEndTime,
	}

	var existing models.BusinessHours
	result := models.DB.Where("establishment_id = ? AND day_of_week = ?", req.EstablishmentID, req.DayOfWeek).First(&existing)

	if result.RowsAffected > 0 {
		models.DB.Model(&existing).Updates(hours)
		return c.JSON(existing)
	}

	models.DB.Create(&hours)
	return c.Status(201).JSON(hours)
}

func GetBusinessHours(c *fiber.Ctx) error {
	establishmentID := c.Params("id")
	var hours []models.BusinessHours
	models.DB.Where("establishment_id = ?", establishmentID).Order("day_of_week").Find(&hours)
	return c.JSON(hours)
}

func CheckEstablishmentOpen(c *fiber.Ctx) error {
	establishmentID := c.Params("id")

	var id uint
	if _, err := fmt.Sscanf(establishmentID, "%d", &id); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid ID"})
	}

	isOpen, err := models.IsEstablishmentOpen(id)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to check hours"})
	}

	return c.JSON(fiber.Map{
		"is_open":          isOpen,
		"establishment_id": id,
	})
}

func BulkUpdateBusinessHours(c *fiber.Ctx) error {
	var reqs []BusinessHoursRequest
	if err := c.BodyParser(&reqs); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}

	// IDOR protection: verify ownership on the first entry.
	// All entries must target the same establishment.
	if len(reqs) > 0 {
		if err := verifyOwnership(c, reqs[0].EstablishmentID); err != nil {
			return err
		}
	}

	for _, req := range reqs {
		hours := models.BusinessHours{
			EstablishmentID: req.EstablishmentID,
			DayOfWeek:       req.DayOfWeek,
			IsOpen:          req.IsOpen,
			OpenTime:        req.OpenTime,
			CloseTime:       req.CloseTime,
			BreakStartTime:  req.BreakStartTime,
			BreakEndTime:    req.BreakEndTime,
		}

		var existing models.BusinessHours
		result := models.DB.Where("establishment_id = ? AND day_of_week = ?", req.EstablishmentID, req.DayOfWeek).First(&existing)

		if result.RowsAffected > 0 {
			models.DB.Model(&existing).Updates(hours)
		} else {
			models.DB.Create(&hours)
		}
	}

	return c.JSON(fiber.Map{"message": "Horários atualizados"})
}
