//go:build integration

package main

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// orderDocumentRow espelha apenas as colunas que a query SQL de
// GetMonthlyOrders precisa (tabela order_documents).
type orderDocumentRow struct {
	ID              int64 `gorm:"primaryKey"`
	LegacyID        string
	EstablishmentID int64
	Status          string
	Payload         []byte `gorm:"type:jsonb"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (orderDocumentRow) TableName() string { return "order_documents" }

// establishmentRow espelha a tabela establishments com o campo zone_id.
type establishmentRow struct {
	ID     uint `gorm:"primaryKey"`
	Name   string
	ZoneID uint
}

func (establishmentRow) TableName() string { return "establishments" }

// zoneRow espelha a tabela zones.
type zoneRow struct {
	ID   uint `gorm:"primaryKey"`
	Name string
}

func (zoneRow) TableName() string { return "zones" }

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	ctx := context.Background()

	pgDSN := os.Getenv("POSTGRES_TEST_URI")
	if pgDSN == "" {
		pgContainer, err := postgres.Run(ctx, "postgres:16-alpine",
			postgres.WithDatabase("fuudelivery_test_split"),
			postgres.WithUsername("test"),
			postgres.WithPassword("test"),
		)
		require.NoError(t, err, "subir Postgres")
		t.Cleanup(func() { _ = pgContainer.Terminate(ctx) })

		pgDSN, err = pgContainer.ConnectionString(ctx, "sslmode=disable")
		require.NoError(t, err)
	}

	var db *gorm.DB
	var err error
	for attempt := 0; attempt < 10; attempt++ {
		db, err = gorm.Open(postgres.Open(pgDSN), &gorm.Config{})
		if err == nil {
			var ping int
			if pingErr := db.Raw("SELECT 1").Scan(&ping).Error; pingErr == nil && ping == 1 {
				break
			}
			err = fmt.Errorf("postgres nao respondeu ao ping")
		}
		time.Sleep(2 * time.Second)
	}
	require.NoError(t, err, "conectar ao Postgres")

	// Criar tabelas necessárias
	require.NoError(t, db.AutoMigrate(&zoneRow{}, &establishmentRow{}, &orderDocumentRow{}))

	return db
}

func seedTestData(t *testing.T, db *gorm.DB) (zoneID uint) {
	t.Helper()

	// Criar zona
	zone := zoneRow{Name: "Centro"}
	require.NoError(t, db.Create(&zone).Error)

	// Criar estabelecimento vinculado à zona
	estab := establishmentRow{Name: "Restaurante A", ZoneID: zone.ID}
	require.NoError(t, db.Create(&estab).Error)

	now := time.Now()

	// 5 pedidos dentro dos últimos 30 dias (devem ser contados)
	for i := 0; i < 5; i++ {
		order := orderDocumentRow{
			LegacyID:        fmt.Sprintf("recent-%d", i),
			EstablishmentID: int64(estab.ID),
			Status:          "DELIVERED",
			CreatedAt:       now.AddDate(0, 0, -i*3), // 0, 3, 6, 9, 12 dias atrás
			UpdatedAt:       now.AddDate(0, 0, -i*3),
		}
		require.NoError(t, db.Create(&order).Error)
	}

	// 3 pedidos com mais de 30 dias (NÃO devem ser contados)
	for i := 0; i < 3; i++ {
		order := orderDocumentRow{
			LegacyID:        fmt.Sprintf("old-%d", i),
			EstablishmentID: int64(estab.ID),
			Status:          "DELIVERED",
			CreatedAt:       now.AddDate(0, 0, -(31 + i*10)), // 31, 41, 51 dias atrás
			UpdatedAt:       now.AddDate(0, 0, -(31 + i*10)),
		}
		require.NoError(t, db.Create(&order).Error)
	}

	// 2 pedidos recentes de outro estabelecimento na MESMA zona (devem ser contados)
	estab2 := establishmentRow{Name: "Restaurante B", ZoneID: zone.ID}
	require.NoError(t, db.Create(&estab2).Error)
	for i := 0; i < 2; i++ {
		order := orderDocumentRow{
			LegacyID:        fmt.Sprintf("other-recent-%d", i),
			EstablishmentID: int64(estab2.ID),
			Status:          "DELIVERED",
			CreatedAt:       now.AddDate(0, 0, -i*5),
			UpdatedAt:       now.AddDate(0, 0, -i*5),
		}
		require.NoError(t, db.Create(&order).Error)
	}

	return zone.ID
}

func TestGetMonthlyOrders_OnlyLast30Days(t *testing.T) {
	db := setupTestDB(t)
	zoneID := seedTestData(t, db)

	provider := &splitMetricsProvider{DB: db}

	count := provider.GetMonthlyOrders(zoneID)

	// 5 (Restaurante A) + 2 (Restaurante B) = 7 pedidos recentes
	// Os 3 pedidos antigos (>30 dias) NÃO devem ser contados
	require.Equal(t, 7, count,
		"GetMonthlyOrders deve contar apenas pedidos dos últimos 30 dias")
}

func TestGetMonthlyOrders_DifferentZones(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Zona A
	zoneA := zoneRow{Name: "Zona A"}
	require.NoError(t, db.Create(&zoneA).Error)
	estabA := establishmentRow{Name: "Estab A", ZoneID: zoneA.ID}
	require.NoError(t, db.Create(&estabA).Error)

	// Zona B
	zoneB := zoneRow{Name: "Zona B"}
	require.NoError(t, db.Create(&zoneB).Error)
	estabB := establishmentRow{Name: "Estab B", ZoneID: zoneB.ID}
	require.NoError(t, db.Create(&estabB).Error)

	now := time.Now()

	// 3 pedidos na Zona A
	for i := 0; i < 3; i++ {
		order := orderDocumentRow{
			LegacyID:        fmt.Sprintf("zone-a-%d", i),
			EstablishmentID: int64(estabA.ID),
			Status:          "DELIVERED",
			CreatedAt:       now.AddDate(0, 0, -i),
			UpdatedAt:       now.AddDate(0, 0, -i),
		}
		require.NoError(t, db.Create(&order).Error)
	}

	// 1 pedido na Zona B
	order := orderDocumentRow{
		LegacyID:        "zone-b-0",
		EstablishmentID: int64(estabB.ID),
		Status:          "DELIVERED",
		CreatedAt:       now.AddDate(0, 0, -1),
		UpdatedAt:       now.AddDate(0, 0, -1),
	}
	require.NoError(t, db.Create(&order).Error)

	_ = ctx

	provider := &splitMetricsProvider{DB: db}

	countA := provider.GetMonthlyOrders(zoneA.ID)
	countB := provider.GetMonthlyOrders(zoneB.ID)

	require.Equal(t, 3, countA, "Zona A deve ter 3 pedidos")
	require.Equal(t, 1, countB, "Zona B deve ter 1 pedido")
}

func TestGetMonthlyOrders_NilDB(t *testing.T) {
	provider := &splitMetricsProvider{DB: nil}
	count := provider.GetMonthlyOrders(1)
	require.Equal(t, 0, count, "DB nil deve retornar 0")
}

func TestGetMonthlyOrders_NoOrders(t *testing.T) {
	db := setupTestDB(t)

	zone := zoneRow{Name: "Zona Vazia"}
	require.NoError(t, db.Create(&zone).Error)

	provider := &splitMetricsProvider{DB: db}
	count := provider.GetMonthlyOrders(zone.ID)
	require.Equal(t, 0, count, "Zona sem pedidos deve retornar 0")
}
