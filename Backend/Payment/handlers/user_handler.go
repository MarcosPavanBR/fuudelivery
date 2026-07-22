package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/carloshomar/vercardapio/payment/services"
)

type UserHandler struct {
	Service *services.UserService
}

func NewUserHandler() *UserHandler {
	return &UserHandler{
		Service: services.NewUserService(),
	}
}

func (uh *UserHandler) Login(c *fiber.Ctx) error {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	resp, err := uh.Service.Login(req.Email, req.Password)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{"error": "Invalid credentials"})
	}

	return c.JSON(resp)
}

func (uh *UserHandler) GetUser(c *fiber.Ctx) error {
	id := c.Params("id")
	user, err := uh.Service.GetUser(id)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "User not found"})
	}
	return c.JSON(user)
}

func (uh *UserHandler) ListUsers(c *fiber.Ctx) error {
	users, err := uh.Service.ListUsers()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to list users"})
	}
	return c.JSON(users)
}

func (uh *UserHandler) CreateUser(c *fiber.Ctx) error {
	var user struct {
		Email    string `json:"email"`
		Name     string `json:"name"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := c.BodyParser(&user); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	return c.JSON(fiber.Map{"message": "User created"})
}
