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
	// DB é a conexão Postgres (GORM) — fonte da verdade desde o corte 4
	// da migração banco-único (docs/ARQUITETURA-BANCO-UNICO.md).
	DB *gorm.DB
)

const pgMaxRetries = 5
const pgRetryInterval = 5 * time.Second

// ConnectPostgresDatabase conecta ao Supabase/Postgres usando a mesma env var
// do resto do monorepo (DB_CONNECTION_STRING) e o mesmo padrão de retry dos
// outros módulos — manter todos iguais facilita manutenção.
func ConnectPostgresDatabase() {
	dsn := os.Getenv("DB_CONNECTION_STRING")
	if dsn == "" {
		panic("DB_CONNECTION_STRING não configurado")
	}

	var database *gorm.DB
	var err error

	for attempt := 1; attempt <= pgMaxRetries; attempt++ {
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
		log.Printf("[PAYMENT-DB] tentativa %d/%d falhou: %v", attempt, pgMaxRetries, err)
		time.Sleep(pgRetryInterval)
	}

	if err != nil {
		panic(fmt.Sprintf("Falha ao conectar ao PostgreSQL após %d tentativas", pgMaxRetries))
	}

	// Limites de pool (ver database.go do auth_api): 5 módulos compartilham
	// o mesmo Supabase — cada pool precisa de teto.
	if sqlDB, sErr := database.DB(); sErr == nil {
		sqlDB.SetMaxOpenConns(10)
		sqlDB.SetMaxIdleConns(5)
		sqlDB.SetConnMaxLifetime(5 * time.Minute)
	}

	// O schema oficial vive nos scripts SQL (sql/03_dominio_pagamentos.sql e
	// sql/10_wallet_ledger_kind.sql). O AutoMigrate é rede de segurança para
	// ambientes novos/dev — em produção quem manda é o run_all.sh.
	if err := database.AutoMigrate(&Payment{}, &Wallet{}, &WalletTxn{}); err != nil {
		// Schema de produção é governado por sql/03 + sql/10 — drift do GORM
		// não pode derrubar o serviço em loop.
		log.Printf("[CRITICAL] AutoMigrate das tabelas de pagamento falhou (seguindo): %v", err)
	}

	DB = database
	log.Println("Conexão com o PostgreSQL estabelecida com sucesso!")
}
