package handlers

import (
	"fmt"
	"time"

	"github.com/carloshomar/fuudelivery/orders_api/app/models"
	"github.com/gofiber/fiber/v2"
)

// deliveryFee é o resultado do cálculo do frete.
type deliveryFee struct {
	Value                float64 // o que o cliente paga (0 se a assinatura zerar)
	BaseValue            float64 // antes do desconto de assinatura
	SubscriptionDiscount bool
}

// computeDeliveryFee calcula o frete a partir da distância e das configurações
// do estabelecimento, aplicando o frete grátis da assinatura quando houver.
//
// Existe como função (e não só dentro do handler) porque DOIS caminhos
// precisam do mesmo número: a cotação (POST /delivery/calculate-delivery-value,
// que o app chama antes de fechar o pedido) e a CRIAÇÃO DO PEDIDO
// (computeOrderTotal). Antes, só a cotação calculava — a criação do pedido
// aceitava o `deliveryValue` que o cliente mandava no corpo e o somava ao
// total. Ou seja, quem controlava o pedido definia o próprio frete.
//
// Isso importa porque o frete alimenta o split do pagamento: a cobrança passou
// a conferir o frete contra o pedido, mas se o valor gravado NO pedido já veio
// do cliente, a conferência só empurra o problema um passo para trás.
//
// orderSubtotal é o valor dos itens (sem frete) e serve ao plano basic, cujo
// frete grátis depende de um mínimo de compra.
func computeDeliveryFee(distance float32, establishmentID int64, userID *uint, orderSubtotal float64) (deliveryFee, error) {
	if establishmentID == 0 {
		establishmentID = 1 // matriz
	}

	var delivery models.Delivery
	if err := models.DB.Where("establishment_id = ?", establishmentID).First(&delivery).Error; err != nil {
		return deliveryFee{}, fmt.Errorf("configuração de entrega do estabelecimento %d: %w", establishmentID, err)
	}

	baseDeliveryValue := (distance * delivery.PerKm) + delivery.FixedTaxa
	result := deliveryFee{
		Value:     float64(baseDeliveryValue),
		BaseValue: float64(baseDeliveryValue),
	}

	if userID == nil || *userID == 0 {
		return result, nil
	}

	// Assinatura ativa que concede frete grátis. subscriptions vive no mesmo
	// Postgres (banco único), então dá para consultar daqui.
	//
	// As datas são lidas como time.Time, NÃO como texto. A versão anterior
	// fazia `current_period_start::text` e tentava dar parse com dois layouts
	// ("2006-01-02T15:04:05Z" e "2006-01-02 15:04:05"). A coluna é time.Time
	// no modelo, ou seja timestamptz no banco, e o ::text produz
	// "2026-09-08 13:41:07.123456+00" — que não casa com nenhum dos dois. O
	// parse falhava SEMPRE, e o fallback era `assume vigente`: na prática
	// qualquer assinatura com status 'active' dava frete grátis eternamente,
	// mesmo com o período vencido há meses. Sem string no meio, o problema
	// deixa de existir.
	type SubscriptionCheck struct {
		Plan               string
		Status             string
		FreeDeliveryAbove  float64
		CurrentPeriodStart time.Time
		CurrentPeriodEnd   time.Time
	}
	var sub SubscriptionCheck
	if err := models.DB.Table("subscriptions").
		Where("user_id = ? AND status = 'active'", *userID).
		Select("plan, status, free_delivery_above, current_period_start, current_period_end").
		Scan(&sub).Error; err != nil || sub.Status != "active" {
		return result, nil
	}

	// Período zerado = assinatura sem ciclo definido; tratada como vigente,
	// que era a intenção do fallback original.
	now := time.Now()
	if !sub.CurrentPeriodEnd.IsZero() && now.After(sub.CurrentPeriodEnd) {
		return result, nil
	}
	if !sub.CurrentPeriodStart.IsZero() && now.Before(sub.CurrentPeriodStart) {
		return result, nil
	}

	switch sub.Plan {
	case "premium":
		// Premium: frete grátis sempre.
		result.Value = 0
		result.SubscriptionDiscount = true
	case "basic":
		// Basic: frete grátis acima do valor mínimo.
		if sub.FreeDeliveryAbove > 0 && orderSubtotal >= sub.FreeDeliveryAbove {
			result.Value = 0
			result.SubscriptionDiscount = true
		}
	}

	return result, nil
}

// CalculateDeliveryValue é a cotação do frete usada pelo app antes de fechar o
// pedido. Wrapper HTTP sobre computeDeliveryFee — a regra mora lá, para não
// divergir do que a criação do pedido calcula.
func CalculateDeliveryValue(c *fiber.Ctx) error {
	var request struct {
		Distance        float32 `json:"distance"`
		EstablishmentID int64   `json:"establishmentId"`
		UserID          *uint   `json:"user_id,omitempty"`     // opcional: para verificar frete gratis da assinatura
		OrderTotal      float64 `json:"order_total,omitempty"` // valor total do pedido para frete gratis
	}

	if err := c.BodyParser(&request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Failed to parse request body",
		})
	}

	fee, err := computeDeliveryFee(request.Distance, request.EstablishmentID, request.UserID, request.OrderTotal)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch delivery settings",
		})
	}

	return c.JSON(fiber.Map{
		"deliveryValue":        fee.Value,
		"baseDeliveryValue":    fee.BaseValue,
		"subscriptionDiscount": fee.SubscriptionDiscount,
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
		OriginLat       float64 `json:"origin_lat"`
		OriginLng       float64 `json:"origin_lng"`
		DestLat         float64 `json:"dest_lat"`
		DestLng         float64 `json:"dest_lng"`
		EstablishmentID int64   `json:"establishmentId"`
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
		"distance_km":    fmt.Sprintf("%.2f", distanceKm),
		"duration_min":   fmt.Sprintf("%.1f", durationMin),
		"delivery_value": deliveryValue,
		"source":         source,
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
