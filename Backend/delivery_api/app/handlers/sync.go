package handlers

// sync.go — ligação pedido ↔ entrega.
//
// Desde a remoção do RabbitMQ nada alimentava delivery_solicitations: o
// pedido aprovado pela loja nunca chegava à lista do entregador. O monólito
// agora chama SyncSolicitation a cada mudança de status do pedido e, no
// sentido inverso, OnCourierUpdate leva o avanço do entregador de volta ao
// pedido (é o que o consumidor antigo do orders_api fazia).

import (
	"encoding/json"
	"errors"
	"log"
	"time"

	"github.com/carloshomar/fuudelivery/delivery_api/app/dto"
	"github.com/carloshomar/fuudelivery/delivery_api/app/models"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm/clause"
)

// dispatchableStatuses são os status do pedido em que ele entra (ou continua)
// na fila do entregador. O resto só atualiza uma solicitação que já existe
// (cancelado depois de aprovado, finalizado).
var dispatchableStatuses = map[string]bool{
	"APPROVED": true, "PREPARING": true, "DONE": true, "IN_ROUTE_DELIVERY": true,
}

// claimableStatuses são os que aparecem na lista e podem ser aceitos.
var claimableStatuses = []string{"APPROVED", "PREPARING", "DONE"}

// OnCourierUpdate leva o entregador e a etapa dele de volta ao pedido
// (payload.deliveryman e, em IN_ROUTE_DELIVERY/FINISHED, o status). Quem liga
// é o monólito; nil nos testes.
var OnCourierUpdate func(orderID string, courier dto.DeliveryManDTO) error

// SyncSolicitation grava no read-model o status atual do pedido. Idempotente:
// o mesmo pedido chega aqui a cada mudança de status.
func SyncSolicitation(order dto.OrderDTO) error {
	if order.OrderId == "" {
		return errors.New("pedido sem order_id")
	}
	if models.DB == nil {
		return errors.New("postgres indisponível")
	}
	if !dispatchableStatuses[order.Status] {
		return models.DB.Model(&models.DeliverySolicitation{}).
			Where("order_id = ?", order.OrderId).
			Update("status", order.Status).Error
	}

	var row models.DeliverySolicitation
	row.FromDTO(order)
	// Quem define o entregador é o aceite (HandShakeDeliveryman), nunca a loja.
	row.DeliveryManID, row.DeliveryManName, row.DeliveryManStatus = 0, "", ""
	return models.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "order_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"status", "updated_at"}),
	}).Create(&row).Error
}

// notifyCourierUpdate chama OnCourierUpdate registrando a falha — o avanço do
// entregador já está gravado; o pedido se acerta na próxima etapa.
func notifyCourierUpdate(orderID string, courier dto.DeliveryManDTO) {
	if OnCourierUpdate == nil {
		return
	}
	if err := OnCourierUpdate(orderID, courier); err != nil {
		log.Printf("[DELIVERY] pedido %s: etapa %s do entregador %d não chegou ao pedido: %v",
			orderID, courier.Status, courier.Id, err)
	}
}

// orderSnapshot é o pedido como o orders_api guarda (order_documents).
type orderSnapshot struct {
	Status  string
	Payload map[string]interface{}
}

// loadOrderSnapshots lê status e payload dos pedidos. Mesmo Postgres do
// orders_api (DB_CONNECTION_STRING); um erro aqui não derruba a resposta — a
// view cai para os campos do read-model.
func loadOrderSnapshots(orderIDs []string) map[string]orderSnapshot {
	out := make(map[string]orderSnapshot, len(orderIDs))
	if len(orderIDs) == 0 {
		return out
	}
	var docs []struct {
		LegacyID string
		Status   string
		Payload  []byte
	}
	if err := models.DB.Table("order_documents").
		Select("legacy_id", "status", "payload").
		Where("legacy_id IN ?", orderIDs).
		Scan(&docs).Error; err != nil {
		log.Printf("[DELIVERY] lendo pedidos para o entregador: %v", err)
		return out
	}
	for _, d := range docs {
		snap := orderSnapshot{Status: d.Status}
		_ = json.Unmarshal(d.Payload, &snap.Payload)
		out[d.LegacyID] = snap
	}
	return out
}

// orderStatus devolve o status do pedido em order_documents ("" se não achar).
func orderStatus(orderID string) string {
	return loadOrderSnapshots([]string{orderID})[orderID].Status
}

// O quanto do cliente a view mostra.
const (
	customerNone    = iota // lista de pedidos disponíveis: ninguém aceitou ainda
	customerName           // extrato: entrega encerrada, só o nome
	customerDetails        // entrega em andamento do próprio entregador
)

// courierOrderViews monta os pedidos no formato que o app do entregador lê —
// o do pedido (RequestPayload do orders_api): order_id, status,
// establishmentId, establishment{id, name, lat, long, location_string},
// deliveryValue, distance, deliveryman, operationDate e, conforme o nível,
// user e location. O "corte 3" passou a devolver o OrderDTO (orderid, sem
// location nem deliveryValue) e o app deixou de achar os campos.
//
// Dado do cliente vai só para quem precisa: na lista (qualquer entregador por
// perto vê) não sai nome, telefone nem endereço.
func courierOrderViews(rows []models.DeliverySolicitation, customer int) []fiber.Map {
	ids := make([]string, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.OrderID)
	}
	snaps := loadOrderSnapshots(ids)

	views := make([]fiber.Map, 0, len(rows))
	for _, r := range rows {
		snap := snaps[r.OrderID]
		p := snap.Payload
		status := snap.Status
		if status == "" {
			status = r.Status
		}

		est := fiber.Map{}
		if m, ok := p["establishment"].(map[string]interface{}); ok {
			for _, k := range []string{"name", "location_string", "Image", "lat", "long"} {
				if v, ok := m[k]; ok {
					est[k] = v
				}
			}
		}
		// O read-model vem do cadastro da loja (não do payload do cliente).
		est["id"] = r.EstablishmentID
		if r.EstablishmentName != "" {
			est["name"] = r.EstablishmentName
		}
		if r.EstablishmentAddress != "" {
			est["location_string"] = r.EstablishmentAddress
		}
		if r.EstablishmentLat != 0 || r.EstablishmentLong != 0 {
			est["lat"], est["long"] = r.EstablishmentLat, r.EstablishmentLong
		}
		// delivery_mode.tsx (openMap) lê latitude/longitude.
		est["latitude"], est["longitude"] = est["lat"], est["long"]

		v := fiber.Map{
			"order_id":        r.OrderID,
			"status":          status,
			"establishmentId": r.EstablishmentID,
			"establishment":   est,
			"deliveryValue":   numberOr(p["deliveryValue"], 0),
			"paymentMethod":   p["paymentMethod"],
			"distance":        deliveryDistance(r, p),
			"operationDate":   r.UpdatedAt.UTC().Format(time.RFC3339),
		}
		if r.DeliveryManID != 0 {
			v["deliveryman"] = fiber.Map{"id": r.DeliveryManID, "name": r.DeliveryManName, "status": r.DeliveryManStatus}
		}

		switch customer {
		case customerName:
			v["user"] = fiber.Map{"nome": customerField(p, r.UserName, "nome")}
		case customerDetails:
			v["user"] = fiber.Map{
				"nome":  customerField(p, r.UserName, "nome"),
				"phone": customerField(p, r.UserPhone, "phone"),
			}
			v["location"] = p["location"]
			v["cart"] = p["cart"]
			v["order_total"] = numberOr(p["order_total"], r.Total)
		}
		views = append(views, v)
	}
	return views
}

// deliveryDistance é a distância da loja ao endereço de entrega, calculada no
// servidor. O "distance" do payload é o que o app do cliente mandou e fica só
// como último recurso.
func deliveryDistance(r models.DeliverySolicitation, p map[string]interface{}) float64 {
	if loc, ok := p["location"].(map[string]interface{}); ok {
		if coords, ok := loc["coords"].(map[string]interface{}); ok {
			lat := numberOr(coords["latitude"], 0)
			lng := numberOr(coords["longitude"], 0)
			if (lat != 0 || lng != 0) && (r.EstablishmentLat != 0 || r.EstablishmentLong != 0) {
				return calculateDistance(r.EstablishmentLat, r.EstablishmentLong, lat, lng)
			}
		}
	}
	return numberOr(p["distance"], 0)
}

func customerField(p map[string]interface{}, fallback, key string) string {
	if u, ok := p["user"].(map[string]interface{}); ok {
		if s, ok := u[key].(string); ok && s != "" {
			return s
		}
	}
	return fallback
}

func numberOr(v interface{}, def float64) float64 {
	if f, ok := v.(float64); ok {
		return f
	}
	return def
}
