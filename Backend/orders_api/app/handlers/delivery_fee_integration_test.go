//go:build integration

// Frete grátis por assinatura, contra Postgres de verdade.
//
// Por que não no harness sqlite dos outros testes de frete: a query de
// subscriptions usa cast `::text` (sintaxe Postgres), que o sqlite rejeita com
// "unrecognized token: :". Lá o Scan erraria, computeDeliveryFee cairia no
// retorno sem desconto, e uma asserção de "cobrou frete" passaria pelo motivo
// errado — verde sem testar nada.
//
// Como rodar:
//
//	POSTGRES_TEST_URI=... go test -tags=integration -run 'TestDeliveryFeeAssinatura' ./app/handlers/
package handlers

import (
	"os"
	"testing"

	"github.com/carloshomar/fuudelivery/orders_api/app/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func setupDeliveryFeePostgres(t *testing.T) {
	t.Helper()
	dsn := os.Getenv("POSTGRES_TEST_URI")
	if dsn == "" {
		// Skip serve para rodar `go test -tags=integration` na máquina de quem
		// não subiu Postgres. Em CI ele seria um falso verde: o job passaria
		// sem ter executado nada, que é justamente o buraco que este job veio
		// fechar. Lá, a ausência da variável é erro de configuração do
		// workflow e tem que falhar alto.
		if os.Getenv("CI") != "" {
			t.Fatal("POSTGRES_TEST_URI ausente em CI — o job precisa provê-la, senão este teste não roda")
		}
		t.Skip("POSTGRES_TEST_URI não definida (rode com um Postgres local)")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("conectar Postgres: %v", err)
	}
	if err := db.Exec(`DROP TABLE IF EXISTS subscriptions CASCADE`).Error; err != nil {
		t.Fatalf("limpar subscriptions: %v", err)
	}
	if err := db.Exec(`CREATE TABLE subscriptions (
		user_id BIGINT, status TEXT, plan TEXT,
		free_delivery_above DOUBLE PRECISION,
		current_period_start TIMESTAMPTZ, current_period_end TIMESTAMPTZ
	)`).Error; err != nil {
		t.Fatalf("criar subscriptions: %v", err)
	}
	if err := db.Exec(`DROP TABLE IF EXISTS deliveries CASCADE`).Error; err != nil {
		t.Fatalf("limpar deliveries: %v", err)
	}
	if err := db.AutoMigrate(&models.Delivery{}); err != nil {
		t.Fatalf("migrar deliveries: %v", err)
	}
	if err := db.Create(&models.Delivery{EstablishmentID: 1, FixedTaxa: 5.00, PerKm: 2.00}).Error; err != nil {
		t.Fatalf("semear delivery: %v", err)
	}

	prev := models.DB
	models.DB = db
	t.Cleanup(func() { models.DB = prev })
}

func TestDeliveryFeeAssinatura(t *testing.T) {
	setupDeliveryFeePostgres(t)

	// Vigência aberta em ambos os lados para isolar a regra do plano.
	seedSub := func(userID int, plan string, above float64) {
		t.Helper()
		if err := models.DB.Exec(`INSERT INTO subscriptions
			(user_id, status, plan, free_delivery_above, current_period_start, current_period_end)
			VALUES (?, 'active', ?, ?, now() - interval '1 day', now() + interval '30 days')`,
			userID, plan, above).Error; err != nil {
			t.Fatalf("semear assinatura: %v", err)
		}
	}

	t.Run("premium tem frete grátis sempre", func(t *testing.T) {
		seedSub(701, "premium", 0)
		uid := uint(701)
		fee, err := computeDeliveryFee(3, 1, &uid, 0)
		if err != nil {
			t.Fatalf("erro inesperado: %v", err)
		}
		if fee.Value != 0 {
			t.Fatalf("premium deveria zerar o frete, veio %.2f", fee.Value)
		}
		if fee.BaseValue != 11.00 {
			t.Errorf("o valor base (3×2+5) deve ser preservado para exibição, veio %.2f", fee.BaseValue)
		}
		if !fee.SubscriptionDiscount {
			t.Error("o desconto deveria estar sinalizado")
		}
	})

	t.Run("basic só acima do mínimo", func(t *testing.T) {
		seedSub(702, "basic", 50.0)
		uid := uint(702)

		abaixo, err := computeDeliveryFee(3, 1, &uid, 49.99)
		if err != nil {
			t.Fatalf("erro inesperado: %v", err)
		}
		if abaixo.Value != 11.00 {
			t.Fatalf("abaixo do mínimo o frete é cobrado, veio %.2f", abaixo.Value)
		}

		acima, err := computeDeliveryFee(3, 1, &uid, 50.00)
		if err != nil {
			t.Fatalf("erro inesperado: %v", err)
		}
		if acima.Value != 0 {
			t.Fatalf("no mínimo exato o frete é grátis, veio %.2f", acima.Value)
		}
	})

	t.Run("assinatura fora da vigência não vale", func(t *testing.T) {
		if err := models.DB.Exec(`INSERT INTO subscriptions
			(user_id, status, plan, free_delivery_above, current_period_start, current_period_end)
			VALUES (703, 'active', 'premium', 0, now() - interval '60 days', now() - interval '30 days')`).Error; err != nil {
			t.Fatalf("semear assinatura vencida: %v", err)
		}
		uid := uint(703)
		fee, err := computeDeliveryFee(3, 1, &uid, 0)
		if err != nil {
			t.Fatalf("erro inesperado: %v", err)
		}
		if fee.Value != 11.00 {
			t.Fatalf("assinatura vencida não dá frete grátis, veio %.2f", fee.Value)
		}
	})

	t.Run("usuário sem assinatura paga o frete", func(t *testing.T) {
		uid := uint(999)
		fee, err := computeDeliveryFee(3, 1, &uid, 1000.00)
		if err != nil {
			t.Fatalf("erro inesperado: %v", err)
		}
		if fee.Value != 11.00 {
			t.Fatalf("sem assinatura o frete é cobrado, veio %.2f", fee.Value)
		}
	})
}
