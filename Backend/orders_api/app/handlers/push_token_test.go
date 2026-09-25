package handlers

import (
	"testing"
	"time"

	"github.com/carloshomar/fuudelivery/orders_api/app/models"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func pushJWT(t *testing.T, claims jwt.MapClaims) string {
	t.Helper()
	claims["exp"] = time.Now().Add(time.Hour).Unix()
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(couponAuthzSecret))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// O dono do push token vem do JWT, nunca do corpo. Antes, um usuário
// logado substituía o token da vítima e recebia as notificações dela.
func TestRegisterPushToken_IdentidadeDoToken(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.PushToken{}); err != nil {
		t.Fatal(err)
	}
	prev := models.DB
	models.DB = db
	t.Cleanup(func() { models.DB = prev })
	t.Setenv("JWT_SECRET", couponAuthzSecret)

	// Vítima: cliente 7, com token de push registrado.
	db.Create(&models.PushToken{UserID: 7, UserType: "client", PushToken: "ExponentPushToken[vitima]"})

	app := fiber.New()
	app.Post("/notifications/register", RegisterPushToken)

	atacante := pushJWT(t, jwt.MapClaims{"id": 5, "role": "client"})
	body := `{"user_id":7,"user_type":"client","push_token":"ExponentPushToken[atacante]"}`
	if got := doIDOR(t, app, "/notifications/register", atacante, body); got != 200 {
		t.Fatalf("got %d", got)
	}

	var vitima models.PushToken
	db.Where("user_id = 7 AND user_type = 'client'").First(&vitima)
	if vitima.PushToken != "ExponentPushToken[vitima]" {
		t.Fatalf("token da vítima foi sobrescrito: %q", vitima.PushToken)
	}
	var proprio models.PushToken
	if err := db.Where("user_id = 5 AND user_type = 'client'").First(&proprio).Error; err != nil {
		t.Fatalf("o token do atacante deveria ficar na conta dele (5): %v", err)
	}

	// AppComida manda user_type "customer"; o leitor consulta "client".
	// O tipo vem do papel do token.
	cliente := pushJWT(t, jwt.MapClaims{"id": 8, "role": "client"})
	doIDOR(t, app, "/notifications/register", cliente, `{"user_id":8,"user_type":"customer","push_token":"ExponentPushToken[c8]"}`)
	var c8 models.PushToken
	if err := db.Where("user_id = 8 AND user_type = 'client'").First(&c8).Error; err != nil {
		t.Fatalf("cliente deveria ser registrado como 'client': %v", err)
	}

	loja := pushJWT(t, jwt.MapClaims{"id": 3, "role": "user", "establishment_id": 42})
	doIDOR(t, app, "/notifications/register", loja, `{"user_id":3,"user_type":"restaurant","push_token":"ExponentPushToken[l3]"}`)
	var l3 models.PushToken
	if err := db.Where("user_id = 3 AND user_type = 'restaurant'").First(&l3).Error; err != nil {
		t.Fatalf("usuário de loja deveria ser 'restaurant': %v", err)
	}

	if got := doIDOR(t, app, "/notifications/register", "", body); got != 401 {
		t.Errorf("sem token: got %d, want 401", got)
	}
}
