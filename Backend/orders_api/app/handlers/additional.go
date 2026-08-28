package handlers

import (
	"github.com/carloshomar/fuudelivery/orders_api/app/dto"
	"github.com/carloshomar/fuudelivery/orders_api/app/models"
	"github.com/gofiber/fiber/v2"
)

func CreateAdditional(c *fiber.Ctx) error {
	var request dto.AdditionalRequest
	if err := c.BodyParser(&request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Failed to parse request body"})
	}

	if !canActOnEstablishment(c, int64(request.EstablishmentID)) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Forbidden"})
	}

	additional := models.Additional{
		Name:            request.Name,
		Price:           request.Price,
		Image:           request.Image,
		Description:     request.Description,
		EstablishmentID: request.EstablishmentID,
	}

	if err := models.DB.Create(&additional).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create additional item"})
	}

	return c.JSON(&additional)
}

func ListAdditional(c *fiber.Ctx) error {
	establishmentId, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Failed to parse establishment ID"})
	}

	var additionals []models.Additional
	if err := models.DB.Where("establishment_id = ?", establishmentId).Find(&additionals).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch additional items"})
	}

	return c.JSON(additionals)
}

func UpdateAdditional(c *fiber.Ctx) error {
	additionalID := c.Params("id")

	var existingAdditional models.Additional
	if err := models.DB.First(&existingAdditional, additionalID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Additional item not found"})
	}

	if !canActOnEstablishment(c, int64(existingAdditional.EstablishmentID)) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Forbidden"})
	}

	var request dto.AdditionalRequest
	if err := c.BodyParser(&request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Failed to parse request body"})
	}

	existingAdditional.Name = request.Name
	existingAdditional.Price = request.Price
	existingAdditional.Image = request.Image
	existingAdditional.Description = request.Description

	if err := models.DB.Save(&existingAdditional).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update additional item"})
	}

	return c.JSON(existingAdditional)
}

func CreateProductToAdditional(c *fiber.Ctx) error {
	var request dto.AdditionalProductsRequest
	if err := c.BodyParser(&request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Failed to parse request body"})
	}

	var existingProduct models.Product
	result := models.DB.First(&existingProduct, request.ProductID)
	if result.Error != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Product not found"})
	}

	var existingAdditional models.Additional
	if err := models.DB.First(&existingAdditional, request.AdditionalID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Additional item not found"})
	}

	estID := int64(existingProduct.EstablishmentID)
	if estID == 0 {
		estID = int64(existingAdditional.EstablishmentID)
	}
	if !canActOnEstablishment(c, estID) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Forbidden"})
	}

	var existingAdditionalProducts models.AdditionalProducts
	result = models.DB.Where(&models.AdditionalProducts{
		ProductID:    request.ProductID,
		AdditionalID: request.AdditionalID,
	}).First(&existingAdditionalProducts)

	if result.RowsAffected > 0 {
		if err := models.DB.Where(&models.AdditionalProducts{
			ProductID:    request.ProductID,
			AdditionalID: request.AdditionalID,
		}).Delete(&existingAdditionalProducts).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to remove product link"})
		}
		return c.JSON(&existingAdditionalProducts)
	}

	additionalProducts := models.AdditionalProducts{
		ProductID:    request.ProductID,
		AdditionalID: request.AdditionalID,
	}

	if err := models.DB.Create(&additionalProducts).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create product link"})
	}

	return c.JSON(&additionalProducts)
}

func DeleteAdditional(c *fiber.Ctx) error {
	additionalID := c.Params("id")

	var existingAdditional models.Additional
	if err := models.DB.First(&existingAdditional, additionalID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Additional item not found"})
	}

	if !canActOnEstablishment(c, int64(existingAdditional.EstablishmentID)) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Forbidden"})
	}

	if err := models.DB.Where("additional_id = ?", additionalID).Delete(&models.AdditionalProducts{}).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to delete associated relationships"})
	}

	if err := models.DB.Delete(&existingAdditional).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to delete additional item"})
	}

	return c.JSON(fiber.Map{"message": "Additional item deleted successfully"})
}
