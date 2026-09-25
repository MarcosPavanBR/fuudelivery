package handlers

import (
	"os"
	"testing"

	"github.com/carloshomar/fuudelivery/auth_api/app/models"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Renovar sem cobrança estendia o frete grátis a cada chamada: só admin.
func TestRenewSubscription_SoAdmin(t *testing.T) {
	t.Setenv("JWT_SECRET", hoursTestSecret)
	cliente := hoursToken(t, jwt.MapClaims{"id": 9, "role": "client"})
	if got := postHours(t, "/subscriptions/renew", cliente, `{"user_id":9}`, RenewSubscription); got != 403 {
		t.Errorf("cliente renovando: got %d, want 403", got)
	}
}

// Assinatura criada pelo usuário nasce pendente (sem frete grátis) e só vira
// ativa quando o admin confirma o pagamento via renew.
func TestCreateSubscription_NasceSemBeneficio(t *testing.T) {
	uri := os.Getenv("POSTGRES_TEST_URI")
	if uri == "" {
		t.Skip("POSTGRES_TEST_URI não definido")
	}
	db, err := gorm.Open(postgres.Open(uri), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	_ = db.Migrator().DropTable(&models.Subscription{})
	if err := db.AutoMigrate(&models.Subscription{}); err != nil {
		t.Fatal(err)
	}
	prev := models.DB
	models.DB = db
	defer func() { models.DB = prev }()
	t.Setenv("JWT_SECRET", hoursTestSecret)

	cliente := hoursToken(t, jwt.MapClaims{"id": 9, "role": "client"})
	admin := hoursToken(t, jwt.MapClaims{"id": 1, "role": "admin"})
	status := func() string {
		var s models.Subscription
		if err := db.Where("user_id = 9").First(&s).Error; err != nil {
			t.Fatal(err)
		}
		return s.Status
	}

	if got := postHours(t, "/subscriptions", cliente, `{"plan":"premium"}`, CreateSubscription); got != 201 {
		t.Fatalf("criar: got %d", got)
	}
	if st := status(); st != models.SubscriptionPending {
		t.Fatalf("assinatura recém-criada deveria ser pending, é %q", st)
	}
	// Segunda criação enquanto pendente não duplica.
	if got := postHours(t, "/subscriptions", cliente, `{"plan":"premium"}`, CreateSubscription); got != 409 {
		t.Errorf("segunda criação: got %d, want 409", got)
	}
	if got := postHours(t, "/subscriptions/renew", admin, `{"user_id":9}`, RenewSubscription); got != 200 {
		t.Fatalf("admin ativando: got %d", got)
	}
	if st := status(); st != models.SubscriptionActive {
		t.Fatalf("após confirmação do admin deveria ser active, é %q", st)
	}
}
