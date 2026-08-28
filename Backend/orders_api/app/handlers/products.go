package handlers

import (
	"github.com/carloshomar/fuudelivery/auth_api/app/middlewares"
	"github.com/carloshomar/fuudelivery/orders_api/app/dto"
	"github.com/carloshomar/fuudelivery/orders_api/app/models"
	"github.com/gofiber/fiber/v2"
)

func Ping(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{})
}

func GetByEstablishmentId(c *fiber.Ctx) error {
	establishmentId, err := c.ParamsInt("establishmentId")

	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Failed to parse request body"})
	}

	var product []models.Product

	if err := models.DB.Where(&models.Product{
		EstablishmentID: uint(establishmentId),
	}).Preload("Additional").Find(&product).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch products"})
	}

	if err := models.DB.Preload("Categories").Find(&product).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch product categories"})
	}

	return c.JSON(&product)
}

func CreateProduct(c *fiber.Ctx) error {
	var request dto.ProductRequest
	if err := c.BodyParser(&request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Failed to parse request body"})
	}

	if !canActOnEstablishment(c, int64(request.EstablishmentID)) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Forbidden"})
	}

	product := models.Product{
		Name:            request.Name,
		Description:     request.Description,
		Price:           request.Price,
		Image:           request.Image,
		EstablishmentID: uint(request.EstablishmentID),
	}
	if err := models.DB.Create(&product).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create product"})
	}

	return c.JSON(&product)
}

func UpdateProduct(c *fiber.Ctx) error {

	var request dto.ProductRequest
	if err := c.BodyParser(&request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Failed to parse request body"})
	}

	productID := c.Params("id")
	if productID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Product ID is required"})
	}

	var existingProduct models.Product
	if err := models.DB.Where("id = ?", productID).First(&existingProduct).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Product not found"})
	}

	if !canActOnEstablishment(c, int64(existingProduct.EstablishmentID)) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Forbidden"})
	}

	existingProduct.Name = request.Name
	existingProduct.Description = request.Description
	existingProduct.Price = request.Price
	existingProduct.Image = request.Image

	if err := models.DB.Save(&existingProduct).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update product"})
	}

	return c.JSON(existingProduct)
}

func CreateMultProducts(c *fiber.Ctx) error {
	var requests []dto.ProductRequest
	if err := c.BodyParser(&requests); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Failed to parse request body"})
	}

	if len(requests) > 0 && !canActOnEstablishment(c, int64(requests[0].EstablishmentID)) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Forbidden"})
	}

	var createdProducts []models.Product

	for _, request := range requests {
		product := models.Product{
			Name:            request.Name,
			Description:     request.Description,
			Price:           request.Price,
			Image:           request.Image,
			EstablishmentID: uint(request.EstablishmentID),
		}

		if err := models.DB.Create(&product).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create product"})
		}
		createdProducts = append(createdProducts, product)
	}

	return c.JSON(&createdProducts)
}

func GetByEstablishmentIdWithRelations(c *fiber.Ctx) error {
	establishmentId, err := c.ParamsInt("establishmentId")

	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Failed to parse request body"})
	}

	var categories []models.Category
	var categoriesWithProducts []models.CategoryProducts

	models.DB.Where(&models.Category{
		EstablishmentID: uint(establishmentId),
	}).Find(&categories)

	for _, category := range categories {
		var products []models.Product

		models.DB.Model(&category).Association("Products").Find(&products)

		categoriesWithProducts = append(categoriesWithProducts, models.CategoryProducts{
			Category: category,
		})

	}

	return c.JSON(&categories)
}

func DeleteProduct(c *fiber.Ctx) error {
	productID := c.Params("id")

	var existingProduct models.Product
	if err := models.DB.First(&existingProduct, productID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Product not found"})
	}

	if !canActOnEstablishment(c, int64(existingProduct.EstablishmentID)) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Forbidden"})
	}

	if err := models.DB.Where("product_id = ?", productID).Delete(&models.CategoryProducts{}).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to delete associated relationships"})
	}

	if err := models.DB.Where("product_id = ?", productID).Delete(&models.AdditionalProducts{}).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to delete associated relationships"})
	}

	if err := models.DB.Delete(&existingProduct).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to delete product"})
	}

	return c.JSON(fiber.Map{"message": "Product deleted successfully"})
}
