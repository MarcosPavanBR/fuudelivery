package handlers

import (
	"log"
	"strings"
	"time"

	"github.com/carloshomar/fuudelivery/auth_api/app/middlewares"
	"github.com/carloshomar/fuudelivery/auth_api/app/models"
	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"
)

// RegisterEstablishment cadastra um restaurante e a conta do dono em uma
// única transação (POST /establishments/register — público, usado pelo
// WebRestaurant). Cria: usuário (role 'restaurant'), estabelecimento
// vinculado e horários de funcionamento (business_hours, 7 dias).
// Retorna token JWT para login imediato.
func RegisterEstablishment(c *fiber.Ctx) error {
	var req struct {
		Name        string `json:"name"`
		OwnerName   string `json:"owner_name"`
		Email       string `json:"email"`
		Password    string `json:"password"`
		Phone       string `json:"phone"`
		Address     string `json:"address"`
		City        string `json:"city"`
		OpeningTime string `json:"opening_time"`
		ClosingTime string `json:"closing_time"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Failed to parse request body"})
	}

	if req.Name == "" || req.OwnerName == "" || req.Email == "" || req.Password == "" || req.Phone == "" || req.Address == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "name, owner_name, email, password, phone e address sao obrigatorios"})
	}
	if len(req.Password) < 6 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Password must be at least 6 characters"})
	}

	var existing models.User
	if err := models.DB.Where("email = ?", req.Email).First(&existing).Error; err == nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "Email already registered"})
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to hash password"})
	}

	sqlDB, err := models.DB.DB()
	if err != nil || sqlDB == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Database not available"})
	}
	tx, err := sqlDB.Begin()
	if err != nil || tx == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to start transaction"})
	}

	var userID, estID uint

	// Role: 'restaurant' no enum de produção; "user" no banco novo
	// (schema_compat.go).
	roleVal := userRoleValue(tx)

	tx.Exec("CREATE SEQUENCE IF NOT EXISTS users_id_seq OWNED BY users.id")
	if err := tx.QueryRow("SELECT nextval('users_id_seq')").Scan(&userID); err != nil {
		tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if _, err := tx.Exec(insertUserSQL(tx, true),
		userID, req.OwnerName, req.Email, string(hashedPassword), roleVal, req.Phone); err != nil {
		tx.Rollback()
		if strings.Contains(err.Error(), "duplicate key value violates unique constraint") {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "Email already registered"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create account"})
	}

	locationString := req.Address
	if req.City != "" {
		locationString += ", " + req.City
	}

	horario := ""
	if req.OpeningTime != "" || req.ClosingTime != "" {
		horario = req.OpeningTime + " - " + req.ClosingTime
	}

	tx.Exec("CREATE SEQUENCE IF NOT EXISTS establishments_id_seq OWNED BY establishments.id")
	if err := tx.QueryRow("SELECT nextval('establishments_id_seq')").Scan(&estID); err != nil {
		tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if _, err := tx.Exec("INSERT INTO establishments (id, name, description, owner_id, lat, long, location_string, max_distance_delivery, horario_funcionamento) VALUES ($1, $2, '', $3, 0, 0, $4, 15, $5)",
		estID, req.Name, userID, locationString, horario); err != nil {
		tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if _, err := tx.Exec("UPDATE users SET establishment_id = $1 WHERE id = $2", estID, userID); err != nil {
		tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Horários de funcionamento (7 dias) a partir de abertura/fechamento.
	if req.OpeningTime != "" && req.ClosingTime != "" {
		for day := 0; day < 7; day++ {
			if _, err := tx.Exec("INSERT INTO business_hours (establishment_id, day_of_week, is_open, open_time, close_time, break_start_time, break_end_time) VALUES ($1, $2, true, $3, $4, '', '')",
				estID, day, req.OpeningTime, req.ClosingTime); err != nil {
				log.Printf("[REGISTER] business_hours day=%d: %v", day, err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	user := models.User{ID: userID, Name: req.OwnerName, Email: req.Email, EstablishmentID: estID, Role: roleVal}
	establishment := models.Establishment{ID: estID, Name: req.Name, OwnerID: userID, LocationString: locationString, HorarioFuncionamento: horario}

	tokenString, jwtError := middlewares.GenerateJWT(&user, &establishment)
	if jwtError != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to generate JWT token"})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message":       "Establishment registered successfully",
		"user":          fiber.Map{"id": userID, "name": req.OwnerName, "email": req.Email},
		"establishment": fiber.Map{"id": estID, "name": req.Name},
		"token":         tokenString,
	})
}

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
		ZoneID       *uint   `json:"zone_id,omitempty"`
		// Campos com o nome do modelo (painel admin).
		Description         string   `json:"description"`
		MaxDistanceDelivery *float64 `json:"max_distance_delivery"`
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
	if req.MaxDistanceDelivery != nil && *req.MaxDistanceDelivery > 0 {
		maxDist = *req.MaxDistanceDelivery
	}

	establishment := models.Establishment{
		Name:                req.Name,
		Description:         req.Description,
		Image:               "",
		PrimaryColor:        "#EA1D2C",
		SecondaryColor:      "#FFFFFF",
		Lat:                 req.Latitude,
		Long:                req.Longitude,
		LocationString:      locationString,
		MaxDistanceDelivery: maxDist,
		ZoneID:              req.ZoneID,
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
	if !validID(establishmentId) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "ID inválido"})
	}

	var establishment models.Establishment
	if err := models.DB.First(&establishment, "id = ?", establishmentId).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Establishment not found"})
	}
	return c.JSON(establishment)
}

// ListEstablishments é a vitrine do app do cliente: lojas com o botão
// "Aberto" ligado, cada uma com o status pela grade de horários, a nota
// média e o selo de patrocinado. Ordem em models.OrderListing.
func ListEstablishments(c *fiber.Ctx) error {
	var establishments []models.Establishment
	if err := models.DB.Where("open_data IS NOT NULL").Order("name asc").Find(&establishments).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to list establishments"})
	}
	return c.JSON(models.BuildListing(models.DB, establishments, time.Now()))
}

// canManageEstablishment decide se o usuário autenticado pode operar sobre o
// estabelecimento alvo. Admin gerencia qualquer um; os demais papéis apenas o
// próprio estabelecimento (establishment_id do token == id alvo). eErr vem de
// GetEstablishmentIDFromToken — usuário sem vínculo a estabelecimento não passa.
func canManageEstablishment(role string, tokenEstID int64, eErr error, targetID uint) bool {
	if role == "admin" {
		return true
	}
	return eErr == nil && tokenEstID == int64(targetID)
}

func GetUserByEstablishment(c *fiber.Ctx) error {
	establishmentId, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "id not found"})
	}

	// Ownership check: admin can access any, others only their own
	role, rErr := middlewares.GetUserRoleFromToken(c)
	if rErr != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Invalid token"})
	}
	tokenEstID, eErr := middlewares.GetEstablishmentIDFromToken(c)
	if !canManageEstablishment(role, tokenEstID, eErr, uint(establishmentId)) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Forbidden"})
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
	if !validID(establishmentID) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "ID inválido"})
	}

	var establishment models.Establishment
	if err := models.DB.First(&establishment, "id = ?", establishmentID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Establishment not found"})
	}

	// Ownership check: admin can toggle any, others only their own
	role, rErr := middlewares.GetUserRoleFromToken(c)
	if rErr != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Invalid token"})
	}
	tokenEstID, eErr := middlewares.GetEstablishmentIDFromToken(c)
	if !canManageEstablishment(role, tokenEstID, eErr, establishment.ID) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Forbidden"})
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
	if !validID(establishmentID) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "ID inválido"})
	}

	if establishmentID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid establishment ID"})
	}

	existingEstablishment := models.Establishment{}

	if err := models.DB.First(&existingEstablishment, "id = ?", establishmentID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Establishment not found"})
	}

	// IDOR protection: verify the authenticated user owns this establishment.
	// Only admins can edit establishments they don't own.
	tokenUserID, err := middlewares.GetUserIDFromToken(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid token"})
	}
	role, _ := middlewares.GetUserRoleFromToken(c)
	if role != "admin" {
		// O id do token só é de users se a conta for de users: sem isto, o
		// cliente 5 herdava a loja do usuário 5 e editava o estabelecimento.
		if t, _ := middlewares.GetAccountTypeFromToken(c); t != middlewares.AccountUser {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Cannot update another user's establishment"})
		}
		var authUser models.User
		if uErr := models.DB.First(&authUser, tokenUserID).Error; uErr != nil {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Cannot update another user's establishment"})
		}
		if authUser.EstablishmentID != existingEstablishment.ID {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Cannot update another user's establishment"})
		}
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

	// ZoneID: se o payload enviar explicitamente, atualiza
	if request.Establishment.ZoneID != nil {
		existingEstablishment.ZoneID = request.Establishment.ZoneID
	}

	if err := models.DB.Save(&existingEstablishment).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update establishment"})
	}

	return c.JSON(fiber.Map{"message": "Establishment updated successfully"})
}

func UpdateEstablishmentWallet(c *fiber.Ctx) error {
	establishmentID := c.Params("id")
	if !validID(establishmentID) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "ID inválido"})
	}
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
	if err := models.DB.First(&establishment, "id = ?", establishmentID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Establishment not found"})
	}

	// Ownership: a carteira de saque (Asaas) define PARA ONDE vai o dinheiro —
	// só o dono do estabelecimento ou um admin pode apontá-la. Sem este check,
	// qualquer autenticado redirecionava saques de qualquer restaurante.
	role, rErr := middlewares.GetUserRoleFromToken(c)
	if rErr != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid token"})
	}
	if role != "admin" {
		tokenEstID, eErr := middlewares.GetEstablishmentIDFromToken(c)
		if eErr != nil || tokenEstID != int64(establishment.ID) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Sem permissão para esta carteira"})
		}
	}

	establishment.PaymentWalletID = req.PaymentWalletID
	if err := models.DB.Save(&establishment).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update wallet"})
	}

	return c.JSON(fiber.Map{
		"message":           "Wallet ID updated successfully",
		"payment_wallet_id": establishment.PaymentWalletID,
	})
}

func DeleteEstablishment(c *fiber.Ctx) error {
	id := c.Params("id")
	if !validID(id) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "ID inválido"})
	}
	if id == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid establishment ID"})
	}

	var establishment models.Establishment
	if err := models.DB.First(&establishment, "id = ?", id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Establishment not found"})
	}

	if err := models.DB.Delete(&establishment).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to delete establishment"})
	}

	return c.JSON(fiber.Map{"message": "Establishment deleted successfully"})
}

// AdminEstablishment é a linha da tela de estabelecimentos do painel admin:
// o estabelecimento inteiro (inclusive os fechados, que GET /establishments —
// a vitrine — esconde) e o contato do responsável, que mora em users.
type AdminEstablishment struct {
	models.Establishment
	AcceptingOrders bool   `json:"accepting_orders"`
	OwnerName       string `json:"owner_name"`
	OwnerEmail      string `json:"owner_email"`
	OwnerPhone      string `json:"owner_phone"`
}

// ListEstablishmentsAdmin — GET /admin/establishments (adminRequired).
func ListEstablishmentsAdmin(c *fiber.Ctx) error {
	var ests []models.Establishment
	if err := models.DB.Order("id asc").Find(&ests).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to list establishments"})
	}

	type owner struct {
		EstablishmentID uint
		Name            string
		Email           string
		Phone           *string
	}
	var owners []owner
	// Um responsável por loja: o usuário mais antigo vinculado a ela.
	if err := models.DB.Raw(`SELECT DISTINCT ON (establishment_id) establishment_id, name, email, phone
		FROM users WHERE establishment_id > 0 ORDER BY establishment_id, id`).Scan(&owners).Error; err != nil {
		log.Printf("[ADMIN] responsáveis das lojas: %v", err)
	}
	byEst := make(map[uint]owner, len(owners))
	for _, o := range owners {
		byEst[o.EstablishmentID] = o
	}

	out := make([]AdminEstablishment, 0, len(ests))
	for _, e := range ests {
		row := AdminEstablishment{Establishment: e, AcceptingOrders: e.OpenData != nil}
		if o, ok := byEst[e.ID]; ok {
			row.OwnerName, row.OwnerEmail = o.Name, o.Email
			if o.Phone != nil {
				row.OwnerPhone = *o.Phone
			}
		}
		out = append(out, row)
	}
	return c.JSON(out)
}
