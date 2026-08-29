// Package handlers implementa os handlers HTTP para autenticacao e gerenciamento de usuarios.
package handlers

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/carloshomar/fuudelivery/auth_api/app/dto"
	"github.com/carloshomar/fuudelivery/auth_api/app/middlewares"
	"github.com/carloshomar/fuudelivery/auth_api/app/models"
)

// createTokenPair gera access token + refresh token para um usuario.
// Usado por Login e CreateUser para manter a resposta consistente.
func createTokenPair(user *models.User, establishment *models.Establishment) (string, string, error) {
	accessToken, err := middlewares.GenerateJWT(user, establishment)
	if err != nil {
		return "", "", err
	}
	refreshToken, err := middlewares.CreateRefreshToken(user.ID)
	if err != nil {
		return "", "", err
	}
	return accessToken, refreshToken, nil
}

// CreateUser cadastra um novo usuario e seu estabelecimento associado.
// Requer: name, email, password, establishment.name.
// Cria usuario e estabelecimento em transacao atomica no PostgreSQL.
// Retorna o usuario criado e um token JWT valido por 7 dias.
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
		Phone:    request.Phone,
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

	sqlDB, err := models.DB.DB()
	if err != nil || sqlDB == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Database not available"})
	}
	tx, err := sqlDB.Begin()
	if err != nil || tx == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to start transaction"})
	}
	if tx != nil {
		var userID, estID uint
		tx.Exec("CREATE SEQUENCE IF NOT EXISTS users_id_seq OWNED BY users.id")
		err = tx.QueryRow("SELECT nextval('users_id_seq')").Scan(&userID)
		if err != nil {
			tx.Rollback()
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create user"})
		}
		var roleVal string
		tx.QueryRow("SELECT enumlabel FROM pg_enum WHERE enumtypid = '\"Role\"'::regtype LIMIT 1").Scan(&roleVal)
		if roleVal == "" {
			roleVal = "user"
		}
		_, err = tx.Exec("INSERT INTO users (id, name, email, password, phone, role, \"createdAt\", \"updatedAt\") VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())", userID, user.Name, user.Email, user.Password, user.Phone, roleVal)
		if err != nil {
			tx.Rollback()
			if strings.Contains(err.Error(), "duplicate key value violates unique constraint") && strings.Contains(err.Error(), "users_email_key") {
				return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "Email already registered"})
			}
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create user"})
		}
		user.ID = userID
		establishment.OwnerID = userID
		tx.Exec("CREATE SEQUENCE IF NOT EXISTS establishments_id_seq OWNED BY establishments.id")
		err = tx.QueryRow("SELECT nextval('establishments_id_seq')").Scan(&estID)
		if err != nil {
			tx.Rollback()
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create establishment"})
		}
		_, err = tx.Exec("INSERT INTO establishments (id, name, description, owner_id, lat, long, location_string, max_distance_delivery) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)", estID, establishment.Name, establishment.Description, establishment.OwnerID, establishment.Lat, establishment.Long, establishment.LocationString, establishment.MaxDistanceDelivery)
		if err != nil {
			tx.Rollback()
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create establishment"})
		}
		establishment.ID = estID
		user.EstablishmentID = estID
		_, err = tx.Exec("UPDATE users SET establishment_id = $1 WHERE id = $2", estID, userID)
		if err != nil {
			tx.Rollback()
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update user"})
		}
		tx.Commit()
	}
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	accessToken, refreshToken, err := createTokenPair(&user, &establishment)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to generate tokens"})
	}

	request.Password = ""
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"user": request, "token": accessToken, "refresh_token": refreshToken})
}

// CreateUserAdmin cria um usuario diretamente pelo painel admin (POST /users).
// Diferente do registro publico (/users/register), nao cria estabelecimento:
// aceita name, email, password, role, status e establishment_id opcional.
func CreateUserAdmin(c *fiber.Ctx) error {
	var request struct {
		Name            string `json:"name"`
		Email           string `json:"email"`
		Password        string `json:"password"`
		Role            string `json:"role"`
		Status          string `json:"status"`
		EstablishmentID uint   `json:"establishment_id"`
	}
	if err := c.BodyParser(&request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Failed to parse request body"})
	}
	if request.Name == "" || request.Email == "" || request.Password == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "name, email e password sao obrigatorios"})
	}
	if len(request.Password) < 8 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Password must be at least 8 characters"})
	}
	if request.Role == "" {
		request.Role = "client"
	}
	if request.Status == "" {
		request.Status = "active"
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(request.Password), bcrypt.DefaultCost)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to hash password"})
	}

	var userID uint
	if err := models.DB.Exec("CREATE SEQUENCE IF NOT EXISTS users_id_seq OWNED BY users.id").Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if err := models.DB.Raw("SELECT nextval('users_id_seq')").Scan(&userID).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	user := models.User{
		ID:              userID,
		Name:            request.Name,
		Email:           request.Email,
		Password:        string(hashedPassword),
		Role:            request.Role,
		Status:          request.Status,
		EstablishmentID: request.EstablishmentID,
	}
	if err := models.DB.Create(&user).Error; err != nil {
		if strings.Contains(err.Error(), "duplicate key value violates unique constraint") {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "Email already registered"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create user"})
	}

	request.Password = ""
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"user": request, "id": user.ID})
}

func ListAllUsers(c *fiber.Ctx) error {
	var results []map[string]interface{}
	result := models.DB.Raw("SELECT id, name, email, establishment_id, COALESCE(role, 'user') as role, COALESCE(status, 'active') as status, \"createdAt\" FROM users").Scan(&results)
	if result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to query users: " + result.Error.Error()})
	}
	return c.JSON(results)
}

// Login autentica um usuario existente.
// Valida email e senha (bcrypt), busca o estabelecimento associado,
// e retorna um token JWT valido por 7 dias.
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

	// Usuarios sem estabelecimento (ex.: admin) nao devem ser vinculados a
	// nenhum. NAO usar Where(&models.Establishment{ID: user.EstablishmentID}):
	// condicao baseada em struct do GORM ignora campos com valor zero, entao
	// ID: 0 vira uma query sem filtro (SELECT * FROM establishments LIMIT 1)
	// e retorna o primeiro estabelecimento da tabela em vez de "nenhum".
	var establishmentPtr *models.Establishment
	if user.EstablishmentID != 0 {
		var establishment models.Establishment
		if err := models.DB.Where("id = ?", user.EstablishmentID).First(&establishment).Error; err != nil {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Incorrect credentials"})
		}
		establishmentPtr = &establishment
	}

	accessToken, refreshToken, jwtError := createTokenPair(&user, establishmentPtr)

	if jwtError != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Incorrect credentials"})
	}

	return c.JSON(fiber.Map{
		"token":         accessToken,
		"refresh_token": refreshToken,
	})
}

func GetUser(c *fiber.Ctx) error {
	userID := c.Params("id")

	// Ownership: o usuario so pode ver seu proprio perfil, ou admin ve qualquer um.
	tokenUserID, err := middlewares.GetUserIDFromToken(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid token"})
	}
	role, _ := middlewares.GetUserRoleFromToken(c)
	if role != "admin" && fmt.Sprintf("%d", tokenUserID) != userID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Cannot view another user's profile"})
	}

	var user models.User
	if err := models.DB.First(&user, userID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
	}

	return c.JSON(user)
}

// UpdateUser atualiza os dados de um usuario (PUT /users/:id).
// Admin pode editar qualquer usuario (nome, email, role, establishment_id e
// senha opcional); o proprio usuario pode editar apenas o proprio perfil
// (nome/email — a senha passa por ChangePassword).
func UpdateUser(c *fiber.Ctx) error {
	userID := c.Params("id")

	tokenUserID, err := middlewares.GetUserIDFromToken(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid token"})
	}

	var reqUserID uint
	if _, scanErr := fmt.Sscanf(userID, "%d", &reqUserID); scanErr != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid user ID"})
	}

	var request struct {
		Name            string `json:"name"`
		Email           string `json:"email"`
		Phone           string `json:"phone"`
		AvatarURL       string `json:"avatar_url"`
		Role            string `json:"role"`
		Status          string `json:"status"`
		EstablishmentID uint   `json:"establishment_id"`
		Password        string `json:"password"`
	}
	if err := c.BodyParser(&request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Failed to parse request body"})
	}

	var user models.User
	if err := models.DB.First(&user, reqUserID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
	}

	role, _ := middlewares.GetUserRoleFromToken(c)
	isAdmin := role == "admin"
	if tokenUserID != int64(reqUserID) && !isAdmin {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Cannot update another user's account"})
	}

	updates := map[string]interface{}{}
	if request.Name != "" {
		updates["name"] = request.Name
	}
	if request.Email != "" {
		updates["email"] = request.Email
	}
	if request.Phone != "" {
		updates["phone"] = request.Phone
	}
	if request.AvatarURL != "" {
		updates["avatar_url"] = request.AvatarURL
	}
	// Somente admin altera role, status e vinculo de estabelecimento.
	if isAdmin {
		if request.Role != "" {
			updates["role"] = request.Role
		}
		if request.Status != "" {
			updates["status"] = request.Status
		}
		if request.EstablishmentID != 0 {
			updates["establishment_id"] = request.EstablishmentID
		}
	}
	if request.Password != "" {
		if len(request.Password) < 8 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Password must be at least 8 characters"})
		}
		hashedPassword, hashErr := bcrypt.GenerateFromPassword([]byte(request.Password), bcrypt.DefaultCost)
		if hashErr != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to hash password"})
		}
		updates["password"] = string(hashedPassword)
	}

	if len(updates) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "No fields to update"})
	}

	if err := models.DB.Model(&user).Updates(updates).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update user"})
	}

	return c.JSON(fiber.Map{"message": "User updated successfully", "id": user.ID})
}

// ChangePassword altera a senha de um usuario.
// Verifica que o usuario autenticado e o mesmo da requisicao.
// Requer a senha atual (para confirmar identidade) e a nova senha (minimo 8 caracteres).
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

	if len(request.NewPassword) < 8 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "New password must be at least 8 characters"})
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

// DeleteUser remove uma conta de usuario.
// Apenas o proprio usuario ou um admin podem deletar a conta.
func DeleteUser(c *fiber.Ctx) error {
	userID := c.Params("id")

	tokenUserID, err := middlewares.GetUserIDFromToken(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid token"})
	}

	var reqUserID uint
	if _, scanErr := fmt.Sscanf(userID, "%d", &reqUserID); scanErr != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid user ID"})
	}

	role, _ := middlewares.GetUserRoleFromToken(c)
	if tokenUserID != int64(reqUserID) && role != "admin" {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Cannot delete another user's account"})
	}

	var user models.User
	if err := models.DB.First(&user, reqUserID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
	}

	if err := models.DB.Delete(&user).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to delete user"})
	}

	return c.JSON(fiber.Map{"message": "Account deleted successfully"})
}

// BootstrapAdmin cria o primeiro admin (instalação nova) OU promove um usuário
// existente para 'admin'. Requer ADMIN_BOOTSTRAP_SECRET no ambiente.
// Uso único no setup inicial; remover a env após confirmar.
func BootstrapAdmin(c *fiber.Ctx) error {
	bootstrapSecret := os.Getenv("ADMIN_BOOTSTRAP_SECRET")
	if bootstrapSecret == "" {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Bootstrap not configured"})
	}

	var req struct {
		Email    string `json:"email"`
		Secret   string `json:"secret"`
		Password string `json:"password"`
		Name     string `json:"name"`
		Phone    string `json:"phone"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request"})
	}
	if req.Email == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "email é obrigatório"})
	}
	if req.Secret != bootstrapSecret {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Invalid secret"})
	}

	var user models.User
	err := models.DB.Where("email = ?", req.Email).First(&user).Error
	switch {
	case err == nil:
		// Usuário existe — promove a admin.
		if uErr := models.DB.Model(&user).Update("role", "admin").Error; uErr != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to promote user"})
		}
		return c.JSON(fiber.Map{"message": fmt.Sprintf("User %s promoted to admin", req.Email)})

	case errors.Is(err, gorm.ErrRecordNotFound):
		// Instalação nova: cria o primeiro admin com os dados informados.
		if req.Password == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "password é obrigatória para criar o primeiro admin"})
		}
		hash, hErr := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if hErr != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to hash password"})
		}
		user = models.User{
			Name:     req.Name,
			Email:    req.Email,
			Phone:    req.Phone,
			Password: string(hash),
			Role:     "admin",
		}
		if cErr := models.DB.Create(&user).Error; cErr != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create admin"})
		}
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": fmt.Sprintf("Admin %s created", req.Email)})

	default:
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to look up user"})
	}
}

// RefreshToken renova um access token usando um refresh token valido.
// Rotação ATÔMICA com reuse detection: o token antigo só pode ser consumido
// uma vez; replay pós-rotação revoga toda a família do usuário.
func RefreshToken(c *fiber.Ctx) error {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if req.RefreshToken == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "refresh_token is required"})
	}

	userID, newRefreshToken, rErr := middlewares.RotateRefreshToken(req.RefreshToken)
	if errors.Is(rErr, middlewares.ErrRefreshReuse) {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Sessão encerrada por segurança. Faça login novamente.",
		})
	}
	if rErr != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid or expired refresh token"})
	}

	// Busca o usuário
	var user models.User
	if err := models.DB.First(&user, userID).Error; err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "User not found"})
	}

	// Busca o estabelecimento (se houver)
	var establishmentPtr *models.Establishment
	if user.EstablishmentID != 0 {
		var establishment models.Establishment
		if err := models.DB.Where("id = ?", user.EstablishmentID).First(&establishment).Error; err == nil {
			establishmentPtr = &establishment
		}
	}

	accessToken, err := middlewares.GenerateJWT(&user, establishmentPtr)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to generate tokens"})
	}

	return c.JSON(fiber.Map{
		"token":         accessToken,
		"refresh_token": newRefreshToken,
	})
}

// Logout revoga o refresh token do usuario, encerrando a sessao.
func Logout(c *fiber.Ctx) error {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if req.RefreshToken != "" {
		if err := middlewares.RevokeRefreshToken(req.RefreshToken); err != nil {
			log.Printf("[WARN] Failed to revoke refresh token on logout: %v", err)
		}
	}

	return c.JSON(fiber.Map{"message": "Logged out successfully"})
}
