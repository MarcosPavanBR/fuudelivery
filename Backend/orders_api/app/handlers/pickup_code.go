package handlers

import (
    "context"
    "crypto/rand"
    "math/big"
    "time"

    "github.com/gofiber/fiber/v2"
    "go.mongodb.org/mongo-driver/bson"
    "go.mongodb.org/mongo-driver/bson/primitive"
    "github.com/carloshomar/vercardapio/app/models"
)

func generateSecureCode() string {
    const charset = "0123456789"
    code := make([]byte, 6)
    for i := range code {
        n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
        code[i] = charset[n.Int64()]
    }
    return string(code)
}

func GeneratePickupCode(c *fiber.Ctx) error {
    var req struct {
        OrderID string `json:"order_id"`
    }
    if err := c.BodyParser(&req); err != nil {
        return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
    }

    orderID, err := primitive.ObjectIDFromHex(req.OrderID)
    if err != nil {
        return c.Status(400).JSON(fiber.Map{"error": "Invalid order ID"})
    }

    code := generateSecureCode()

    collection := models.MongoDabase.Collection("orders")
    filter := bson.M{"_id": orderID}
    update := bson.M{
        "$set": bson.M{
            "pickup_code": code,
            "pickup_code_generated_at": time.Now(),
        },
    }

    result, err := collection.UpdateOne(context.Background(), filter, update)
    if err != nil || result.ModifiedCount == 0 {
        return c.Status(500).JSON(fiber.Map{"error": "Failed to generate code"})
    }

    return c.JSON(fiber.Map{
        "pickup_code": code,
        "order_id":    req.OrderID,
        "message":     "Código de retirada gerado com sucesso",
    })
}

func ValidatePickupCode(c *fiber.Ctx) error {
    var req struct {
        OrderID    string `json:"order_id"`
        PickupCode string `json:"pickup_code"`
    }
    if err := c.BodyParser(&req); err != nil {
        return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
    }

    orderID, err := primitive.ObjectIDFromHex(req.OrderID)
    if err != nil {
        return c.Status(400).JSON(fiber.Map{"error": "Invalid order ID"})
    }

    collection := models.MongoDabase.Collection("orders")
    filter := bson.M{"_id": orderID}

    var order bson.M
    if err := collection.FindOne(context.Background(), filter).Decode(&order); err != nil {
        return c.Status(404).JSON(fiber.Map{"error": "Order not found"})
    }

    storedCode, ok := order["pickup_code"].(string)
    if !ok || storedCode == "" {
        return c.Status(400).JSON(fiber.Map{"error": "Nenhum código de retirada gerado"})
    }

    if storedCode != req.PickupCode {
        return c.Status(401).JSON(fiber.Map{
            "valid": false,
            "error": "Código inválido",
        })
    }

    return c.JSON(fiber.Map{
        "valid":     true,
        "message":   "Código válido! Pedido liberado para retirada.",
        "order_id":  req.OrderID,
    })
}

func GetPickupCode(c *fiber.Ctx) error {
    orderID := c.Params("id")

    oid, err := primitive.ObjectIDFromHex(orderID)
    if err != nil {
        return c.Status(400).JSON(fiber.Map{"error": "Invalid order ID"})
    }

    collection := models.MongoDabase.Collection("orders")
    filter := bson.M{"_id": oid}

    var order bson.M
    if err := collection.FindOne(context.Background(), filter).Decode(&order); err != nil {
        return c.Status(404).JSON(fiber.Map{"error": "Order not found"})
    }

    code, _ := order["pickup_code"].(string)
    generatedAt, _ := order["pickup_code_generated_at"].(string)

    return c.JSON(fiber.Map{
        "order_id":          orderID,
        "pickup_code":       code,
        "generated_at":      generatedAt,
        "has_pickup_code":   code != "",
    })
}
