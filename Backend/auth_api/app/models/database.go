package models

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

const maxRetries = 5
const retryInterval = 5 * time.Second

func ConnectDatabase() {
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
		panic(fmt.Sprintf("Falha ao conectar ao banco de dados após %d tentativas", maxRetries))
	}

	database.AutoMigrate(&User{})
	database.AutoMigrate(&Establishment{})
	database.AutoMigrate(&DeliveryMan{})
	database.AutoMigrate(&BusinessHours{})

	DB = database
}
