package handlers

import (
	"time"

	"github.com/carloshomar/vercardapio/auth_api/app/models"
	"github.com/gofiber/fiber/v2"
)

func CreateEstablishment(c *fiber.Ctx) error {
	var req struct {
		Name         string  `json:"name"`
		Email        string  `json:"email"`
		Phone        string  `json:"phone"`
		Address      string  `json:"address"`
		City         string  `json:"city"`
		State        string  `json:"state"`
		ZipCode      string  `json:"zip_code"`
		Latitude     float64 `json:"latitude"`
		Longitude    float64 `json:"longitude"`
		Status       string  `json:"status"`
		DeliveryFee  float64 `json:"delivery_fee"`
		MinOrder     float64 `json:"min_order"`
		DeliveryTime int     `json:"delivery_time"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if req.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Name is required"})
	}

	locationString := req.Address
	if req.City != "" || req.State != "" {
		if locationString != "" {
			locationString += ", "
		}
		locationString += req.City
		if req.State != "" {
			locationString += " - " + req.State
		}
	}

	maxDist := 10.0
	if req.DeliveryTime > 0 {
		maxDist = float64(req.DeliveryTime) / 5.0
	}

	establishment := models.Establishment{
		Name:                req.Name,
		Description:         "",
		Image:               "",
		PrimaryColor:        "#EA1D2C",
		SecondaryColor:      "#FFFFFF",
		Lat:                 req.Latitude,
		Long:                req.Longitude,
		LocationString:      locationString,
		MaxDistanceDelivery: maxDist,
	}

	result := models.DB.Create(&establishment)
	if result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create establishment"})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message":       "Establishment created successfully",
		"establishment": establishment,
	})
}

func GetEstablishments(c *fiber.Ctx) error {
	establishmentId := c.Params("id")

	var establishment models.Establishment
	if err := models.DB.First(&establishment, establishmentId).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Establishment not found"})
	}
	return c.JSON(establishment)
}
func ListEstablishments(c *fiber.Ctx) error {
	var establishments []models.Establishment
	models.DB.Where("open_data IS NOT NULL").Find(&establishments)
	return c.JSON(establishments)
}

func GetUserByEstablishment(c *fiber.Ctx) error {
	establishmentId, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "id not found"})
	}
	var user []models.User

	if err := models.DB.Select("name", "email", "id", "establishment_id").Where(&models.User{
		EstablishmentID: uint(establishmentId),
	}).Find(&user).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to query users"})
	}

	return c.JSON(user)
}

func HandlerEstablishmentStatus(c *fiber.Ctx) error {
	establishmentID := c.Params("id")

	var establishment models.Establishment
	if err := models.DB.First(&establishment, establishmentID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Establishment not found"})
	}

	if establishment.OpenData != nil {
		establishment.OpenData = nil
	} else {
		currentTime := time.Now()
		currentTimeString := currentTime.Format(time.RFC3339)
		establishment.OpenData = &currentTimeString
	}

	if err := models.DB.Save(&establishment).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update establishment status"})
	}

	return c.JSON(establishment)
}

func UpdateEstablishment(c *fiber.Ctx) error {
	establishmentID := c.Params("id")

	if establishmentID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid establishment ID"})
	}

	existingEstablishment := models.Establishment{}

	if err := models.DB.First(&existingEstablishment, establishmentID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Establishment not found"})
	}

	request := struct {
		Establishment *models.Establishment `json:"establishment"`
	}{}

	if err := c.BodyParser(&request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if request.Establishment == nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "No valid establishment data provided"})
	}

	existingEstablishment.Name = request.Establishment.Name
	existingEstablishment.Description = request.Establishment.Description
	existingEstablishment.Image = request.Establishment.Image
	existingEstablishment.PrimaryColor = request.Establishment.PrimaryColor
	existingEstablishment.HorarioFuncionamento = request.Establishment.HorarioFuncionamento
	existingEstablishment.SecondaryColor = request.Establishment.SecondaryColor
	existingEstablishment.Lat = request.Establishment.Lat
	existingEstablishment.Long = request.Establishment.Long
	existingEstablishment.MaxDistanceDelivery = request.Establishment.MaxDistanceDelivery
	existingEstablishment.LocationString = request.Establishment.LocationString

	if err := models.DB.Save(&existingEstablishment).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update establishment"})
	}

	return c.JSON(fiber.Map{"message": "Establishment updated successfully"})
}

func UpdateEstablishmentWallet(c *fiber.Ctx) error {
	establishmentID := c.Params("id")
	if establishmentID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid establishment ID"})
	}

	var req struct {
		PaymentWalletID string `json:"payment_wallet_id"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if req.PaymentWalletID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "payment_wallet_id is required"})
	}

	var establishment models.Establishment
	if err := models.DB.First(&establishment, establishmentID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Establishment not found"})
	}

	establishment.PaymentWalletID = req.PaymentWalletID
	if err := models.DB.Save(&establishment).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update wallet"})
	}

	return c.JSON(fiber.Map{
		"message": "Wallet ID updated successfully",
		"payment_wallet_id": establishment.PaymentWalletID,
	})
}

func DeleteEstablishment(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid establishment ID"})
	}

	var establishment models.Establishment
	if err := models.DB.First(&establishment, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Establishment not found"})
	}

	if err := models.DB.Delete(&establishment).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to delete establishment"})
	}

	return c.JSON(fiber.Map{"message": "Establishment deleted successfully"})
}
