package handlers

// courier_sync.go — ligação pedido ↔ entrega (lado do pedido).
//
// Desde a remoção do RabbitMQ os dois lados não conversavam: o pedido
// aprovado não chegava ao entregador e o avanço do entregador não voltava ao
// pedido. O monólito liga OnOrderStatusChanged à fila do entregador
// (delivery_api.SyncSolicitation) e chama ApplyCourierUpdate quando o
// entregador aceita ou avança uma entrega.

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/carloshomar/fuudelivery/orders_api/app/dto"
	"github.com/carloshomar/fuudelivery/orders_api/app/models"
	"github.com/gofiber/fiber/v2"
)

// OnOrderStatusChanged é chamado depois que uma mudança de status do pedido é
// gravada. nil nos testes.
var OnOrderStatusChanged func(doc *models.OrderDocument)

func notifyOrderStatusChanged(doc *models.OrderDocument) {
	// Indicação: prêmio no 1º pedido entregue / devolução do cupom se cancelar.
	handleReferralOnStatus(doc)
	if OnOrderStatusChanged != nil {
		OnOrderStatusChanged(doc)
	}
}

// ApplyCourierUpdate grava no pedido o entregador e a etapa dele — o que o
// consumidor antigo do RabbitMQ (ReceiveMessage) fazia:
//   - payload.deliveryman passa a ser o entregador (é por ele que
//     canValidatePickupCode reconhece quem pode validar o código);
//   - "a caminho da entrega" leva o pedido pronto (DONE) a IN_ROUTE_DELIVERY;
//   - "entregue" finaliza o pedido, se a transição vale.
//
// A loja é avisada pelo WebSocket e o cliente pelo push quando o status muda.
// Um pedido já de outro entregador não é sobrescrito.
func ApplyCourierUpdate(orderID string, courier dto.DeliveryMan, sendMessageToClient func(clientID int64, message []byte) error) error {
	doc, err := findOrderByLegacyID(orderID)
	if err != nil {
		return fmt.Errorf("pedido %s: %w", orderID, err)
	}
	before := doc.Status

	err = patchOrderDoc(doc, func(d *models.OrderDocument, p *dto.RequestPayload) error {
		if p.DeliveryMan.Id != 0 && p.DeliveryMan.Id != courier.Id {
			return fmt.Errorf("pedido %s já é do entregador %d", orderID, p.DeliveryMan.Id)
		}
		p.DeliveryMan = courier
		switch courier.Status {
		case "IN_ROUTE_DELIVERY":
			if d.Status == "DONE" {
				p.Status = "IN_ROUTE_DELIVERY"
			}
		case "FINISHED":
			if isValidOrderTransition(d.Status, "FINISHED") {
				p.Status = "FINISHED"
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	if doc.Status != before {
		notifyOrderStatusChanged(doc)
		var order dto.RequestPayload
		if json.Unmarshal(doc.Payload, &order) == nil {
			go sendStatusPushNotification(order, doc.Status)
		}
	}

	if sendMessageToClient != nil {
		msg, _ := json.Marshal(fiber.Map{"id": orderID, "status": doc.Status, "deliveryman": courier})
		if err := sendMessageToClient(doc.EstablishmentID, msg); err != nil {
			log.Printf("[ORDER] aviso à loja %d sobre o pedido %s falhou: %v", doc.EstablishmentID, orderID, err)
		}
	}
	return nil
}
