package handlers

import (
	"strconv"
	"time"

	"github.com/carloshomar/vercardapio/auth_api/app/middlewares"
	"github.com/carloshomar/vercardapio/auth_api/app/models"
	"github.com/gofiber/fiber/v2"
)

// ListSponsoredListings lista todos os patrocínios (admin).
// GET /api/sponsored
func ListSponsoredListings(c *fiber.Ctx) error {
	var sponsors []models.SponsoredListing
	if err := models.DB.Order("priority desc, created_at desc").Find(&sponsors).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to list sponsored listings"})
	}
	return c.JSON(sponsors)
}

// GetSponsoredListing retorna um patrocínio pelo ID (admin).
// GET /api/sponsored/:id
func GetSponsoredListing(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid sponsored listing ID"})
	}

	var sponsor models.SponsoredListing
	if err := models.DB.First(&sponsor, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Sponsored listing not found"})
	}
	return c.JSON(sponsor)
}

// CreateSponsoredListing cria um novo patrocínio para um estabelecimento.
// Verifica disponibilidade de slots na zona antes de criar.
// POST /api/sponsored
func CreateSponsoredListing(c *fiber.Ctx) error {
	var req struct {
		EstablishmentID uint   `json:"establishment_id"`
		ZoneID          uint   `json:"zone_id"`
		Plan            string `json:"plan"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if req.EstablishmentID == 0 || req.ZoneID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "establishment_id and zone_id are required"})
	}

	if req.Plan == "" {
		req.Plan = models.SponsorPlanBasic
	}
	if req.Plan != models.SponsorPlanBasic && req.Plan != models.SponsorPlanPremium {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid plan. Use 'basic' or 'premium'",
		})
	}

	// Verifica se estabelecimento existe
	var est models.Establishment
	if err := models.DB.First(&est, req.EstablishmentID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Establishment not found"})
	}

	// Verifica se zona existe e está ativa
	var zone models.Zone
	if err := models.DB.First(&zone, req.ZoneID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Zone not found"})
	}
	if !zone.IsActive {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Zone is inactive"})
	}

	// Verifica se estabelecimento já tem patrocínio ativo nesta zona
	existing, _ := models.GetSponsoredByEstablishment(req.EstablishmentID, req.ZoneID)
	if existing != nil && existing.IsActive() {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "Establishment already has an active sponsorship in this zone"})
	}

	// Verifica disponibilidade de slots na zona
	available, err := models.CheckSponsoredSlotsAvailable(req.ZoneID, req.Plan)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to check slot availability"})
	}
	if !available {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error": "No available sponsored slots for this plan in this zone. Maximum slots reached.",
		})
	}

	amount := models.GetSponsorPlanAmount(req.Plan)
	hasBanner, hasPush := models.GetSponsorPlanBenefits(req.Plan)
	priority := models.PriorityScheduler(req.ZoneID, req.Plan)
	now := time.Now()

	sponsor := models.SponsoredListing{
		EstablishmentID:     req.EstablishmentID,
		ZoneID:              req.ZoneID,
		Plan:                req.Plan,
		Status:              models.SponsorStatusActive,
		Amount:              amount,
		Priority:            priority,
		HasBanner:           hasBanner,
		HasPushNotification: hasPush,
		StartDate:           now,
		EndDate:             now.AddDate(0, 1, 0), // +1 mês
	}

	if err := models.DB.Create(&sponsor).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create sponsored listing"})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message":  "Sponsored listing created successfully",
		"sponsor":  sponsor,
	})
}

// CancelSponsoredListing cancela um patrocínio ativo.
// POST /api/sponsored/:id/cancel
func CancelSponsoredListing(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid sponsored listing ID"})
	}

	var sponsor models.SponsoredListing
	if err := models.DB.First(&sponsor, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Sponsored listing not found"})
	}

	if sponsor.Status != models.SponsorStatusActive {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Sponsorship is not active"})
	}

	now := time.Now()
	sponsor.Status = models.SponsorStatusCancelled
	sponsor.CancelledAt = &now

	if err := models.DB.Save(&sponsor).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to cancel sponsored listing"})
	}

	return c.JSON(fiber.Map{
		"message": "Sponsorship cancelled. Establishment benefits end immediately.",
		"sponsor": sponsor,
	})
}

// RenewSponsoredListing renova um patrocínio por mais um mês.
// POST /api/sponsored/:id/renew
func RenewSponsoredListing(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid sponsored listing ID"})
	}

	var sponsor models.SponsoredListing
	if err := models.DB.First(&sponsor, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Sponsored listing not found"})
	}

	if sponsor.Status == models.SponsorStatusExpired {
		// Reativa: novo período
		now := time.Now()
		sponsor.Status = models.SponsorStatusActive
		sponsor.StartDate = now
		sponsor.EndDate = now.AddDate(0, 1, 0)
		sponsor.CancelledAt = nil
	} else if sponsor.Status == models.SponsorStatusCancelled {
		// Verifica se ainda há slots
		available, err := models.CheckSponsoredSlotsAvailable(sponsor.ZoneID, sponsor.Plan)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to check slot availability"})
		}
		if !available {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"error": "No available sponsored slots for this plan in this zone. Maximum slots reached.",
			})
		}
		now := time.Now()
		sponsor.Status = models.SponsorStatusActive
		sponsor.StartDate = now
		sponsor.EndDate = now.AddDate(0, 1, 0)
		sponsor.CancelledAt = nil
	} else if sponsor.Status == models.SponsorStatusActive {
		// Estende por +1 mês
		sponsor.EndDate = sponsor.EndDate.AddDate(0, 1, 0)
	}

	if err := models.DB.Save(&sponsor).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to renew sponsored listing"})
	}

	return c.JSON(fiber.Map{
		"message": "Sponsorship renewed successfully",
		"sponsor": sponsor,
	})
}

// UpdateSponsoredListing atualiza um patrocínio (admin).
// PUT /api/sponsored/:id
func UpdateSponsoredListing(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid sponsored listing ID"})
	}

	var sponsor models.SponsoredListing
	if err := models.DB.First(&sponsor, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Sponsored listing not found"})
	}

	var req struct {
		Plan      *string  `json:"plan"`
		Status    *string  `json:"status"`
		Amount    *float64 `json:"amount"`
		Priority  *int     `json:"priority"`
		StartDate *string  `json:"start_date"`
		EndDate   *string  `json:"end_date"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if req.Plan != nil {
		sponsor.Plan = *req.Plan
		// Recalcula benefícios ao mudar de plano
		amount := models.GetSponsorPlanAmount(sponsor.Plan)
		sponsor.Amount = amount
		hasBanner, hasPush := models.GetSponsorPlanBenefits(sponsor.Plan)
		sponsor.HasBanner = hasBanner
		sponsor.HasPushNotification = hasPush
	}
	if req.Status != nil {
		sponsor.Status = *req.Status
	}
	if req.Amount != nil {
		sponsor.Amount = *req.Amount
	}
	if req.Priority != nil {
		sponsor.Priority = *req.Priority
	}

	if err := models.DB.Save(&sponsor).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update sponsored listing"})
	}

	return c.JSON(fiber.Map{
		"message": "Sponsored listing updated successfully",
		"sponsor": sponsor,
	})
}

// GetEstablishmentSponsorship retorna o patrocínio ativo de um estabelecimento.
// GET /api/sponsored/by-establishment/:id
func GetEstablishmentSponsorship(c *fiber.Ctx) error {
	estID, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid establishment ID"})
	}

	// Busca o estabelecimento para descobrir a zona
	var est models.Establishment
	if err := models.DB.First(&est, estID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Establishment not found"})
	}

	if est.ZoneID == nil || *est.ZoneID == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Establishment has no zone assigned"})
	}

	sponsor, err := models.GetSponsoredByEstablishment(uint(estID), *est.ZoneID)
	if err != nil || sponsor == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "No active sponsorship found"})
	}

	return c.JSON(sponsor)
}

// GetFeaturedEstablishments retorna os estabelecimentos em destaque de uma zona.
// Endpoint público — não requer autenticação.
// GET /api/establishments/featured?zone_id=1&limit=8
func GetFeaturedEstablishments(c *fiber.Ctx) error {
	zoneIDStr := c.Query("zone_id")
	if zoneIDStr == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "zone_id query parameter is required"})
	}

	zoneID, err := strconv.ParseUint(zoneIDStr, 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid zone_id"})
	}

	limit := 8
	limitStr := c.Query("limit")
	if limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	featured, err := models.GetFeaturedEstablishments(uint(zoneID), limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch featured establishments"})
	}

	return c.JSON(fiber.Map{
		"zone_id":     zoneID,
		"total":       len(featured),
		"featured":    featured,
	})
}

// ListSponsoredByZone retorna todos os patrocínios ativos de uma zona.
// GET /api/sponsored/by-zone/:id
func ListSponsoredByZone(c *fiber.Ctx) error {
	zoneID, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid zone ID"})
	}

	sponsors, err := models.GetSponsoredListingsByZone(uint(zoneID))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to list sponsored listings"})
	}

	return c.JSON(fiber.Map{
		"zone_id":    zoneID,
		"total":      len(sponsors),
		"sponsors":   sponsors,
	})
}

// helper: extrai user_id do token (usada pelos handlers que recebem user context)
func getSponsoredUserID(c *fiber.Ctx) (int64, error) {
	return middlewares.GetUserIDFromToken(c)
}
