package handlers

import (
	"github.com/carloshomar/fuudelivery/orders_api/app/dto"
	"github.com/carloshomar/fuudelivery/orders_api/app/models"
	"github.com/gofiber/fiber/v2"
)

func CreateCategories(c *fiber.Ctx) error {
	var request dto.CategorieRequest

	if err := c.BodyParser(&request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Failed to parse request body",
		})
	}

	if !canActOnEstablishment(c, int64(request.EstablishmentId)) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Forbidden"})
	}

	categorie := models.Category{
		Name:            request.Name,
		Image:           request.Image,
		EstablishmentID: request.EstablishmentId,
	}

	if err := models.DB.Create(&categorie).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create category"})
	}
	request.Id = categorie.ID

	return c.JSON(&request)
}

func GetCategories(c *fiber.Ctx) error {
	establishmentId, err := c.ParamsInt("establishmentId")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Failed to parse request body"})
	}

	var categories []models.Category

	if err := models.DB.Where(&models.Category{
		EstablishmentID: uint(establishmentId),
	}).Find(&categories).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch categories"})
	}

	return c.JSON(categories)
}

func CreateProductCategorie(c *fiber.Ctx) error {
	var request dto.CategoryRequest
	if err := c.BodyParser(&request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Failed to parse request body"})
	}

	var category models.Category
	if err := models.DB.First(&category, request.CategoryID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Category not found"})
	}

	if !canActOnEstablishment(c, int64(category.EstablishmentID)) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Forbidden"})
	}

	var existingCategoryProduct models.CategoryProducts
	result := models.DB.Where(&models.CategoryProducts{
		CategoryID: request.CategoryID,
		ProductID:  request.ProductID,
	}).First(&existingCategoryProduct)

	if result.RowsAffected > 0 {
		if err := models.DB.Where(&models.CategoryProducts{
			CategoryID: request.CategoryID,
			ProductID:  request.ProductID,
		}).Delete(&existingCategoryProduct).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to remove category link"})
		}
		return c.JSON(&existingCategoryProduct)

	}

	categoryProduct := models.CategoryProducts{
		CategoryID: request.CategoryID,
		ProductID:  request.ProductID,
	}

	if err := models.DB.Create(&categoryProduct).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create category link"})
	}

	return c.JSON(&categoryProduct)
}

func GetCategoriesWithProducts(c *fiber.Ctx) error {
	establishmentID, err := c.ParamsInt("establishmentId")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Failed to parse establishment ID"})
	}

	var categories []models.Category
	var categoriesWithProducts []dto.CategorieRequest

	if err := models.DB.Where(&models.Category{
		EstablishmentID: uint(establishmentID),
	}).Find(&categories).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch categories"})
	}

	for _, category := range categories {
		var products []models.Product

		if err := models.DB.Model(&category).Preload("Additional").Association("Products").Find(&products).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch products"})
		}

		categoriesWithProducts = append(categoriesWithProducts,
			dto.CategorieRequest{
				Id:              category.ID,
				Name:            category.Name,
				Image:           category.Image,
				EstablishmentId: category.EstablishmentID,
				Products:        products,
			},
		)
	}

	return c.JSON(&categoriesWithProducts)
}

func DeleteCategory(c *fiber.Ctx) error {
	categoryID := c.Params("id")

	var existingCategory models.Category
	if err := models.DB.First(&existingCategory, categoryID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Category not found"})
	}

	if !canActOnEstablishment(c, int64(existingCategory.EstablishmentID)) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Forbidden"})
	}

	if err := models.DB.Where("category_id = ?", categoryID).Delete(&models.CategoryProducts{}).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to delete associated relationships"})
	}

	if err := models.DB.Delete(&existingCategory).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to delete category"})
	}

	return c.JSON(fiber.Map{"message": "Category deleted successfully"})
}

func UpdateCategory(c *fiber.Ctx) error {
	var request dto.CategorieRequest

	if err := c.BodyParser(&request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Failed to parse request body"})
	}

	categoryID := c.Params("id")

	var existingCategory models.Category
	if err := models.DB.First(&existingCategory, categoryID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Category not found"})
	}

	if !canActOnEstablishment(c, int64(existingCategory.EstablishmentID)) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Forbidden"})
	}

	existingCategory.Name = request.Name
	existingCategory.Image = request.Image

	if err := models.DB.Save(&existingCategory).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update category"})
	}

	return c.JSON(&existingCategory)
}
