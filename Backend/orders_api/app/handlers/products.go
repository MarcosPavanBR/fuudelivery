package handlers

import (
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

	// Uma consulta, filtrada pela loja, com as duas relações. Havia um
	// segundo Find(&product) SEM filtro só para o Preload das categorias —
	// ele recarregava a tabela inteira, e o cardápio de cada loja vinha com os
	// produtos de todas as lojas.
	if err := models.DB.Where("establishment_id = ?", establishmentId).
		Preload("Additional").Preload("Categories").
		Find(&product).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch products"})
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
	if !validID(productID) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "ID inválido"})
	}
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

	// Devolve as categorias COM os produtos. A versão anterior montava a
	// lista com produtos num laço N+1, descartava e devolvia só as categorias.
	var categories []models.Category
	if err := models.DB.Preload("Products").Where(&models.Category{
		EstablishmentID: uint(establishmentId),
	}).Find(&categories).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch categories"})
	}

	return c.JSON(&categories)
}

func DeleteProduct(c *fiber.Ctx) error {
	productID := c.Params("id")
	if !validID(productID) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "ID inválido"})
	}

	var existingProduct models.Product
	if err := models.DB.First(&existingProduct, "id = ?", productID).Error; err != nil {
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
