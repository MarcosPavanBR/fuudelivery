package handlers

import (
	"encoding/json"
	"sort"
	"strconv"
	"time"

	"github.com/carloshomar/fuudelivery/auth_api/app/middlewares"
	authModels "github.com/carloshomar/fuudelivery/auth_api/app/models"
	"github.com/carloshomar/fuudelivery/payment_api/app/models"
	"github.com/gofiber/fiber/v2"
)

// GetEstablishmentReport retorna o relatório de vendas de um estabelecimento
// (GET /payments/reports/establishment/:id?period=week|month|quarter|year).
//
// Permissão: admin ou o dono do estabelecimento (claim establishment_id).
//
// Fonte: os PEDIDOS (order_documents), não a tabela payments. Antes só
// entravam pagamentos online confirmados: pedido pago em dinheiro/cartão na
// entrega nunca aparecia, e a loja via "R$ 0,00" com pedidos entregues.
// Dias no fuso da loja (o servidor roda em UTC).
//
// A resposta mantém os DOIS formatos (snake_case do DashboardCharts e
// camelCase da página de Relatórios).
func GetEstablishmentReport(c *fiber.Ctx) error {
	estIDStr := c.Params("id")
	estID, err := strconv.ParseInt(estIDStr, 10, 64)
	if err != nil || estID <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid establishment ID"})
	}

	role, _ := middlewares.GetUserRoleFromToken(c)
	if role != "admin" {
		tokenEst, err := middlewares.GetEstablishmentIDFromToken(c)
		if err != nil || tokenEst != estID {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Cannot view another establishment's report"})
		}
	}

	period := c.Query("period", "week")
	loc := authModels.StoreLocation()
	start := reportPeriodStart(period, time.Now().In(loc))

	var rows []reportOrderRow
	if err := models.DB.Table("order_documents").
		Select("status", "created_at", "payload").
		Where("establishment_id = ? AND created_at >= ?", estID, start.UTC()).
		Scan(&rows).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Falha ao consultar pedidos"})
	}

	r := aggregateOrderReport(rows, loc)
	return c.JSON(fiber.Map{
		// Formato do DashboardCharts (snake_case)
		"total_revenue":  r.TotalRevenue,
		"avg_ticket":     r.AvgTicket,
		"revenue_by_day": r.RevenueByDay,
		// Formato da página de Relatórios (camelCase)
		"totalRevenue":     r.TotalRevenue,
		"totalOrders":      r.Delivered,
		"avgTicket":        r.AvgTicket,
		"deliveryRevenue":  r.DeliveryRevenue,
		"ordersByStatus":   fiber.Map{"delivered": r.Delivered, "pending": r.Pending, "cancelled": r.Cancelled},
		"revenueByDay":     r.RevenueByDay,
		"byPayment":        r.ByPayment,
		"period":           period,
		"establishment_id": estID,
	})
}

type reportOrderRow struct {
	Status    string
	CreatedAt time.Time
	Payload   []byte
}

type orderReport struct {
	TotalRevenue    float64
	DeliveryRevenue float64
	AvgTicket       float64
	Delivered       int
	Pending         int
	Cancelled       int
	RevenueByDay    []fiber.Map
	ByPayment       map[string]fiber.Map // "pix" → {count, revenue}
}

// aggregateOrderReport soma os pedidos. Receita = pedidos ENTREGUES
// (FINISHED), pelo total que o cliente pagou (order_total, já com frete e
// cupom). Recusado/cancelado conta à parte; o resto está em andamento.
func aggregateOrderReport(rows []reportOrderRow, loc *time.Location) orderReport {
	var r orderReport
	byDay := map[string]float64{}
	r.ByPayment = map[string]fiber.Map{}
	pay := map[string]*struct {
		n int
		v float64
	}{}
	for _, row := range rows {
		switch row.Status {
		case "DENIED", "CANCELLED":
			r.Cancelled++
			continue
		case "FINISHED":
		default:
			r.Pending++
			continue
		}
		var p struct {
			OrderTotal    float64 `json:"order_total"`
			DeliveryValue float64 `json:"deliveryValue"`
			PaymentMethod struct {
				Type string `json:"type"`
			} `json:"paymentMethod"`
		}
		_ = json.Unmarshal(row.Payload, &p)
		r.Delivered++
		r.TotalRevenue += p.OrderTotal
		r.DeliveryRevenue += p.DeliveryValue
		byDay[row.CreatedAt.In(loc).Format("2006-01-02")] += p.OrderTotal
		kind := p.PaymentMethod.Type
		if kind == "" {
			kind = "outros"
		}
		if pay[kind] == nil {
			pay[kind] = &struct {
				n int
				v float64
			}{}
		}
		pay[kind].n++
		pay[kind].v += p.OrderTotal
	}
	r.TotalRevenue = roundCents(r.TotalRevenue)
	r.DeliveryRevenue = roundCents(r.DeliveryRevenue)
	if r.Delivered > 0 {
		r.AvgTicket = roundCents(r.TotalRevenue / float64(r.Delivered))
	}
	days := make([]string, 0, len(byDay))
	for d := range byDay {
		days = append(days, d)
	}
	sort.Strings(days)
	r.RevenueByDay = make([]fiber.Map, 0, len(days))
	for _, d := range days {
		r.RevenueByDay = append(r.RevenueByDay, fiber.Map{"date": d, "revenue": roundCents(byDay[d])})
	}
	for k, v := range pay {
		r.ByPayment[k] = fiber.Map{"count": v.n, "revenue": roundCents(v.v)}
	}
	return r
}

// reportPeriodStart devolve o início do período informado (semana/mês/trimestre/ano).
func reportPeriodStart(period string, now time.Time) time.Time {
	switch period {
	case "month":
		return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	case "quarter":
		q := (int(now.Month()) - 1) / 3
		return time.Date(now.Year(), time.Month(q*3+1), 1, 0, 0, 0, 0, now.Location())
	case "year":
		return time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location())
	default: // week — começa no domingo
		start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		return start.AddDate(0, 0, -int(now.Weekday()))
	}
}
