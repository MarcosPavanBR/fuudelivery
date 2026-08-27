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

	// Limites de pool: sem isso o módulo abria conexões ilimitadas; somados
	// os 5 módulos do monólito contra o mesmo Supabase, uma rajada esgota
	// o limite do pooler e derruba todas as rotas.
	if sqlDB, sErr := database.DB(); sErr == nil {
		sqlDB.SetMaxOpenConns(10)
		sqlDB.SetMaxIdleConns(5)
		sqlDB.SetConnMaxLifetime(5 * time.Minute)
	}

	if err := database.AutoMigrate(
		&User{},
		&Establishment{},
		&DeliveryMan{},
		&BusinessHours{},
		&Zone{},
		&Subscription{},
		&SponsoredListing{},
		&Client{},
		&RefreshToken{},
		&PasswordResetToken{},
	); err != nil {
		// AutoMigrate é rede de segurança para DEV — em produção o schema é
		// governado por sql/ + run_all.sh (fonte da verdade). Um drift do GORM
		// (ex.: constraint "uni_clients_phone" não existe na base real) NÃO pode
		// derrubar o serviço em loop. O erro fica LOUD no log; conexão com banco
		// indisponível continua sendo fatal (abaixo). O incidente de 2026-08-24
		// (refresh_tokens faltando) é coberto pelo script sql + run_all.sh.
		log.Printf("[CRITICAL] AutoMigrate do auth_api falhou (schema gerenciado por sql/; seguindo): %v", err)
	}

	// Atualiza DeliveryMan com campos do motor de despacho se nao existirem
	// (GORM AutoMigrate adiciona colunas novas, mas nao remove)
	if database.Migrator().HasColumn(&DeliveryMan{}, "zone_id") {
		log.Println("[COURIER] DeliveryMan ja possui campos de zona — migration OK")
	}

	// Cria zona padrao se nao existir (5/85)
	var count int64
	database.Model(&Zone{}).Count(&count)
	if count == 0 {
		defaultZone := Zone{
			Name:                    "Padrão",
			PlatformFeePercentage:   5.0,
			EstablishmentPercentage: 85.0,
			IsActive:                true,
			CitySize:                "medium",
			MinRadiusKm:             2.0,
			RadiusKm:                5.0,
			MaxRadiusKm:             15.0,
			PeakHourStart:           "11:00",
			PeakHourEnd:             "14:00",
			PeakRadiusMultiplier:    0.7,
			MinDeliveryFee:          5.0,
			SurgeMultiplier:         1.0,
			MinCouriersThreshold:    3,
			MatchAlgorithm:          "proximity",
			AllowBatching:           true,

			// Decaimento de split
			SplitInitialPlatformPct:      3.0,
			SplitInitialEstablishmentPct: 87.0,
			SplitTargetPlatformPct:       12.0,
			SplitTargetEstablishmentPct:  78.0,
			SplitStepMonths:              3,
			SplitStepPlatformPct:         1.5,
			SplitStepEstablishmentPct:    -1.5,
			SplitMinMonthlyOrders:        50,
			SplitMinActiveCouriers:       3,
			SplitCurrentPlatformPct:      3.0,
			SplitCurrentEstablishmentPct: 87.0,
		}
		database.Create(&defaultZone)
		log.Println("[ZONE] Zona padrao criada: 5%% plataforma / 85%% estabelecimento, raio 5km")
	}

	DB = database
}
