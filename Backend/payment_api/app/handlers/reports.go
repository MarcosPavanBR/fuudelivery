package handlers

import (
	"sort"
	"strconv"
	"time"

	"github.com/carloshomar/fuudelivery/auth_api/app/middlewares"
	"github.com/carloshomar/fuudelivery/payment_api/app/models"
	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson"
)

// GetEstablishmentReport retorna o relatório de vendas de um estabelecimento
// agregado a partir dos pagamentos (GET /reports/establishment/:id?period=...).
// period: week | month | quarter | year (default week).
//
// Permissão: admin ou o dono do estabelecimento (claim establishment_id do
// JWT — o dono só vê o relatório do próprio restaurante).
//
// A resposta inclui os DOIS formatos (snake_case para o DashboardCharts e
// camelCase para a página de Relatórios) para compatibilidade com o
// WebRestaurant sem exigir mudança nas duas telas.
func GetEstablishmentReport(c *fiber.Ctx) error {
	estIDStr := c.Params("id")
	estID, err := strconv.ParseInt(estIDStr, 10, 64)
	if err != nil || estID <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid establishment ID"})
	}

	// Admin vê qualquer estabelecimento; usuário comum só o próprio.
	role, _ := middlewares.GetUserRoleFromToken(c)
	if role != "admin" {
		tokenEst, err := middlewares.GetEstablishmentIDFromToken(c)
		if err != nil || tokenEst != estID {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Cannot view another establishment's report"})
		}
	}

	period := c.Query("period", "week")
	now := time.Now()
	start := reportPeriodStart(period, now)

	collection := models.MongoDabase.Collection("payments")
	cursor, err := collection.Find(mongoCtx(), bson.M{
		"establishment_id": estID,
		"created_at":       bson.M{"$gte": start},
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Falha ao consultar pagamentos"})
	}
	defer cursor.Close(mongoCtx())

	var (
		totalRevenue    float64
		deliveryRevenue float64
		avgTicket       float64
		totalOrders     int
		statusCounts    = map[string]int{}
		revenueByDay    = map[string]float64{}
	)

	for cursor.Next(mongoCtx()) {
		var p models.Payment
		if err := cursor.Decode(&p); err != nil {
			continue
		}
		statusCounts[p.Status]++

		ts := p.ConfirmedAt
		if ts == nil {
			ts = &p.CreatedAt
		}
		day := ts.Format("2006-01-02")

		if p.Status == "CONFIRMED" {
			totalRevenue += p.Amount
			deliveryRevenue += p.DeliveryAmount
			totalOrders++
			revenueByDay[day] += p.Amount
		}
	}

	if totalOrders > 0 {
		avgTicket = totalRevenue / float64(totalOrders)
	}

	// Ordena os dias cronologicamente.
	days := make([]string, 0, len(revenueByDay))
	for d := range revenueByDay {
		days = append(days, d)
	}
	sort.Strings(days)

	revenueByDayList := make([]fiber.Map, 0, len(days))
	for _, d := range days {
		revenueByDayList = append(revenueByDayList, fiber.Map{"date": d, "revenue": revenueByDay[d]})
	}

	ordersByStatus := fiber.Map{
		"delivered": statusCounts["CONFIRMED"],
		"pending":   statusCounts["PENDING"],
		"cancelled": statusCounts["REFUNDED"] + statusCounts["EXPIRED"] + statusCounts["CANCELLED"] + statusCounts["REJECTED"],
	}

	return c.JSON(fiber.Map{
		// Formato do DashboardCharts (snake_case)
		"total_revenue":  totalRevenue,
		"avg_ticket":     avgTicket,
		"revenue_by_day": revenueByDayList,
		// Formato da página de Relatórios (camelCase)
		"totalRevenue":     totalRevenue,
		"totalOrders":      totalOrders,
		"avgTicket":        avgTicket,
		"deliveryRevenue":  deliveryRevenue,
		"ordersByStatus":   ordersByStatus,
		"revenueByDay":     revenueByDayList,
		"period":           period,
		"establishment_id": estID,
	})
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
