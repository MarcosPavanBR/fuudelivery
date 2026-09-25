package handlers

import (
	"fmt"

	"github.com/carloshomar/fuudelivery/auth_api/app/middlewares"
	"github.com/carloshomar/fuudelivery/auth_api/app/models"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
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

// authorizeBusinessHours barra IDOR: só o dono da loja (ou admin) mexe nos
// horários dela. Sem isto, qualquer usuário logado — até um cliente — fechava
// a loja de qualquer restaurante.
func authorizeBusinessHours(c *fiber.Ctx, establishmentID uint) bool {
	role, rErr := middlewares.GetUserRoleFromToken(c)
	if rErr != nil {
		return false
	}
	tokenEstID, eErr := middlewares.GetEstablishmentIDFromToken(c)
	return canManageEstablishment(role, tokenEstID, eErr, establishmentID)
}

func validBusinessHours(req BusinessHoursRequest) bool {
	return req.EstablishmentID > 0 && req.DayOfWeek >= 0 && req.DayOfWeek <= 6
}

// saveBusinessHours grava o dia com TODOS os campos. Updates/Create com struct
// pulam valores zero (e IsOpen tem default:true), então is_open=false e o
// intervalo vazio nunca chegavam ao banco — a loja não conseguia fechar um dia.
func saveBusinessHours(db *gorm.DB, req BusinessHoursRequest) (models.BusinessHours, error) {
	hours := models.BusinessHours{
		EstablishmentID: req.EstablishmentID,
		DayOfWeek:       req.DayOfWeek,
		IsOpen:          req.IsOpen,
		OpenTime:        req.OpenTime,
		CloseTime:       req.CloseTime,
		BreakStartTime:  req.BreakStartTime,
		BreakEndTime:    req.BreakEndTime,
	}
	cols := []string{"is_open", "open_time", "close_time", "break_start_time", "break_end_time"}
	var existing models.BusinessHours
	if err := db.Where("establishment_id = ? AND day_of_week = ?", req.EstablishmentID, req.DayOfWeek).
		Limit(1).Find(&existing).Error; err != nil {
		return hours, err
	}
	if existing.ID > 0 {
		hours.ID = existing.ID
		return hours, db.Model(&existing).Select(cols).Updates(&hours).Error
	}
	// Map, não struct: com struct o GORM troca is_open=false pelo default:true.
	err := db.Model(&models.BusinessHours{}).Create(map[string]interface{}{
		"establishment_id": hours.EstablishmentID,
		"day_of_week":      hours.DayOfWeek,
		"is_open":          hours.IsOpen,
		"open_time":        hours.OpenTime,
		"close_time":       hours.CloseTime,
		"break_start_time": hours.BreakStartTime,
		"break_end_time":   hours.BreakEndTime,
	}).Error
	if err != nil {
		return hours, err
	}
	err = db.Where("establishment_id = ? AND day_of_week = ?", req.EstablishmentID, req.DayOfWeek).
		First(&hours).Error
	return hours, err
}

func UpsertBusinessHours(c *fiber.Ctx) error {
	var req BusinessHoursRequest
	if err := c.BodyParser(&req); err != nil || !validBusinessHours(req) {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}
	if !authorizeBusinessHours(c, req.EstablishmentID) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Forbidden"})
	}

	hours, err := saveBusinessHours(models.DB, req)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to save hours"})
	}
	return c.JSON(hours)
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
	// Autoriza TUDO antes de gravar qualquer coisa: um item alheio no meio do
	// lote não pode deixar os anteriores gravados.
	for _, req := range reqs {
		if !validBusinessHours(req) {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
		}
		if !authorizeBusinessHours(c, req.EstablishmentID) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Forbidden"})
		}
	}

	err := models.DB.Transaction(func(tx *gorm.DB) error {
		for _, req := range reqs {
			if _, err := saveBusinessHours(tx, req); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to save hours"})
	}
	return c.JSON(fiber.Map{"message": "Horários atualizados"})
}
