package handlers

import (
	"strconv"
	"time"

	"github.com/carloshomar/fuudelivery/auth_api/app/middlewares"
	"github.com/carloshomar/fuudelivery/orders_api/app/models"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// CreateBatch cria um novo lote de entregas.
// POST /batches
func CreateBatch(c *fiber.Ctx) error {
	var req struct {
		ZoneID      uint    `json:"zone_id"`
		CourierID   *uint   `json:"courier_id,omitempty"`
		MaxDetourKm float64 `json:"max_detour_km"`
		OriginLat   float64 `json:"origin_lat"`
		OriginLng   float64 `json:"origin_lng"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if req.ZoneID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "zone_id is required"})
	}
	if req.MaxDetourKm == 0 {
		req.MaxDetourKm = 3.0
	}

	batch := models.Batch{
		ZoneID:      req.ZoneID,
		CourierID:   req.CourierID,
		Status:      "active",
		MaxDetourKm: req.MaxDetourKm,
		OriginLat:   req.OriginLat,
		OriginLng:   req.OriginLng,
	}

	if err := models.DB.Create(&batch).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create batch"})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message": "Batch created successfully",
		"batch":   batch,
	})
}

// GetBatch retorna os detalhes de um lote.
// GET /batches/:id
func GetBatch(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid batch ID"})
	}

	var batch models.Batch
	if err := models.DB.First(&batch, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Batch not found"})
	}

	// Busca orders vinculadas a este lote
	var orders []models.Order
	models.DB.Where("batch_id = ?", id).Find(&orders)

	return c.JSON(fiber.Map{
		"batch":  batch,
		"orders": orders,
	})
}

// AssignBatch atribui um lote a um entregador.
// POST /batches/:id/assign
func AssignBatch(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid batch ID"})
	}

	var req struct {
		CourierID uint `json:"courier_id"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if req.CourierID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "courier_id is required"})
	}

	var batch models.Batch
	if err := models.DB.First(&batch, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Batch not found"})
	}

	if batch.Status != "active" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Batch is not active"})
	}

	if !canBatchAct(c, &batch) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Forbidden"})
	}

	now := time.Now()
	batch.CourierID = &req.CourierID
	batch.Status = "delivering"
	batch.StartedAt = &now

	if err := models.DB.Save(&batch).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to assign batch"})
	}

	return c.JSON(fiber.Map{
		"message": "Batch assigned successfully",
		"batch":   batch,
	})
}

// CompleteBatch finaliza um lote de entregas.
// POST /batches/:id/complete
func CompleteBatch(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid batch ID"})
	}

	var batch models.Batch
	if err := models.DB.First(&batch, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Batch not found"})
	}

	if batch.Status != "delivering" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Batch is not in delivering status"})
	}

	if !canBatchAct(c, &batch) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Forbidden"})
	}

	now := time.Now()
	batch.Status = "completed"
	batch.CompletedAt = &now

	if err := models.DB.Save(&batch).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to complete batch"})
	}

	return c.JSON(fiber.Map{
		"message": "Batch completed successfully",
		"batch":   batch,
	})
}

// ListBatchesByZone lista lotes ativos de uma zona.
// GET /batches/zone/:zoneId
func ListBatchesByZone(c *fiber.Ctx) error {
	zoneID, err := strconv.ParseUint(c.Params("zoneId"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid zone ID"})
	}

	var batches []models.Batch
	if err := models.DB.Where("zone_id = ? AND status != ?", zoneID, "completed").
		Order("created_at desc").
		Find(&batches).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to list batches"})
	}

	return c.JSON(fiber.Map{
		"zone_id": zoneID,
		"total":   len(batches),
		"batches": batches,
	})
}

// AddOrderToBatch vincula um pedido a um lote.
// POST /batches/:id/add-order
func AddOrderToBatch(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid batch ID"})
	}

	var req struct {
		OrderID uint `json:"order_id"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	var batch models.Batch
	if err := models.DB.First(&batch, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Batch not found"})
	}

	if batch.Status != "active" && batch.Status != "delivering" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Batch cannot accept new orders"})
	}

	if !canBatchAct(c, &batch) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Forbidden"})
	}

	var order models.Order
	if err := models.DB.First(&order, req.OrderID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Order not found"})
	}

	batchID := uint(id)
	order.BatchID = &batchID
	if err := models.DB.Save(&order).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update order"})
	}

	if err := models.DB.Model(&batch).Update("total_orders", gorm.Expr("total_orders + ?", 1)).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update batch count"})
	}

	return c.JSON(fiber.Map{
		"message": "Order added to batch successfully",
		"batch":   batch,
		"order":   order,
	})
}

// ForceExpireBatch força a expiracao (cancelamento) de um batch.
// POST /batches/:id/force-expire
func ForceExpireBatch(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid batch ID"})
	}

	var batch models.Batch
	if err := models.DB.First(&batch, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Batch not found"})
	}

	if batch.Status == "cancelled" || batch.Status == "completed" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Batch already ended"})
	}

	if !canBatchAct(c, &batch) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Forbidden"})
	}

	now := time.Now()
	batch.Status = "cancelled"
	batch.CompletedAt = &now

	if err := models.DB.Save(&batch).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to expire batch"})
	}

	// Libera orders vinculadas (remove batch_id)
	models.DB.Model(&models.Order{}).Where("batch_id = ?", id).Update("batch_id", nil)

	return c.JSON(fiber.Map{
		"message": "Batch expired and orders released",
		"batch":   batch,
	})
}

// canBatchAct verifica se o usuário pode agir sobre o lote (admin ou dono do estabelecimento).
func canBatchAct(c *fiber.Ctx, batch *models.Batch) bool {
	role, err := middlewares.GetUserRoleFromToken(c)
	if err != nil {
		return false
	}
	if role == "admin" {
		return true
	}

	var order models.Order
	if err := models.DB.Where("batch_id = ?", batch.ID).First(&order).Error; err != nil {
		return false
	}

	return canActOnEstablishment(c, int64(order.EstablishmentID))
}
