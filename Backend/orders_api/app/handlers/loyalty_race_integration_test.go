//go:build integration

// Pontos de fidelidade sob concorrência, contra Postgres de verdade (o SQLite
// dos testes unitários não tem FOR UPDATE).
package handlers

import (
	"fmt"
	"sync"
	"testing"

	"github.com/carloshomar/fuudelivery/orders_api/app/models"
)

func setupLoyaltyRacePG(t *testing.T) {
	t.Helper()
	setupCouponRacePG(t) // conecta, trata CI sem banco e troca models.DB
	if err := models.DB.Migrator().DropTable(&models.LoyaltyTransaction{}, &models.LoyaltyPoints{}); err != nil {
		t.Fatal(err)
	}
	if err := models.DB.AutoMigrate(&models.LoyaltyPoints{}, &models.LoyaltyTransaction{}); err != nil {
		t.Fatal(err)
	}
}

// Pedidos diferentes do mesmo cliente aprovados ao mesmo tempo: todo pedido
// credita, e a conta continua uma só.
func TestFidelidade_PedidosSimultaneosDoMesmoCliente(t *testing.T) {
	setupLoyaltyRacePG(t)
	const phone = "+5511911110000"
	const n = 20

	start := make(chan struct{})
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			errs <- EarnPointsForOrder(phone, fmt.Sprintf("ped-%d", i), 10)
		}(i)
	}
	close(start)
	wg.Wait()
	close(errs)
	falhas := 0
	for err := range errs {
		if err != nil {
			falhas++
		}
	}

	var contas []models.LoyaltyPoints
	models.DB.Where("user_phone = ?", phone).Find(&contas)
	if len(contas) != 1 {
		t.Fatalf("o cliente deveria ter 1 conta de pontos, tem %d", len(contas))
	}
	if falhas != 0 {
		t.Fatalf("%d de %d créditos falharam por concorrência", falhas, n)
	}
	// bronze = multiplicador 1: 10 pontos por pedido de R$10.
	if contas[0].Points != n*10 {
		t.Fatalf("esperava %d pontos, a conta tem %d", n*10, contas[0].Points)
	}
}

// O mesmo pedido creditado várias vezes em paralelo (webhook repetido +
// reconciliação) credita uma vez só.
func TestFidelidade_MesmoPedidoEmParalelo(t *testing.T) {
	setupLoyaltyRacePG(t)
	const phone = "+5511922220000"
	models.DB.Create(&models.LoyaltyPoints{UserPhone: phone, Tier: "bronze"})

	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_ = EarnPointsForOrder(phone, "ped-unico", 10)
		}()
	}
	close(start)
	wg.Wait()

	var conta models.LoyaltyPoints
	models.DB.Where("user_phone = ?", phone).First(&conta)
	if conta.Points != 10 {
		t.Fatalf("pedido creditado mais de uma vez: %d pontos (esperava 10)", conta.Points)
	}
}
