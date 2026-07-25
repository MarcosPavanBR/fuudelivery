package handlers

import (
	"strconv"
	"time"

	"github.com/carloshomar/vercardapio/auth_api/app/models"
	"github.com/gofiber/fiber/v2"
)

// ListZones retorna todas as zonas ativas.
// GET /api/zones
func ListZones(c *fiber.Ctx) error {
	var zones []models.Zone
	if err := models.DB.Where("is_active = ?", true).Order("name asc").Find(&zones).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to list zones"})
	}
	return c.JSON(zones)
}

// ListAllZones retorna todas as zonas (inclusive inativas).
// GET /api/zones/all
func ListAllZones(c *fiber.Ctx) error {
	var zones []models.Zone
	if err := models.DB.Order("name asc").Find(&zones).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to list zones"})
	}
	return c.JSON(zones)
}

// GetZone retorna uma zona pelo ID.
// GET /api/zones/:id
func GetZone(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid zone ID"})
	}

	var zone models.Zone
	if err := models.DB.First(&zone, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Zone not found"})
	}
	return c.JSON(zone)
}

// CreateZone cria uma nova zona com todos os parametros configuracao.
// POST /api/zones
// Exemplo: {
//   "name": "Centro-SP",
//   "city": "Sao Paulo", "state": "SP",
//   "platform_fee_percentage": 12.0, "establishment_percentage": 78.0,
//   "radius_km": 2.5, "min_radius_km": 1.0, "max_radius_km": 5.0,
//   "peak_hour_start": "11:00", "peak_hour_end": "14:00",
//   "peak_radius_multiplier": 0.7,
//   "city_size": "metro",
//   "min_delivery_fee": 7.0,
//   "min_couriers_threshold": 5
// }
func CreateZone(c *fiber.Ctx) error {
	var req struct {
		Name                    string   `json:"name"`
		City                    string   `json:"city"`
		State                   string   `json:"state"`
		GeohashPrefix           string   `json:"geohash_prefix"`
		PlatformFeePercentage   *float64 `json:"platform_fee_percentage"`
		EstablishmentPercentage *float64 `json:"establishment_percentage"`

		MinRadiusKm *float64 `json:"min_radius_km"`
		RadiusKm    *float64 `json:"radius_km"`
		MaxRadiusKm *float64 `json:"max_radius_km"`

		PeakHourStart        *string  `json:"peak_hour_start"`
		PeakHourEnd          *string  `json:"peak_hour_end"`
		PeakRadiusMultiplier *float64 `json:"peak_radius_multiplier"`

		CitySize             *string  `json:"city_size"`
		MinDeliveryFee       *float64 `json:"min_delivery_fee"`
		SurgeMultiplier      *float64 `json:"surge_multiplier"`
		MinCouriersThreshold *int     `json:"min_couriers_threshold"`
		MatchAlgorithm       *string  `json:"match_algorithm"`
		AllowBatching        *bool    `json:"allow_batching"`

		// Decaimento de split
		SplitInitialPlatformPct       *float64 `json:"split_initial_platform_pct"`
		SplitInitialEstablishmentPct  *float64 `json:"split_initial_establishment_pct"`
		SplitTargetPlatformPct        *float64 `json:"split_target_platform_pct"`
		SplitTargetEstablishmentPct   *float64 `json:"split_target_establishment_pct"`
		SplitStepMonths               *int     `json:"split_step_months"`
		SplitStepPlatformPct          *float64 `json:"split_step_platform_pct"`
		SplitStepEstablishmentPct     *float64 `json:"split_step_establishment_pct"`
		SplitMinMonthlyOrders         *int     `json:"split_min_monthly_orders"`
		SplitMinActiveCouriers        *int     `json:"split_min_active_couriers"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if req.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Zone name is required"})
	}

	zone := models.Zone{
		Name:                    req.Name,
		City:                    req.City,
		State:                   req.State,
		GeohashPrefix:           req.GeohashPrefix,
		IsActive:                true,
	}

	// Aplica defaults para split se nao informado
	zone.PlatformFeePercentage = 5.0
	zone.EstablishmentPercentage = 85.0
	if req.PlatformFeePercentage != nil {
		zone.PlatformFeePercentage = *req.PlatformFeePercentage
	}
	if req.EstablishmentPercentage != nil {
		zone.EstablishmentPercentage = *req.EstablishmentPercentage
	}

	// Aplica defaults para raios baseados no porte da cidade
	zone.CitySize = "medium"
	if req.CitySize != nil {
		zone.CitySize = *req.CitySize
	}

	// Se raios nao foram informados, calcula defaults por porte
	if req.MinRadiusKm == nil || req.RadiusKm == nil || req.MaxRadiusKm == nil {
		base, min, max := zone.GetInitialRadiusByCitySize()
		zone.MinRadiusKm = min
		zone.RadiusKm = base
		zone.MaxRadiusKm = max
	}
	if req.MinRadiusKm != nil {
		zone.MinRadiusKm = *req.MinRadiusKm
	}
	if req.RadiusKm != nil {
		zone.RadiusKm = *req.RadiusKm
	}
	if req.MaxRadiusKm != nil {
		zone.MaxRadiusKm = *req.MaxRadiusKm
	}

	// Campos opcionais
	if req.PeakHourStart != nil {
		zone.PeakHourStart = *req.PeakHourStart
	}
	if req.PeakHourEnd != nil {
		zone.PeakHourEnd = *req.PeakHourEnd
	}
	if req.PeakRadiusMultiplier != nil {
		zone.PeakRadiusMultiplier = *req.PeakRadiusMultiplier
	}
	if req.MinDeliveryFee != nil {
		zone.MinDeliveryFee = *req.MinDeliveryFee
	}
	if req.SurgeMultiplier != nil {
		zone.SurgeMultiplier = *req.SurgeMultiplier
	}
	if req.MinCouriersThreshold != nil {
		zone.MinCouriersThreshold = *req.MinCouriersThreshold
	}
	if req.MatchAlgorithm != nil {
		zone.MatchAlgorithm = *req.MatchAlgorithm
	}
	if req.AllowBatching != nil {
		zone.AllowBatching = *req.AllowBatching
	}

	// === Decaimento de split ===
	if req.SplitInitialPlatformPct != nil {
		zone.SplitInitialPlatformPct = *req.SplitInitialPlatformPct
	}
	if req.SplitInitialEstablishmentPct != nil {
		zone.SplitInitialEstablishmentPct = *req.SplitInitialEstablishmentPct
	}
	if req.SplitTargetPlatformPct != nil {
		zone.SplitTargetPlatformPct = *req.SplitTargetPlatformPct
	}
	if req.SplitTargetEstablishmentPct != nil {
		zone.SplitTargetEstablishmentPct = *req.SplitTargetEstablishmentPct
	}
	if req.SplitStepMonths != nil {
		zone.SplitStepMonths = *req.SplitStepMonths
	}
	if req.SplitStepPlatformPct != nil {
		zone.SplitStepPlatformPct = *req.SplitStepPlatformPct
	}
	if req.SplitStepEstablishmentPct != nil {
		zone.SplitStepEstablishmentPct = *req.SplitStepEstablishmentPct
	}
	if req.SplitMinMonthlyOrders != nil {
		zone.SplitMinMonthlyOrders = *req.SplitMinMonthlyOrders
	}
	if req.SplitMinActiveCouriers != nil {
		zone.SplitMinActiveCouriers = *req.SplitMinActiveCouriers
	}

	// Sincroniza split current com initial na criacao
	zone.SplitCurrentPlatformPct = zone.SplitInitialPlatformPct
	zone.SplitCurrentEstablishmentPct = zone.SplitInitialEstablishmentPct

	if err := models.DB.Create(&zone).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create zone"})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message": "Zone created successfully",
		"zone":    zone,
	})
}

// UpdateZone atualiza os dados de uma zona (partial update).
// PUT /api/zones/:id
func UpdateZone(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid zone ID"})
	}

	var zone models.Zone
	if err := models.DB.First(&zone, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Zone not found"})
	}

	var req struct {
		Name                    *string  `json:"name"`
		City                    *string  `json:"city"`
		State                   *string  `json:"state"`
		GeohashPrefix           *string  `json:"geohash_prefix"`
		PlatformFeePercentage   *float64 `json:"platform_fee_percentage"`
		EstablishmentPercentage *float64 `json:"establishment_percentage"`

		MinRadiusKm *float64 `json:"min_radius_km"`
		RadiusKm    *float64 `json:"radius_km"`
		MaxRadiusKm *float64 `json:"max_radius_km"`

		PeakHourStart        *string  `json:"peak_hour_start"`
		PeakHourEnd          *string  `json:"peak_hour_end"`
		PeakRadiusMultiplier *float64 `json:"peak_radius_multiplier"`

		CitySize             *string  `json:"city_size"`
		MinDeliveryFee       *float64 `json:"min_delivery_fee"`
		SurgeMultiplier      *float64 `json:"surge_multiplier"`
		MinCouriersThreshold *int     `json:"min_couriers_threshold"`
		MatchAlgorithm       *string  `json:"match_algorithm"`
		AllowBatching        *bool    `json:"allow_batching"`
		IsActive             *bool    `json:"is_active"`

		// Decaimento de split
		SplitInitialPlatformPct       *float64 `json:"split_initial_platform_pct"`
		SplitInitialEstablishmentPct  *float64 `json:"split_initial_establishment_pct"`
		SplitTargetPlatformPct        *float64 `json:"split_target_platform_pct"`
		SplitTargetEstablishmentPct   *float64 `json:"split_target_establishment_pct"`
		SplitStepMonths               *int     `json:"split_step_months"`
		SplitStepPlatformPct          *float64 `json:"split_step_platform_pct"`
		SplitStepEstablishmentPct     *float64 `json:"split_step_establishment_pct"`
		SplitMinMonthlyOrders         *int     `json:"split_min_monthly_orders"`
		SplitMinActiveCouriers        *int     `json:"split_min_active_couriers"`
		SplitCurrentPlatformPct       *float64 `json:"split_current_platform_pct"`
		SplitCurrentEstablishmentPct  *float64 `json:"split_current_establishment_pct"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if req.Name != nil {
		zone.Name = *req.Name
	}
	if req.City != nil {
		zone.City = *req.City
	}
	if req.State != nil {
		zone.State = *req.State
	}
	if req.GeohashPrefix != nil {
		zone.GeohashPrefix = *req.GeohashPrefix
	}
	if req.PlatformFeePercentage != nil {
		zone.PlatformFeePercentage = *req.PlatformFeePercentage
	}
	if req.EstablishmentPercentage != nil {
		zone.EstablishmentPercentage = *req.EstablishmentPercentage
	}
	if req.MinRadiusKm != nil {
		zone.MinRadiusKm = *req.MinRadiusKm
	}
	if req.RadiusKm != nil {
		zone.RadiusKm = *req.RadiusKm
	}
	if req.MaxRadiusKm != nil {
		zone.MaxRadiusKm = *req.MaxRadiusKm
	}
	if req.PeakHourStart != nil {
		zone.PeakHourStart = *req.PeakHourStart
	}
	if req.PeakHourEnd != nil {
		zone.PeakHourEnd = *req.PeakHourEnd
	}
	if req.PeakRadiusMultiplier != nil {
		zone.PeakRadiusMultiplier = *req.PeakRadiusMultiplier
	}
	if req.CitySize != nil {
		zone.CitySize = *req.CitySize
	}
	if req.MinDeliveryFee != nil {
		zone.MinDeliveryFee = *req.MinDeliveryFee
	}
	if req.SurgeMultiplier != nil {
		zone.SurgeMultiplier = *req.SurgeMultiplier
	}
	if req.MinCouriersThreshold != nil {
		zone.MinCouriersThreshold = *req.MinCouriersThreshold
	}
	if req.MatchAlgorithm != nil {
		zone.MatchAlgorithm = *req.MatchAlgorithm
	}
	if req.AllowBatching != nil {
		zone.AllowBatching = *req.AllowBatching
	}
	if req.IsActive != nil {
		zone.IsActive = *req.IsActive
	}

	// === Decaimento de split ===
	if req.SplitInitialPlatformPct != nil {
		zone.SplitInitialPlatformPct = *req.SplitInitialPlatformPct
	}
	if req.SplitInitialEstablishmentPct != nil {
		zone.SplitInitialEstablishmentPct = *req.SplitInitialEstablishmentPct
	}
	if req.SplitTargetPlatformPct != nil {
		zone.SplitTargetPlatformPct = *req.SplitTargetPlatformPct
	}
	if req.SplitTargetEstablishmentPct != nil {
		zone.SplitTargetEstablishmentPct = *req.SplitTargetEstablishmentPct
	}
	if req.SplitStepMonths != nil {
		zone.SplitStepMonths = *req.SplitStepMonths
	}
	if req.SplitStepPlatformPct != nil {
		zone.SplitStepPlatformPct = *req.SplitStepPlatformPct
	}
	if req.SplitStepEstablishmentPct != nil {
		zone.SplitStepEstablishmentPct = *req.SplitStepEstablishmentPct
	}
	if req.SplitMinMonthlyOrders != nil {
		zone.SplitMinMonthlyOrders = *req.SplitMinMonthlyOrders
	}
	if req.SplitMinActiveCouriers != nil {
		zone.SplitMinActiveCouriers = *req.SplitMinActiveCouriers
	}
	if req.SplitCurrentPlatformPct != nil {
		zone.SplitCurrentPlatformPct = *req.SplitCurrentPlatformPct
	}
	if req.SplitCurrentEstablishmentPct != nil {
		zone.SplitCurrentEstablishmentPct = *req.SplitCurrentEstablishmentPct
	}

	if err := models.DB.Save(&zone).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update zone"})
	}

	return c.JSON(fiber.Map{
		"message": "Zone updated successfully",
		"zone":    zone,
	})
}

// DeleteZone desativa (soft delete) uma zona.
// DELETE /api/zones/:id
func DeleteZone(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid zone ID"})
	}

	var zone models.Zone
	if err := models.DB.First(&zone, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Zone not found"})
	}

	zone.IsActive = false
	if err := models.DB.Save(&zone).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to deactivate zone"})
	}

	return c.JSON(fiber.Map{"message": "Zone deactivated successfully"})
}

// CalibrateZone aciona a calibracao manual de uma zona especifica.
// Recalcula a densidade e ajusta o raio baseado nos dados atuais.
// POST /api/zones/:id/calibrate
func CalibrateZone(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid zone ID"})
	}

	var zone models.Zone
	if err := models.DB.First(&zone, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Zone not found"})
	}

	// Le dados de calibracao opcionais do body
	var req struct {
		DensityCouriersPerKm2 *float64 `json:"density_couriers_per_km2"`
		RadiusKm              *float64 `json:"radius_km"`
		MinRadiusKm           *float64 `json:"min_radius_km"`
		MaxRadiusKm           *float64 `json:"max_radius_km"`
	}

	if err := c.BodyParser(&req); err == nil {
		if req.DensityCouriersPerKm2 != nil {
			zone.DensityCouriersPerKm2 = *req.DensityCouriersPerKm2
		}
		if req.RadiusKm != nil {
			zone.RadiusKm = *req.RadiusKm
		}
		if req.MinRadiusKm != nil {
			zone.MinRadiusKm = *req.MinRadiusKm
		}
		if req.MaxRadiusKm != nil {
			zone.MaxRadiusKm = *req.MaxRadiusKm
		}
	}

	now := time.Now()
	zone.LastCalibratedAt = &now

	if err := models.DB.Save(&zone).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to calibrate zone"})
	}

	return c.JSON(fiber.Map{
		"message": "Zone calibrated successfully",
		"zone":    zone,
	})
}
