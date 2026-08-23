package handlers

// orders_pg.go — camada de migração do corte 5 (banco-único).
//
// Papel deste arquivo:
//   - Centralizar TODA a lógica de transição Postgres ↔ Mongo para a collection
//     "orders", para que os handlers fiquem limpos e o desligamento futuro do
//     Atlas seja remover as funções *_legacy* daqui (e nada mais).
//   - Pós-corte 5, o POSTGRES é a fonte da verdade:
//       1. Escrita: grava em Postgres; tenta espelhar no Mongo (best-effort,
//          erro é logado e não quebra a requisição).
//       2. Leitura: consulta Postgres primeiro; se o pedido não existir lá mas
//          existir no Mongo (dado legado pré-migração), importa na hora
//          ("lazy import", mesmo padrão do corte 4 em payment_api).

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/carloshomar/fuudelivery/orders_api/app/dto"
	"github.com/carloshomar/fuudelivery/orders_api/app/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// mongoFindOptions monta sort/limit opcionais para as queries fallback do
// Mongo legado. sortField vazio = sem sort; limit 0 = sem limite.
func mongoFindOptions(sortField string, limit int64) *options.FindOptions {
	opts := options.Find()
	if sortField != "" {
		opts.SetSort(bson.D{{Key: sortField, Value: -1}})
	}
	if limit > 0 {
		opts.SetLimit(limit)
	}
	return opts
}

// newLegacyOrderID gera um novo identificador público no MESMO formato que os
// clientes já conhecem (ObjectID hex de 24 chars). Manter o formato evita
// tocar apps mobile, webs, delivery_api e reviews — para eles nada muda.
func newLegacyOrderID() string {
	return primitive.NewObjectID().Hex()
}

// payloadToDoc converte um RequestPayload em linha do Postgres, extraindo as
// colunas tipadas usadas em filtros/índices e serializando o payload completo.
func payloadToDoc(legacyID string, p *dto.RequestPayload) (*models.OrderDocument, error) {
	raw, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("serializando payload do pedido %s: %w", legacyID, err)
	}

	doc := &models.OrderDocument{
		LegacyID:        legacyID,
		EstablishmentID: p.EstablishmentId,
		UserPhone:       p.User.Phone,
		Status:          p.Status,
		ScheduledAt:     p.ScheduledAt,
		IsScheduled:     p.IsScheduled,
		Payload:         raw,
	}
	return doc, nil
}

// saveOrderPrimary grava o pedido no Postgres (upsert por legacy_id) e espelha
// no Mongo best-effort. O payload é sempre re-serializado junto com as colunas,
// garantindo que ambos fiquem consistentes.
func saveOrderPrimary(doc *models.OrderDocument) error {
	if err := models.DB.Where("legacy_id = ?", doc.LegacyID).
		Assign(*doc).FirstOrCreate(doc).Error; err != nil {
		return fmt.Errorf("persistindo pedido %s em Postgres: %w", doc.LegacyID, err)
	}

	dualWriteOrderToMongo(doc)
	return nil
}

// dualWriteOrderToMongo espelha o documento no Mongo legado. Best-effort:
// falha é logada e ignorada — o Atlas será desligado após o ETL completo.
func dualWriteOrderToMongo(doc *models.OrderDocument) {
	if models.MongoDabase == nil || doc == nil {
		return // Mongo indisponível (ou desligado) — comportamento esperado
	}

	var p dto.RequestPayload
	if err := json.Unmarshal(doc.Payload, &p); err != nil {
		log.Printf("[ORDER-MIRROR] Payload inválido do pedido %s: %v", doc.LegacyID, err)
		return
	}

	oid, err := primitive.ObjectIDFromHex(doc.LegacyID)
	if err != nil {
		log.Printf("[ORDER-MIRROR] legacy_id inválido %s: %v", doc.LegacyID, err)
		return
	}

	collection := models.MongoDabase.Collection("orders")
	_, err = collection.UpdateOne(mongoCtx(),
		bson.M{"_id": oid},
		bson.M{"$set": p},
		options.Update().SetUpsert(true))
	if err != nil {
		log.Printf("[ORDER-MIRROR] Falha ao espelhar pedido %s no Mongo: %v", doc.LegacyID, err)
	}
}

// findOrderByLegacyID busca o pedido: Postgres primeiro; se não houver e o
// Mongo tiver (dado legado), importa para o Postgres na hora e devolve.
func findOrderByLegacyID(legacyID string) (*models.OrderDocument, error) {
	if models.DB == nil {
		return nil, fmt.Errorf("Postgres indisponível")
	}

	var doc models.OrderDocument
	err := models.DB.Where("legacy_id = ?", legacyID).First(&doc).Error
	if err == nil {
		return &doc, nil
	}

	// Não achou no Postgres — tenta lazy import do Mongo (dado legado).
	if models.MongoDabase != nil {
		oid, convErr := primitive.ObjectIDFromHex(legacyID)
		if convErr != nil {
			return nil, convErr
		}
		var p dto.RequestPayload
		if findErr := models.MongoDabase.Collection("orders").
			FindOne(mongoCtx(), bson.M{"_id": oid}).Decode(&p); findErr != nil {
			return nil, err // erro original do Postgres prevalece (provavelmente not found)
		}
		imported, buildErr := payloadToDoc(legacyID, &p)
		if buildErr != nil {
			return nil, buildErr
		}
		if saveErr := saveOrderPrimary(imported); saveErr != nil {
			log.Printf("[ORDER-LAZY] Falha ao importar pedido %s para Postgres: %v", legacyID, saveErr)
			// Mesmo assim devolvemos o dado importado — melhor servir do que falhar.
			return imported, nil
		}
		log.Printf("[ORDER-LAZY] Pedido %s importado do Mongo para Postgres", legacyID)
		return imported, nil
	}

	return nil, err
}

// patchOrderDoc aplica mutações no documento (colunas + payload espelhado),
// persiste no Postgres e propaga ao Mongo best-effort.
func patchOrderDoc(doc *models.OrderDocument, mutate func(p *dto.RequestPayload)) error {
	var p dto.RequestPayload
	if err := json.Unmarshal(doc.Payload, &p); err != nil {
		return fmt.Errorf("desserializando payload do pedido %s: %w", doc.LegacyID, err)
	}
	mutate(&p)

	raw, err := json.Marshal(p)
	if err != nil {
		return err
	}

	doc.Status = p.Status
	doc.ScheduledAt = p.ScheduledAt
	doc.IsScheduled = p.IsScheduled
	doc.UserPhone = p.User.Phone
	doc.EstablishmentID = p.EstablishmentId
	doc.Payload = raw

	return saveOrderPrimary(doc)
}

// docToResponseMap reconstrói o "documento" que os frontends consomem:
// payload completo + _id (hex legado) + campos de coluna como fonte da verdade.
func docToResponseMap(doc *models.OrderDocument) map[string]interface{} {
	out := make(map[string]interface{})
	if len(doc.Payload) > 0 {
		_ = json.Unmarshal(doc.Payload, &out) // payload válido por construção
	}
	out["_id"] = doc.LegacyID
	out["status"] = doc.Status
	if doc.PickupCode != "" {
		out["pickup_code"] = doc.PickupCode
	}
	if doc.ScheduledAt != nil {
		out["scheduled_at"] = doc.ScheduledAt.Format(time.RFC3339)
	}
	out["is_scheduled"] = doc.IsScheduled
	out["created_at"] = doc.CreatedAt.Format(time.RFC3339)
	return out
}
