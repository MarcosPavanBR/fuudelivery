package handlers

// orders_pg.go — camada de persistência Postgres para pedidos.
//
// Papel deste arquivo:
//   - Centralizar toda a lógica de persistência de pedidos em Postgres.
//   - Conversão entre RequestPayload (JSON) e OrderDocument (linhas Postgres).

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"time"

	"github.com/carloshomar/fuudelivery/orders_api/app/dto"
	"github.com/carloshomar/fuudelivery/orders_api/app/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// newLegacyOrderID gera um novo identificador público no MESMO formato que os
// clientes já conhecem (hex de 24 chars). Manter o formato evita
// tocar apps mobile, webs, delivery_api e reviews — para eles nada muda.
func newLegacyOrderID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}

// payloadToDoc converte um RequestPayload em linha do Postgres, extraindo as
// colunas tipadas usadas em filtros/índices e serializando o payload completo.
func payloadToDoc(legacyID string, p *dto.RequestPayload) (*models.OrderDocument, error) {
	raw, err := json.Marshal(p)
	if err != nil {
		return nil, err
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

// saveOrderPrimary grava o pedido no Postgres (upsert por legacy_id).
func saveOrderPrimary(doc *models.OrderDocument) error {
	if err := models.DB.Where("legacy_id = ?", doc.LegacyID).
		Assign(*doc).FirstOrCreate(doc).Error; err != nil {
		return fmt.Errorf("persistindo pedido %s em Postgres: %w", doc.LegacyID, err)
	}
	return nil
}

// findOrderByLegacyID busca o pedido no Postgres.
func findOrderByLegacyID(legacyID string) (*models.OrderDocument, error) {
	if models.DB == nil {
		return nil, fmt.Errorf("Postgres indisponível")
	}

	var doc models.OrderDocument
	err := models.DB.Where("legacy_id = ?", legacyID).First(&doc).Error
	if err != nil {
		return nil, err
	}
	return &doc, nil
}

// patchOrderDoc aplica mutações no documento (colunas + payload espelhado)
// e persiste no Postgres.
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

// mongoCtx devolve um contexto com timeout para operações legadas no Mongo.
// Usado apenas como fallback — o Postgres é a fonte da verdade.
func mongoCtx() context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return ctx
}

// mongoFindOptions monta sort/limit opcionais para queries fallback do Mongo.
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
