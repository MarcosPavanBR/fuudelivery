package handlers

import (
	"fmt"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"

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

	sqlDB, _ := models.DB.DB()
	tx, _ := sqlDB.Begin()
	if tx != nil {
		var userID, estID uint
		tx.Exec("CREATE SEQUENCE IF NOT EXISTS users_id_seq OWNED BY users.id")
		err = tx.QueryRow("SELECT nextval('users_id_seq')").Scan(&userID)
		if err != nil {
			tx.Rollback()
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		var roleVal string
		tx.QueryRow("SELECT enumlabel FROM pg_enum WHERE enumtypid = '\"Role\"'::regtype LIMIT 1").Scan(&roleVal)
		if roleVal == "" {
			roleVal = "user"
		}
		_, err = tx.Exec("INSERT INTO users (id, name, email, password, role, \"createdAt\", \"updatedAt\") VALUES ($1, $2, $3, $4, $5, NOW(), NOW())", userID, user.Name, user.Email, user.Password, roleVal)
		if err != nil {
			tx.Rollback()
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		user.ID = userID
		establishment.OwnerID = userID
		tx.Exec("CREATE SEQUENCE IF NOT EXISTS establishments_id_seq OWNED BY establishments.id")
		err = tx.QueryRow("SELECT nextval('establishments_id_seq')").Scan(&estID)
		if err != nil {
			tx.Rollback()
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		_, err = tx.Exec("INSERT INTO establishments (id, name, description, owner_id, lat, long, location_string, max_distance_delivery) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)", estID, establishment.Name, establishment.Description, establishment.OwnerID, establishment.Lat, establishment.Long, establishment.LocationString, establishment.MaxDistanceDelivery)
		if err != nil {
			tx.Rollback()
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		establishment.ID = estID
		user.EstablishmentID = estID
		_, err = tx.Exec("UPDATE users SET establishment_id = $1 WHERE id = $2", estID, userID)
		if err != nil {
			tx.Rollback()
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		tx.Commit()
	}
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

func ListAllUsers(c *fiber.Ctx) error {
	var users []models.User
	if err := models.DB.Find(&users).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to query users"})
	}
	return c.JSON(users)
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

func ChangePassword(c *fiber.Ctx) error {
	userID := c.Params("id")

	tokenUserID, err := middlewares.GetUserIDFromToken(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid token"})
	}

	var reqUserID uint
	if _, scanErr := fmt.Sscanf(userID, "%d", &reqUserID); scanErr != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid user ID"})
	}

	if tokenUserID != int64(reqUserID) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Cannot change another user's password"})
	}

	var request dto.ChangePasswordRequest
	if err := c.BodyParser(&request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Failed to parse request body"})
	}

	if len(request.NewPassword) < 6 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "New password must be at least 6 characters"})
	}

	var user models.User
	if err := models.DB.First(&user, reqUserID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(request.CurrentPassword)); err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Current password is incorrect"})
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(request.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to hash password"})
	}

	if err := models.DB.Model(&user).Update("password", string(hashedPassword)).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update password"})
	}

	return c.JSON(fiber.Map{"message": "Password updated successfully"})
}
