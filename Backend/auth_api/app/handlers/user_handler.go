package handlers

import (
	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/carloshomar/vercardapio/auth_api/app/dto"
	"github.com/carloshomar/vercardapio/auth_api/app/middlewares"
	"github.com/carloshomar/vercardapio/auth_api/app/models"
)

func CreateUser(c *fiber.Ctx) error {
	var request dto.CreateUserRequest
	if err := c.BodyParser(&request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Failed to parse request body"})
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(request.Password), bcrypt.DefaultCost)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to hash password"})
	}

	user := models.User{
		Name:     request.Name,
		Email:    request.Email,
		Password: string(hashedPassword),
	}

	establishment := models.Establishment{
		Name:                request.Establishment.Name,
		Description:         request.Establishment.Description,
		OwnerID:             user.ID,
		Image:               request.Establishment.Image,
		PrimaryColor:        request.Establishment.PrimaryColor,
		SecondaryColor:      request.Establishment.SecondaryColor,
		Lat:                 request.Establishment.Lat,
		Long:                request.Establishment.Long,
		MaxDistanceDelivery: request.Establishment.MaxDistanceDelivery,
		LocationString:      request.Establishment.LocationString,
	}

	err = models.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&user).Error; err != nil {
			return err
		}
		establishment.OwnerID = user.ID
		if err := tx.Create(&establishment).Error; err != nil {
			return err
		}
		user.EstablishmentID = establishment.ID
		if err := tx.Save(&user).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	tokenString, err := middlewares.GenerateJWT(&user, &establishment)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to generate JWT token"})
	}

	request.Password = ""
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"user": request, "token": tokenString})
}

func Login(c *fiber.Ctx) error {
	var request dto.LoginRequest
	if err := c.BodyParser(&request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Failed to parse request body"})
	}

	var user models.User
	if err := models.DB.Where(&models.User{
		Email: request.Email,
	}).First(&user).Error; err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Incorrect credentials"})
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(request.Password)); err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Incorrect credentials"})
	}

	var establishment models.Establishment
	if err := models.DB.Where(&models.Establishment{
		ID: user.EstablishmentID,
	}).First(&establishment).Error; err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Incorrect credentials"})
	}

	tokenString, jwtError := middlewares.GenerateJWT(&user, &establishment)

	if jwtError != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Incorrect credentials"})
	}

	return c.JSON(fiber.Map{
		"token": tokenString,
	})
}

func GetUser(c *fiber.Ctx) error {

	userID := c.Params("id")
	var user models.User

	if err := models.DB.First(&user, userID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
	}

	return c.JSON(user)
}
