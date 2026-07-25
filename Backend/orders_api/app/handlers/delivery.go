package handlers

import (
	"fmt"
	"time"

	"github.com/carloshomar/vercardapio/orders_api/app/models"
	"github.com/gofiber/fiber/v2"
)

func CalculateDeliveryValue(c *fiber.Ctx) error {
	var request struct {
		Distance        float32 `json:"distance"`
		EstablishmentID int64   `json:"establishmentId"`
		UserID          *uint   `json:"user_id,omitempty"` // opcional: para verificar frete gratis da assinatura
		OrderTotal      float64 `json:"order_total,omitempty"` // valor total do pedido para frete gratis
	}

	if err := c.BodyParser(&request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Failed to parse request body",
		})
	}

	// Se establishmentId não estiver presente na solicitação, definimos o valor padrão como 1 (matriz)
	if request.EstablishmentID == 0 {
		request.EstablishmentID = 1
	}

	var delivery models.Delivery
	if err := models.DB.Where("establishment_id = ?", request.EstablishmentID).First(&delivery).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch delivery settings",
		})
	}

	// Calcular o valor base da entrega
	baseDeliveryValue := (request.Distance * delivery.PerKm) + delivery.FixedTaxa
	deliveryValue := float64(baseDeliveryValue)
	subscriptionDiscount := false

	// Verifica se o usuario tem assinatura ativa que concede frete gratis
	if request.UserID != nil && *request.UserID > 0 {
		// Busca assinatura ativa do usuario na tabela subscriptions (auth_api)
		// Usamos o mesmo DB (compartilhado via GORM) porque subscriptions
		// esta no mesmo banco PostgreSQL que orders_api
		type SubscriptionCheck struct {
			Plan               string
			Status             string
			FreeDeliveryAbove  float64
			CurrentPeriodStart string
			CurrentPeriodEnd   string
		}
		var sub SubscriptionCheck
		// Tenta buscar a subscription no banco compartilhado
		if err := models.DB.Table("subscriptions").
			Where("user_id = ? AND status = 'active'", *request.UserID).
			Select("plan, status, free_delivery_above, current_period_start::text as current_period_start, current_period_end::text as current_period_end").
			Scan(&sub).Error; err == nil && sub.Status == "active" {

			now := time.Now()
			// Verifica periodo vigente
			startTime, _ := time.Parse("2006-01-02T15:04:05Z", sub.CurrentPeriodStart)
			endTime, _ := time.Parse("2006-01-02T15:04:05Z", sub.CurrentPeriodEnd)
			// Tenta parse sem timezone
			if startTime.IsZero() {
				startTime, _ = time.Parse("2006-01-02 15:04:05", sub.CurrentPeriodStart)
			}
			if endTime.IsZero() {
				endTime, _ = time.Parse("2006-01-02 15:04:05", sub.CurrentPeriodEnd)
			}
			// Se falhou parse, assume que esta valido
			periodValid := startTime.IsZero() || (now.After(startTime) && now.Before(endTime))

			if periodValid {
				switch sub.Plan {
				case "premium":
					// Premium: frete gratis sempre
					deliveryValue = 0
					subscriptionDiscount = true
				case "basic":
					// Basic: frete gratis acima do valor minimo
					if sub.FreeDeliveryAbove > 0 && request.OrderTotal >= sub.FreeDeliveryAbove {
						deliveryValue = 0
						subscriptionDiscount = true
					}
				}
			}
		}
	}

	return c.JSON(fiber.Map{
		"deliveryValue":        deliveryValue,
		"baseDeliveryValue":    float64(baseDeliveryValue),
		"subscriptionDiscount": subscriptionDiscount,
	})
}

func InsertDelivery(c *fiber.Ctx) error {
	var request struct {
		EstablishmentID uint    `json:"establishmentId"`
		FixedTaxa       float32 `json:"fixedTaxa"`
		PerKm           float32 `json:"perKm"`
	}

	if err := c.BodyParser(&request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Failed to parse request body",
		})
	}

	newDelivery := models.Delivery{
		EstablishmentID: request.EstablishmentID,
		FixedTaxa:       request.FixedTaxa,
		PerKm:           request.PerKm,
	}

	if err := models.CreateOrUpdateDelivery(&newDelivery); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to insert or update delivery data",
		})
	}

	return c.JSON(fiber.Map{
		"delivery": newDelivery,
	})
}


func CalculateRoute(c *fiber.Ctx) error {
	var request struct {
		OriginLat      float64 `json:"origin_lat"`
		OriginLng      float64 `json:"origin_lng"`
		DestLat        float64 `json:"dest_lat"`
		DestLng        float64 `json:"dest_lng"`
		EstablishmentID int64  `json:"establishmentId"`
	}

	if err := c.BodyParser(&request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if request.EstablishmentID == 0 {
		request.EstablishmentID = 1
	}

	var delivery models.Delivery
	if err := models.DB.Where("establishment_id = ?", request.EstablishmentID).First(&delivery).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch delivery settings"})
	}

	// Try OSRM first (real driving distance)
	distanceKm, durationMin, osrmOk := getOSRMDistance(
		request.OriginLat, request.OriginLng,
		request.DestLat, request.DestLng,
	)

	source := "osrm"
	if !osrmOk {
		// Fallback to Haversine
		distanceKm = calculateDistance(
			request.OriginLat, request.OriginLng,
			request.DestLat, request.DestLng,
		)
		durationMin = (distanceKm / 30.0) * 60.0 // ~30km/h avg speed
		source = "haversine"
	}

	deliveryValue := (float32(distanceKm) * delivery.PerKm) + delivery.FixedTaxa

	return c.JSON(fiber.Map{
		"distance_km":   fmt.Sprintf("%.2f", distanceKm),
		"duration_min":  fmt.Sprintf("%.1f", durationMin),
		"delivery_value": deliveryValue,
		"source":        source,
	})
}

func GetDeliveryByEstablishmentID(c *fiber.Ctx) error {
	// Extrair o establishmentId dos parâmetros da URL
	establishmentID := c.Params("establishmentId")

	// Converter o establishmentId para o tipo correto (int64)
	var id int64
	if _, err := fmt.Sscanf(establishmentID, "%d", &id); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid establishmentId format",
		})
	}

	// Buscar as informações de entrega no banco de dados
	var delivery models.Delivery
	if err := models.DB.Where("establishment_id = ?", id).First(&delivery).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Delivery settings not found for the establishment",
		})
	}

	return c.JSON(delivery)
}
