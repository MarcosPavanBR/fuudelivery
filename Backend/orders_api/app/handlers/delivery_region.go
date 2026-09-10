package handlers

// delivery_region.go — CRUD das faixas de CEP que definem o frete.
//
// Todas as rotas são `adminRequired` no monolito: quem define o preço do frete
// é o dono da plataforma, não o restaurante nem o cliente. Uma região é
// dinheiro em todo pedido que casar com ela.

import (
	"strings"

	"github.com/carloshomar/fuudelivery/orders_api/app/models"
	"github.com/gofiber/fiber/v2"
)

type regionRequest struct {
	Name     string  `json:"name"`
	CepStart string  `json:"cep_start"`
	CepEnd   string  `json:"cep_end"`
	City     string  `json:"city"`
	UF       string  `json:"uf"`
	Fee      float64 `json:"fee"`
	Priority int     `json:"priority"`
	Active   *bool   `json:"active"`
}

// validar devolve a regra pronta ou a mensagem de erro para o admin.
//
// As mesmas travas existem em CHECK no banco (sql/24). Aqui elas viram
// mensagem legível em vez de erro de constraint.
func (r regionRequest) validar() (models.DeliveryRegionFee, string) {
	nome := strings.TrimSpace(r.Name)
	if nome == "" {
		return models.DeliveryRegionFee{}, "Informe o nome da região"
	}

	inicio := models.NormalizeCep(r.CepStart)
	fim := models.NormalizeCep(r.CepEnd)
	if inicio == 0 || fim == 0 {
		return models.DeliveryRegionFee{}, "CEP inicial e final precisam ter 8 dígitos"
	}
	if inicio > fim {
		return models.DeliveryRegionFee{}, "O CEP inicial tem de ser menor ou igual ao final"
	}
	// Frete negativo viraria crédito para o cliente dentro do split.
	if r.Fee < 0 {
		return models.DeliveryRegionFee{}, "O valor do frete não pode ser negativo"
	}

	prioridade := r.Priority
	if prioridade <= 0 {
		prioridade = 100
	}
	ativo := true
	if r.Active != nil {
		ativo = *r.Active
	}

	return models.DeliveryRegionFee{
		Name:     nome,
		CepStart: inicio,
		CepEnd:   fim,
		City:     strings.TrimSpace(r.City),
		UF:       strings.ToUpper(strings.TrimSpace(r.UF)),
		Fee:      r.Fee,
		Priority: prioridade,
		Active:   ativo,
	}, ""
}

// ListDeliveryRegions devolve todas as regiões, da mais específica para a mais
// ampla — a mesma ordem que a resolução usa, para a tela mostrar quem ganha de
// quem.
func ListDeliveryRegions(c *fiber.Ctx) error {
	var regioes []models.DeliveryRegionFee
	if err := models.DB.
		Order("priority ASC, (cep_end - cep_start) ASC, id ASC").
		Find(&regioes).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Erro ao listar regiões de entrega",
		})
	}
	return c.JSON(regioes)
}

func CreateDeliveryRegion(c *fiber.Ctx) error {
	var req regionRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Corpo inválido"})
	}

	regra, msg := req.validar()
	if msg != "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": msg})
	}
	if err := models.DB.Create(&regra).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Erro ao criar região de entrega",
		})
	}
	return c.Status(fiber.StatusCreated).JSON(regra)
}

func UpdateDeliveryRegion(c *fiber.Ctx) error {
	var existente models.DeliveryRegionFee
	if err := models.DB.First(&existente, c.Params("id")).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Região não encontrada"})
	}

	var req regionRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Corpo inválido"})
	}
	regra, msg := req.validar()
	if msg != "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": msg})
	}
	regra.ID = existente.ID

	// Select explícito nos campos: `Updates` com struct pula os zero-values, e
	// é assim que um "desativar" (Active=false) ou um "frete grátis nesta
	// região" (Fee=0) seriam silenciosamente ignorados.
	if err := models.DB.Model(&existente).
		Select("name", "cep_start", "cep_end", "city", "uf", "fee", "priority", "active").
		Updates(regra).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Erro ao atualizar região de entrega",
		})
	}
	return c.JSON(regra)
}

func DeleteDeliveryRegion(c *fiber.Ctx) error {
	var existente models.DeliveryRegionFee
	if err := models.DB.First(&existente, c.Params("id")).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Região não encontrada"})
	}
	if err := models.DB.Delete(&existente).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Erro ao remover região de entrega",
		})
	}
	return c.JSON(fiber.Map{"message": "Região removida"})
}
