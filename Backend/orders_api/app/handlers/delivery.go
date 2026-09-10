package handlers

import (
	"errors"
	"fmt"
	"log"
	"time"

	authModels "github.com/carloshomar/fuudelivery/auth_api/app/models"
	"github.com/carloshomar/fuudelivery/orders_api/app/dto"
	"github.com/carloshomar/fuudelivery/orders_api/app/models"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// errNoDeliveryConfig sinaliza que o estabelecimento não tem linha em
// `deliveries` — ninguém chamou POST /delivery para ele.
//
// Precisa ser distinguível de um erro de banco porque os dois chamadores
// reagem de forma oposta, e por um motivo concreto: antes desta mudança
// computeOrderTotal NÃO consultava `deliveries` (só somava o frete que o
// cliente mandava), então um estabelecimento sem configuração conseguia
// receber pedido normalmente. Tratar a ausência como erro faria TODO pedido
// desse estabelecimento passar a falhar — troca de um problema de dinheiro
// por uma interrupção de venda.
var errNoDeliveryConfig = errors.New("estabelecimento sem configuração de entrega")

// deliveryFee é o resultado do cálculo do frete.
type deliveryFee struct {
	Value                float64 // o que o cliente paga (0 se a assinatura zerar)
	BaseValue            float64 // antes do desconto de assinatura
	SubscriptionDiscount bool

	// RegionName é a região que definiu o preço, vazia quando o preço veio do
	// fallback por km. Aparece na cotação para o cliente saber por que está
	// pagando aquilo, e no log para o Marcos ver CEP sem cobertura.
	RegionName string
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

// distanciaServidorKm mede do estabelecimento até as coordenadas do pedido.
//
// Reusa authModels.HaversineDistance (auth_api/app/models/zone.go), que já
// existe e é a mesma conta usada pelo motor de despacho — não vale criar uma
// terceira cópia da fórmula.
//
// O ponto importante: as coordenadas do ESTABELECIMENTO vêm do banco, não do
// corpo. Então nem o cliente nem um app desatualizado conseguem encolher a
// distância; o máximo que o corpo faz é dizer para onde entregar, e é para lá
// que o entregador vai.
//
// Devolve 0 quando falta coordenada — nesse caso o preço por região continua
// valendo (não depende de distância), e só o fallback por km fica sem base,
// caindo na taxa fixa do estabelecimento.
func distanciaServidorKm(loc dto.Location, establishmentID int64) float64 {
	if authModels.DB == nil || establishmentID == 0 {
		return 0
	}
	if loc.Coords.Latitude == 0 && loc.Coords.Longitude == 0 {
		return 0
	}

	var est authModels.Establishment
	if err := authModels.DB.Select("lat", "long").First(&est, establishmentID).Error; err != nil {
		log.Printf("[FRETE] estabelecimento %d sem coordenadas para medir distância: %v", establishmentID, err)
		return 0
	}
	if est.Lat == 0 && est.Long == 0 {
		return 0
	}

	return authModels.HaversineDistance(est.Lat, est.Long, loc.Coords.Latitude, loc.Coords.Longitude)
}

// resolveBaseFee decide o preço BASE do frete (antes de assinatura) para um
// endereço.
//
// Ordem, e o porquê de cada degrau:
//
//  1. Região que casa com o CEP. É o caminho normal e o único que não depende
//     de nada que o cliente escolha além do endereço para onde a comida vai.
//  2. Sem região, cai na configuração por km do estabelecimento
//     (fixed_taxa + per_km × distância). Existe só para o período de
//     transição: enquanto o Marcos não cadastrar as regiões da praça, o
//     sistema continua vendendo em vez de parar. A distância aqui é a
//     CALCULADA no servidor, nunca a do corpo.
//
// O que NUNCA acontece: frete zero por falta de configuração. Sem região e sem
// configuração do estabelecimento, o erro sobe e o chamador decide — hoje,
// cobrar zero com log de aviso, que é o comportamento que já existia e não
// piora nada.
func resolveBaseFee(loc dto.Location, establishmentID int64, distanciaKm float64) (float64, string, error) {
	if regra, ok := models.ResolveRegionFee(loc.Cep, loc.Localidade, loc.UF); ok {
		if regra.CepEnd-regra.CepStart > 9000000 {
			// Regra que cobre o país inteiro é a "taxa padrão" que o admin
			// cadastrou como rede de segurança. Funciona, mas significa que
			// este CEP não tem região própria — logar é o que faz alguém
			// cadastrar a faixa certa.
			log.Printf("[FRETE] CEP %s (%s/%s) sem região específica — usando a faixa geral %q",
				loc.Cep, loc.Localidade, loc.UF, regra.Name)
		}
		return regra.Fee, regra.Name, nil
	}

	var delivery models.Delivery
	if err := models.DB.Where("establishment_id = ?", establishmentID).First(&delivery).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, "", errNoDeliveryConfig
		}
		return 0, "", fmt.Errorf("configuração de entrega do estabelecimento %d: %w", establishmentID, err)
	}

	log.Printf("[FRETE] CEP %q sem região cadastrada — caindo na taxa por km do estabelecimento %d",
		loc.Cep, establishmentID)
	return (distanciaKm * float64(delivery.PerKm)) + float64(delivery.FixedTaxa), "", nil
}

// computeDeliveryFee calcula o frete do ENDEREÇO de entrega.
//
// distanciaKm é calculada no SERVIDOR (haversine entre o estabelecimento e as
// coordenadas do pedido) e serve só ao fallback por km; o preço por região não
// depende dela. O campo `distance` que o app manda no corpo é ignorado — era
// ele que permitia pagar só a taxa fixa mandando zero.
func computeDeliveryFee(loc dto.Location, distanciaKm float64, establishmentID int64, userID *uint, orderSubtotal float64) (deliveryFee, error) {
	if establishmentID == 0 {
		establishmentID = 1 // matriz
	}

	base, regiao, err := resolveBaseFee(loc, establishmentID, distanciaKm)
	if err != nil {
		return deliveryFee{}, err
	}

	result := deliveryFee{
		Value:      base,
		BaseValue:  base,
		RegionName: regiao,
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
	// ORDER BY + LIMIT 1: o Scan numa struct única pega a PRIMEIRA linha, e sem
	// ordenação "primeira" é o que o Postgres devolver. Um usuário com mais de
	// uma assinatura ativa (upgrade que não encerrou a anterior, linha
	// duplicada por retry de webhook) tinha o benefício decidido por acaso —
	// podendo perder o frete grátis que pagou. A mais recente é a que vale.
	var sub SubscriptionCheck
	if err := models.DB.Table("subscriptions").
		Where("user_id = ? AND status = 'active'", *userID).
		Select("plan, status, free_delivery_above, current_period_start, current_period_end").
		Order("current_period_end DESC NULLS LAST").
		Limit(1).
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
		// O ENDEREÇO é o que define o preço. `distance` continua sendo aceito
		// para não quebrar app antigo, mas é ignorado — era ele que permitia
		// cotar frete de R$0 mandando zero.
		Location        dto.Location `json:"location"`
		Distance        float32      `json:"distance"` // ignorado; ver acima
		EstablishmentID int64        `json:"establishmentId"`
		UserID          *uint        `json:"user_id,omitempty"`     // opcional: para verificar frete gratis da assinatura
		OrderTotal      float64      `json:"order_total,omitempty"` // valor total do pedido para frete gratis
	}

	if err := c.BodyParser(&request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Failed to parse request body",
		})
	}

	fee, err := computeDeliveryFee(
		request.Location,
		distanciaServidorKm(request.Location, request.EstablishmentID),
		request.EstablishmentID, request.UserID, request.OrderTotal)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch delivery settings",
		})
	}

	return c.JSON(fiber.Map{
		"deliveryValue":        fee.Value,
		"baseDeliveryValue":    fee.BaseValue,
		"subscriptionDiscount": fee.SubscriptionDiscount,
		"region":               fee.RegionName,
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
