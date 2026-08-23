// ============================================================================
// etl-orders — migração de dados legados do MongoDB para o Postgres
// (CORTE 5 banco-único — docs/ARQUITETURA-BANCO-UNICO.md).
//
// O que faz (one-shot, idempotente — pode rodar mais de uma vez):
//
//	Copia cada documento da collection "orders" (Mongo legado) para a tabela
//	order_documents do Postgres, exatamente como o handler faz em produção:
//	payload completo em JSONB + colunas tipadas extraídas. Dedup por
//	legacy_id (upsert manual) — pedidos já importados pelo lazy import do
//	handler não são duplicados.
//
// Por que existe: o lazy import cobre leituras pontuais por ID, mas as
// LISTAGENS só servem dados que já estão no Postgres. Rodar este ETL uma vez
// garante que nenhum pedido antigo desapareça das listas quando o Atlas for
// desligado.
//
// Como rodar (as env vars são as mesmas do monolito):
//
//	DB_CONNECTION_STRING="postgres://..." \
//	MONGO_URI="mongodb+srv://..." \
//	MONGO_DATABASE="fuudelivery" \
//	go run ./cmd/etl-orders
//
// Segurança: NÃO apaga nada em nenhum dos bancos. Rode antes de desligar o
// Mongo Atlas e confira os totais impressos ao final.
// ============================================================================
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/carloshomar/fuudelivery/orders_api/app/dto"
	"github.com/carloshomar/fuudelivery/orders_api/app/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	// --- Conexões ---
	mongoURI := os.Getenv("MONGO_URI")
	if mongoURI == "" {
		log.Fatal("MONGO_URI não configurado — informe o Mongo Atlas legado")
	}
	dbName := os.Getenv("MONGO_DATABASE")
	if dbName == "" {
		dbName = "fuudelivery"
	}

	mc, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		log.Fatalf("conectar no Mongo: %v", err)
	}
	defer mc.Disconnect(ctx)
	if err := mc.Ping(ctx, nil); err != nil {
		log.Fatalf("ping no Mongo falhou: %v", err)
	}
	legacy := mc.Database(dbName)
	log.Printf("[ETL] Mongo legado: %s (db=%s)", mongoURI, dbName)

	models.ConnectPostgresDatabase() // mesmo padrão de retry do monolito
	db := models.DB

	cursor, err := legacy.Collection("orders").Find(ctx, bson.M{})
	if err != nil {
		log.Fatalf("buscar pedidos: %v", err)
	}
	defer cursor.Close(ctx)

	var imported, skipped int
	for cursor.Next(ctx) {
		var raw bson.M
		if err := cursor.Decode(&raw); err != nil {
			log.Printf("[ETL] documento inválido ignorado: %v", err)
			skipped++
			continue
		}

		idRaw, ok := raw["_id"]
		if !ok {
			skipped++
			continue
		}
		oid, ok := idRaw.(primitive.ObjectID)
		if !ok {
			log.Printf("[ETL] _id inesperado (%T) — pulando", idRaw)
			skipped++
			continue
		}
		legacyID := oid.Hex()

		// Reinterpreta o documento como RequestPayload (mesma estrutura que
		// os handlers gravam). Campos extras do documento legado (ex.:
		// pickup_code) não fazem parte da struct e são descartados com
		// segurança — vivem em colunas tipadas ou são transitórios.
		rawJSON, err := bsonToJSON(raw)
		if err != nil {
			log.Printf("[ETL] pedido %s: falha ao serializar: %v", legacyID, err)
			skipped++
			continue
		}
		var p dto.RequestPayload
		if err := json.Unmarshal(rawJSON, &p); err != nil {
			log.Printf("[ETL] pedido %s: payload ilegível: %v", legacyID, err)
			skipped++
			continue
		}

		payloadBytes, _ := json.Marshal(p)
		doc := &models.OrderDocument{
			LegacyID:        legacyID,
			EstablishmentID: p.EstablishmentId,
			UserPhone:       p.User.Phone,
			Status:          p.Status,
			ScheduledAt:     p.ScheduledAt,
			IsScheduled:     p.IsScheduled,
			Payload:         payloadBytes,
		}

		// Upsert por legacy_id (idempotente). Preserva pickup_code e datas
		// caso o lazy import já tenha criado a linha.
		res := db.Where("legacy_id = ?", legacyID).
			Assign(models.OrderDocument{
				EstablishmentID: doc.EstablishmentID,
				UserPhone:       doc.UserPhone,
				Status:          doc.Status,
				ScheduledAt:     doc.ScheduledAt,
				IsScheduled:     doc.IsScheduled,
				Payload:         doc.Payload,
			}).FirstOrCreate(doc)
		if res.Error != nil {
			log.Printf("[ETL] pedido %s: falha ao gravar: %v", legacyID, res.Error)
			continue
		}
		imported++
	}
	if err := cursor.Err(); err != nil {
		log.Fatalf("cursor do Mongo: %v", err)
	}

	fmt.Println("\n================ RESUMO DO ETL (orders) ================")
	fmt.Printf("Pedidos importados/atualizados : %d\n", imported)
	fmt.Printf("Documentos ignorados/inválidos : %d\n", skipped)
	fmt.Println("Confira estes totais contra o Atlas antes de pausá-lo.")
	fmt.Println("========================================================")
}

// bsonToJSON converte um documento BSON para JSON usando a serialização
// oficial do driver (trata ObjectID, datas, Decimal128 etc.).
func bsonToJSON(raw bson.M) ([]byte, error) {
	marshalExt, err := bson.MarshalExtJSON(raw, false, false)
	if err != nil {
		return nil, fmt.Errorf("bson→json: %w", err)
	}
	return marshalExt, nil
}
