package handlers

// orders_pg.go — camada de persistência Postgres para pedidos.
//
// Papel deste arquivo:
//   - Centralizar toda a lógica de persistência de pedidos em Postgres.
//   - Conversão entre RequestPayload (JSON) e OrderDocument (linhas Postgres).

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"time"

	"github.com/carloshomar/fuudelivery/orders_api/app/dto"
	"github.com/carloshomar/fuudelivery/orders_api/app/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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

// patchOrderDoc aplica mutate ao pedido e grava (colunas + payload).
//
// A linha é RECARREGADA sob lock (SELECT ... FOR UPDATE) dentro de uma
// transação e mutate recebe essa versão, não a que o chamador leu antes: a
// loja mudando o status e o entregador avançando a entrega ao mesmo tempo não
// apagam a mudança um do outro (o payload inteiro é regravado). Um erro de
// mutate desfaz tudo e volta para o chamador. *doc fica com o que foi gravado.
func patchOrderDoc(doc *models.OrderDocument, mutate func(d *models.OrderDocument, p *dto.RequestPayload) error) error {
	var saved models.OrderDocument
	err := models.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("legacy_id = ?", doc.LegacyID).First(&saved).Error; err != nil {
			return fmt.Errorf("recarregando pedido %s: %w", doc.LegacyID, err)
		}
		var p dto.RequestPayload
		if err := json.Unmarshal(saved.Payload, &p); err != nil {
			return fmt.Errorf("desserializando payload do pedido %s: %w", doc.LegacyID, err)
		}
		if err := mutate(&saved, &p); err != nil {
			return err
		}

		raw, err := json.Marshal(p)
		if err != nil {
			return err
		}
		// Vazio no payload não apaga a coluna (o upsert antigo, com Assign de
		// struct, também pulava zero): loja, telefone e status só mudam
		// quando o payload diz algo.
		if p.Status != "" {
			saved.Status = p.Status
		}
		if p.User.Phone != "" {
			saved.UserPhone = p.User.Phone
		}
		if p.EstablishmentId != 0 {
			saved.EstablishmentID = p.EstablishmentId
		}
		saved.ScheduledAt = p.ScheduledAt
		saved.IsScheduled = p.IsScheduled
		saved.Payload = raw
		return tx.Save(&saved).Error
	})
	if err != nil {
		return err
	}
	*doc = saved
	return nil
}

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
