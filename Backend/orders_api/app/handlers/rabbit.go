package handlers

import (
	"encoding/json"
	"log"
	"time"

	"github.com/carloshomar/vercardapio/orders_api/app/dto"
	"github.com/carloshomar/vercardapio/orders_api/app/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func ReceiveMessage(msg string, sendMessageToClient func(clientID int64, message []byte) error) {
	var orderMsg dto.RequestPayload

	err := json.Unmarshal([]byte(msg), &orderMsg)
	if err != nil {
		log.Printf("Erro ao decodificar a mensagem JSON: %s", err)
	}

	collection := models.MongoDabase.Collection("orders")

	orderID, err := primitive.ObjectIDFromHex(orderMsg.OrderId)
	if err != nil {
		log.Printf("Erro ao converter o ID para ObjectID: %s", err)
	}

	filter := bson.M{"_id": orderID}

	update := bson.M{
		"$set": bson.M{
			"deliveryman":  orderMsg.DeliveryMan,
			"lastModified": time.Now(),
		},
	}

	if orderMsg.DeliveryMan.Status == "FINISHED" {
		update = bson.M{
			"$set": bson.M{
				"status":       "FINISHED",
				"deliveryman":  orderMsg.DeliveryMan,
				"lastModified": time.Now(),
			},
		}
	}

	_, err = collection.UpdateOne(mongoCtx(), filter, update)
	if err != nil {
		log.Printf("Erro ao atualizar o documento: %s", err)
	}

	jsonData, _ := json.Marshal(orderMsg)
	sendMessageToClient(orderMsg.EstablishmentId, jsonData)

	log.Println("Documento atualizado com sucesso.")
}

// PublishMessage envia mensagem para fila de delivery.
// NOTA: RabbitMQ foi removido — stub mantido para compatibilidade.
func PublishMessage(body []byte) error {
	log.Println("[QUEUE] RabbitMQ removido — mensagem ignorada (fila via Redis no monolito)")
	return nil
}
