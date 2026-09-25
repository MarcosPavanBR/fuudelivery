package main

// Ligação pedido ↔ entrega. orders_api e delivery_api não se importam (são
// módulos separados); o monólito, que tem os dois, faz a ponte:
//   - status do pedido gravado → fila do entregador (delivery_solicitations);
//   - entregador aceitou/avançou → pedido (payload.deliveryman e status).
// Antes, isso era o RabbitMQ; desde que ele saiu, nada ligava os dois lados.

import (
	"encoding/json"
	"log"

	"github.com/carloshomar/fuudelivery/auth_api/app/models"
	deliveryDTO "github.com/carloshomar/fuudelivery/delivery_api/app/dto"
	deliveryHandlers "github.com/carloshomar/fuudelivery/delivery_api/app/handlers"
	ordersDTO "github.com/carloshomar/fuudelivery/orders_api/app/dto"
	ordersHandlers "github.com/carloshomar/fuudelivery/orders_api/app/handlers"
	ordersModels "github.com/carloshomar/fuudelivery/orders_api/app/models"
)

func wireCourierFlow() {
	ordersHandlers.OnOrderStatusChanged = syncDeliverySolicitation
	deliveryHandlers.OnCourierUpdate = func(orderID string, c deliveryDTO.DeliveryManDTO) error {
		return ordersHandlers.ApplyCourierUpdate(orderID,
			ordersDTO.DeliveryMan{Id: c.Id, Name: c.Name, Status: c.Status}, sendToEstablishment)
	}
}

// syncDeliverySolicitation leva o pedido à fila do entregador. Os dados da
// loja vêm do cadastro (não do payload, que o app do cliente montou).
func syncDeliverySolicitation(doc *ordersModels.OrderDocument) {
	order, err := solicitationFromOrder(doc)
	if err != nil {
		log.Printf("[DELIVERY] pedido %s: payload ilegível, fila do entregador não atualizada: %v", doc.LegacyID, err)
		return
	}
	if err := deliveryHandlers.SyncSolicitation(order); err != nil {
		log.Printf("[DELIVERY] pedido %s (%s): fila do entregador não atualizada: %v", doc.LegacyID, doc.Status, err)
	}
}

func solicitationFromOrder(doc *ordersModels.OrderDocument) (deliveryDTO.OrderDTO, error) {
	var p ordersDTO.RequestPayload
	if err := json.Unmarshal(doc.Payload, &p); err != nil {
		return deliveryDTO.OrderDTO{}, err
	}
	order := deliveryDTO.OrderDTO{
		OrderId: doc.LegacyID,
		Status:  doc.Status,
		Total:   p.OrderTotal,
		Payment: deliveryDTO.PaymentDTO{Method: p.PaymentMethod.Type},
		User:    deliveryDTO.UserDTO{Name: p.User.Nome, Phone: doc.UserPhone},
		Establishment: deliveryDTO.EstablishmentDTO{
			Id:      doc.EstablishmentID,
			Name:    p.Establishment.Name,
			Lat:     p.Establishment.Latitude,
			Long:    p.Establishment.Longitude,
			Address: p.Establishment.LocationString,
			Image:   p.Establishment.Image,
		},
	}
	if est, err := ordersHandlers.GetEstablishment(doc.EstablishmentID); err == nil {
		order.Establishment = deliveryDTO.EstablishmentDTO{
			Id:      doc.EstablishmentID,
			Name:    est.Name,
			Lat:     est.Latitude,
			Long:    est.Longitude,
			Address: est.LocationString,
			Image:   est.Image,
		}
	}
	// user_id é o id do CLIENTE (tabela clients) — é com ele que o
	// WebSocket reconhece o cliente do pedido.
	if models.DB != nil && doc.UserPhone != "" {
		var client models.Client
		if err := models.DB.Select("id").Where("phone = ?", doc.UserPhone).First(&client).Error; err == nil {
			order.User.ID = int64(client.ID)
		}
	}
	return order, nil
}
