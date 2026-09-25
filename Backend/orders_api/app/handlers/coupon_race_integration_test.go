//go:build integration

// Limite de cupom por cliente sob pedidos simultâneos, contra Postgres de
// verdade — o SQLite dos testes unitários não tem FOR UPDATE.
package handlers

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/carloshomar/fuudelivery/orders_api/app/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupCouponRacePG(t *testing.T) {
	t.Helper()
	dsn := os.Getenv("POSTGRES_TEST_URI")
	if dsn == "" {
		if os.Getenv("CI") != "" {
			t.Fatal("POSTGRES_TEST_URI ausente em CI — o job precisa provê-la, senão este teste não roda")
		}
		t.Skip("POSTGRES_TEST_URI não definida (rode com um Postgres local)")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("conectar Postgres: %v", err)
	}
	if err := db.Migrator().DropTable(&models.CouponUsage{}, &models.Coupon{}); err != nil {
		t.Fatalf("limpar: %v", err)
	}
	if err := db.AutoMigrate(&models.Coupon{}, &models.CouponUsage{}); err != nil {
		t.Fatalf("migrar: %v", err)
	}
	if sqlDB, err := db.DB(); err == nil {
		sqlDB.SetMaxOpenConns(40)
	}
	prev := models.DB
	models.DB = db
	t.Cleanup(func() { models.DB = prev })
}

// pedidosSimultaneos dispara n applyCouponToOrder do mesmo telefone ao mesmo
// tempo e devolve quantos ganharam desconto.
func pedidosSimultaneos(t *testing.T, code, phone string, n int) int {
	t.Helper()
	start := make(chan struct{})
	var wg sync.WaitGroup
	var mu sync.Mutex
	ok := 0
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			if _, err := applyCouponToOrder(code, fmt.Sprintf("%s-ped-%d", code, i), phone, 0, 50, 5); err == nil {
				mu.Lock()
				ok++
				mu.Unlock()
			}
		}(i)
	}
	close(start)
	wg.Wait()
	return ok
}

// Antes da trava em consumeCoupon, 30 pedidos paralelos do mesmo telefone
// num cupom de 1 por cliente renderam até 15 descontos.
func TestCupomPorCliente_PedidosSimultaneos(t *testing.T) {
	setupCouponRacePG(t)
	for _, c := range []struct {
		code       string
		porCliente int
	}{{"PRIMEIRA", 1}, {"DUASVEZES", 2}} {
		if err := models.DB.Create(&models.Coupon{
			Code: c.code, DiscountType: "FIXED", DiscountValue: 10,
			MaxUses: 1000, MaxUsesPerUser: c.porCliente, IsActive: true,
			StartDate: time.Now().Add(-time.Hour), ExpiryDate: time.Now().Add(time.Hour),
		}).Error; err != nil {
			t.Fatal(err)
		}
		for rodada := 0; rodada < 3; rodada++ {
			phone := fmt.Sprintf("+55119%02d%d", rodada, c.porCliente)
			if got := pedidosSimultaneos(t, c.code, phone, 30); got != c.porCliente {
				t.Fatalf("%s rodada %d: %d descontos para o mesmo telefone, limite %d", c.code, rodada, got, c.porCliente)
			}
		}
	}
}
