package handlers

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/carloshomar/fuudelivery/auth_api/app/middlewares"
	"github.com/carloshomar/fuudelivery/orders_api/app/models"
	"github.com/gofiber/fiber/v2"
)

type PushTicket struct {
	Status string `json:"status"`
	ID     string `json:"id,omitempty"`
}

type RegisterPushTokenRequest struct {
	// UserID e UserType do corpo são IGNORADOS (mantidos só para o JSON dos
	// apps continuar válido): a identidade vem do token. Ver pushIdentity.
	UserID    int64  `json:"user_id"`
	UserType  string `json:"user_type"`
	PushToken string `json:"push_token"`
}

// pushIdentity devolve de quem é o push token, a partir do JWT. O user_type
// segue o que os leitores consultam: "client" (sendStatusPushNotification),
// "restaurant" e "deliveryman".
//
// Antes o corpo mandava: qualquer usuário logado substituía o token de push
// de outro (upsert por user_id+user_type) e passava a receber as
// notificações de pedido dele. E o AppComida mandava "customer", que nenhum
// leitor consulta — o cliente nunca recebia push de status.
func pushIdentity(c *fiber.Ctx) (int64, string, bool) {
	userID, err := middlewares.GetUserIDFromToken(c)
	if err != nil || userID <= 0 {
		return 0, "", false
	}
	// O tipo vem de AccountTypeFromClaims: clientes, lojas e entregadores
	// têm ids de tabelas diferentes, e o par (user_id, user_type) é o que
	// separa o token de push do cliente 5 do entregador 5. Pelo role, um
	// usuário de loja com role vazio virava "deliveryman".
	accountType, aErr := middlewares.GetAccountTypeFromToken(c)
	if aErr != nil {
		return 0, "", false
	}
	switch accountType {
	case middlewares.AccountClient:
		return userID, "client", true
	case middlewares.AccountDeliveryMan:
		return userID, "deliveryman", true
	}
	if estID, eErr := middlewares.GetEstablishmentIDFromToken(c); eErr == nil && estID > 0 {
		return userID, "restaurant", true
	}
	role, _ := middlewares.GetUserRoleFromToken(c)
	if role == "" {
		role = "user"
	}
	return userID, role, true
}

func RegisterPushToken(c *fiber.Ctx) error {
	var req RegisterPushTokenRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if req.PushToken == "" {
		return c.Status(400).JSON(fiber.Map{"error": "push_token is required"})
	}

	userID, userType, ok := pushIdentity(c)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid token"})
	}
	req.UserID, req.UserType = userID, userType

	db := models.DB
	if db == nil {
		return c.Status(500).JSON(fiber.Map{"error": "Database not available"})
	}

	pushToken := models.PushToken{
		UserID:    req.UserID,
		UserType:  req.UserType,
		PushToken: req.PushToken,
	}
	if err := db.Where(models.PushToken{UserID: req.UserID, UserType: req.UserType}).
		Assign(models.PushToken{PushToken: req.PushToken, UpdatedAt: time.Now()}).
		FirstOrCreate(&pushToken).Error; err != nil {
		log.Printf("[PUSH_TOKEN] Falha ao salvar token no Postgres (user=%d): %v", req.UserID, err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to register token"})
	}

	return c.JSON(fiber.Map{"message": "Token registered"})
}

// pushHTTPClient tem timeout: o http.Post padrão não tem, e um Expo lento
// deixava a goroutine do push presa indefinidamente.
var pushHTTPClient = &http.Client{Timeout: 10 * time.Second}

func SendPushNotification(userID int64, userType, title, body string, data map[string]interface{}) error {
	db := models.DB
	if db == nil {
		return nil
	}

	var pushToken models.PushToken
	if err := db.Where("user_id = ? AND user_type = ?", userID, userType).First(&pushToken).Error; err != nil {
		return nil // token not found, silently skip
	}

	payload := map[string]interface{}{
		"to":    pushToken.PushToken,
		"title": title,
		"body":  body,
		"data":  data,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := pushHTTPClient.Post("https://exp.host/--/api/v2/push/send", "application/json", bytes.NewReader(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}
