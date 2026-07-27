// Package handlers - user_handler.go
// Handlers HTTP para autenticacao e gestao de usuarios.
package handlers

import (
	"github.com/carloshomar/vercardapio/payment/models"
	"github.com/carloshomar/vercardapio/payment/services"
	"github.com/gofiber/fiber/v2"
)

// UserHandler e responsavel pelas rotas de usuarios.
type UserHandler struct {
	Service *services.UserService
}

// NewUserHandler cria uma nova instancia do handler.
func NewUserHandler() *UserHandler {
	return &UserHandler{
		Service: services.NewUserService(),
	}
}

// Login autentica um usuario e retorna um token JWT.
// POST /api/auth/login
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

// GetUser busca um usuario pelo ID.
// GET /api/users/:id
func (uh *UserHandler) GetUser(c *fiber.Ctx) error {
	id := c.Params("id")
	user, err := uh.Service.GetUser(id)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "User not found"})
	}
	return c.JSON(user)
}

// ListUsers retorna todos os usuarios.
// GET /api/users
func (uh *UserHandler) ListUsers(c *fiber.Ctx) error {
	users, err := uh.Service.ListUsers()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to list users"})
	}
	return c.JSON(users)
}

// CreateUser cria um novo usuario.
// POST /api/users
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

	if user.Email == "" || user.Password == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Email e senha obrigatorios"})
	}
	if len(user.Password) < 6 {
		return c.Status(400).JSON(fiber.Map{"error": "Senha deve ter pelo menos 6 caracteres"})
	}

	// Valida role contra valores permitidos
	validRoles := map[models.UserRole]bool{
		models.RoleAdmin:    true,
		models.RoleOperator: true,
		models.RoleViewer:   true,
	}
	role := models.UserRole(user.Role)
	if !validRoles[role] {
		return c.Status(400).JSON(fiber.Map{"error": "Role invalida. Use: admin, operator ou viewer"})
	}

	u := &models.User{
		Email:    user.Email,
		Name:     user.Name,
		Password: user.Password, // Hash via bcrypt no repository.CreateUser
		Role:     role,
	}

	if err := uh.Service.CreateUser(u); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to create user"})
	}

	return c.Status(201).JSON(fiber.Map{"message": "User created", "user": u})
}
