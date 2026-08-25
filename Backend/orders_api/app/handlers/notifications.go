package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/carloshomar/fuudelivery/orders_api/app/models"
	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// mongoCtx devolve um contexto com timeout para as operações legadas de
// dual-write no Mongo. O cancelamento é agendado via time.AfterFunc em vez
// de defer: o helper retorna o contexto para o chamador, então o cancel
// precisa disparar DEPOIS que a operação Mongo rodar.
func mongoCtx() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(5*time.Second, cancel)
	return ctx
}

type PushTicket struct {
	Status string `json:"status"`
	ID     string `json:"id,omitempty"`
}

type PushMessage struct {
	To    string                 `json:"to"`
	Title string                 `json:"title"`
	Body  string                 `json:"body"`
	Data  map[string]interface{} `json:"data,omitempty"`
}

func SendPushNotification(token string, title string, body string, data map[string]interface{}) error {
	message := PushMessage{
		To:    token,
		Title: title,
		Body:  body,
		Data:  data,
	}

	jsonData, _ := json.Marshal(message)

	resp, err := http.Post(
		"https://exp.host/--/api/v2/push/send",
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

func RegisterPushToken(c *fiber.Ctx) error {
	var req struct {
		UserID    int64  `json:"user_id"`
		UserType  string `json:"user_type"`
		PushToken string `json:"push_token"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}

	// ── CORTE 1 (banco-único): escrita PRIMÁRIA em Postgres ─────────────
	// Upsert por (user_id, user_type) — espelha o índice único da tabela
	// criada em sql/01_dominio_pedidos.sql.
	db := models.DB
	if db == nil {
		return c.Status(500).JSON(fiber.Map{"error": "Postgres indisponível"})
	}
	// Normaliza o tipo: os apps enviam "customer", mas o envio de push de
	// status consulta user_type = "client" (sendStatusPushNotification).
	if req.UserType == "customer" {
		req.UserType = "client"
	}
	pushToken := models.PushToken{
		UserID:    req.UserID,
		UserType:  req.UserType,
		PushToken: req.PushToken,
	}
	if err := db.Where(models.PushToken{UserID: req.UserID, UserType: req.UserType}).
		Assign(models.PushToken{PushToken: req.PushToken, UpdatedAt: time.Now()}).
		FirstOrCreate(&pushToken).Error; err != nil {
		log.Printf("[PUSH_TOKEN] Falha ao salvar token no Postgres (user=%d): %v", req.UserID, err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to register token"})
	}

	// ── Escrita legada no Mongo (dual-write temporário) ──────────────────
	// Mantida até o ETL/desligamento do Mongo. Erro aqui NÃO falha a request:
	// o Postgres já é a fonte de verdade da leitura.
	if models.MongoDabase != nil {
		collection := models.MongoDabase.Collection("push_tokens")
		filter := bson.M{"user_id": req.UserID, "user_type": req.UserType}
		update := bson.M{"$set": bson.M{"push_token": req.PushToken, "updated_at": time.Now()}}
		opts := options.Update().SetUpsert(true)
		if _, err := collection.UpdateOne(mongoCtx(), filter, update, opts); err != nil {
			log.Printf("[PUSH_TOKEN] Dual-write Mongo falhou (não-crítico): %v", err)
		}
	}

	return c.JSON(fiber.Map{"message": "Token registered"})
}
