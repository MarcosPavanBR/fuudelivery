package models

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var (
	DB *gorm.DB
)

const maxRetries = 5
const retryInterval = 5 * time.Second

func ConnectPostgresDatabase() {
	dsn := os.Getenv("DB_CONNECTION_STRING")
	if dsn == "" {
		panic("DB_CONNECTION_STRING não configurado")
	}

	var database *gorm.DB
	var err error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		pgDSN := dsn
		if !strings.Contains(pgDSN, "default_query_exec_mode") {
			if strings.Contains(pgDSN, "?") {
				pgDSN += "&default_query_exec_mode=simple_protocol"
			} else {
				pgDSN += "?default_query_exec_mode=simple_protocol"
			}
		}
		database, err = gorm.Open(postgres.Open(pgDSN), &gorm.Config{PrepareStmt: false})
		if err == nil {
			break
		}

		time.Sleep(retryInterval)
	}

	if err != nil {
		panic(fmt.Sprintf("Falha ao conectar ao banco de dados PostgreSQL após %d tentativas", maxRetries))
	}

	// Limites de pool (ver database.go do auth_api): 5 módulos compartilham
	// o mesmo Supabase — cada pool precisa de teto.
	if sqlDB, sErr := database.DB(); sErr == nil {
		sqlDB.SetMaxOpenConns(10)
		sqlDB.SetMaxIdleConns(5)
		sqlDB.SetConnMaxLifetime(5 * time.Minute)
	}

	if err := database.AutoMigrate(
		&Category{},
		&CategoryProducts{},
		&Product{},
		&Additional{},
		&AdditionalProducts{},
		&OrderItem{},
		&Order{},
		&Delivery{},
		&Coupon{},
		&CouponUsage{},
		&LoyaltyPoints{},
		&LoyaltyTransaction{},
		&Review{},
		&Batch{},
		// Corte 1 da migração banco-único: push tokens agora em Postgres
		// (tabela também criada por sql/01_dominio_pedidos.sql).
		&PushToken{},
		// Corte 5 da migração banco-único: pedidos (collection Mongo "orders")
		// agora têm espelho em Postgres. Ver models/order_document.go.
		&OrderDocument{},
	); err != nil {
		log.Printf("[CRITICAL] Falha no AutoMigrate do PostgreSQL: %v. "+
			"O servidor continuara, mas tabelas podem estar ausentes. "+
			"Execute scripts/migrate-batches.sql manualmente se necessario.", err)
	}

	DB = database
}


