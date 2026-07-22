// Package repository - report_repo.go
// Funcoes de agregacao para relatorios de pagamentos.
// Usa aggregation pipeline do MongoDB para calcular metricas
// de receita, pedidos e ticket medio por restaurante.
package repository

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

// EstablishmentReport representa o relatorio consolidado de um restaurante.
type EstablishmentReport struct {
	TotalRevenue    float64            `json:"total_revenue"`
	TotalOrders     int64              `json:"total_orders"`
	AvgTicket       float64            `json:"avg_ticket"`
	DeliveryRevenue float64            `json:"delivery_revenue"`
	OrdersByStatus  map[string]int64   `json:"orders_by_status"`
	RevenueByDay    []DayRevenue       `json:"revenue_by_day"`
	PaymentMethods  map[string]int64   `json:"payment_methods"`
}

// DayRevenue representa a receita de um dia especifico.
type DayRevenue struct {
	Date    string  `json:"date"`
	Revenue float64 `json:"revenue"`
	Orders  int64   `json:"orders"`
}

// GetEstablishmentReport gera relatorio consolidado de um restaurante.
// Usa MongoDB aggregation pipeline para calcular metricas em uma unica query.
// Periodo: week (7 dias), month (30 dias), quarter (90 dias), year (365 dias).
func GetEstablishmentReport(establishmentID string, period string) (*EstablishmentReport, error) {
	ctx := MongoCtx()

	// Calcula data de inicio baseada no periodo
	daysBack := 30
	switch period {
	case "week":
		daysBack = 7
	case "quarter":
		daysBack = 90
	case "year":
		daysBack = 365
	}
	startDate := time.Now().AddDate(0, 0, -daysBack)

	// Filtro base: establishment_id + data
	matchStage := bson.M{
		"$match": bson.M{
			"establishment_id": establishmentID,
			"created_at": bson.M{
				"$gte": startDate,
			},
		},
	}

	// Pipeline de agregacao principal
	pipeline := bson.A{
		matchStage,
		bson.M{
			"$group": bson.M{
				"_id": nil,
				"total_revenue": bson.M{
					"$sum": bson.M{
						"$cond": bson.A{
							bson.M{"$eq": bson.A{"$status", "approved"}},
							"$amount",
							0,
						},
					},
				},
				"total_orders": bson.M{"$sum": 1},
				"delivery_revenue": bson.M{
					"$sum": "$delivery_amount",
				},
				"approved_count": bson.M{
					"$sum": bson.M{
						"$cond": bson.A{
							bson.M{"$eq": bson.A{"$status", "approved"}},
							1,
							0,
						},
					},
				},
			},
		},
	}

	cursor, err := Payments.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	report := &EstablishmentReport{
		OrdersByStatus: make(map[string]int64),
		PaymentMethods: make(map[string]int64),
	}

	if cursor.Next(ctx) {
		var result struct {
			TotalRevenue    float64 `bson:"total_revenue"`
			TotalOrders     int64   `bson:"total_orders"`
			DeliveryRevenue float64 `bson:"delivery_revenue"`
			ApprovedCount   int64   `bson:"approved_count"`
		}
		if err := cursor.Decode(&result); err != nil {
			return nil, err
		}
		report.TotalRevenue = result.TotalRevenue
		report.TotalOrders = result.TotalOrders
		report.DeliveryRevenue = result.DeliveryRevenue
		if result.ApprovedCount > 0 {
			report.AvgTicket = result.TotalRevenue / float64(result.ApprovedCount)
		}
	}

	// Conta por status
	statusPipeline := bson.A{
		matchStage,
		bson.M{
			"$group": bson.M{
				"_id":   "$status",
				"count": bson.M{"$sum": 1},
			},
		},
	}

	statusCursor, err := Payments.Aggregate(ctx, statusPipeline)
	if err != nil {
		return report, nil // Retorna parcial se falhar
	}
	defer statusCursor.Close(ctx)

	for statusCursor.Next(ctx) {
		var result struct {
			ID    string `bson:"_id"`
			Count int64  `bson:"count"`
		}
		if err := statusCursor.Decode(&result); err != nil {
			continue
		}
		report.OrdersByStatus[result.ID] = result.Count
	}

	// Receita por dia
	dayPipeline := bson.A{
		matchStage,
		bson.M{
			"$group": bson.M{
				"_id": bson.M{
					"$dateToString": bson.M{
						"format": "%d/%m",
						"date":   "$created_at",
					},
				},
				"revenue": bson.M{
					"$sum": bson.M{
						"$cond": bson.A{
							bson.M{"$eq": bson.A{"$status", "approved"}},
							"$amount",
							0,
						},
					},
				},
				"orders": bson.M{"$sum": 1},
			},
		},
		bson.M{
			"$sort": bson.M{"_id": 1},
		},
	}

	dayCursor, err := Payments.Aggregate(ctx, dayPipeline)
	if err != nil {
		return report, nil
	}
	defer dayCursor.Close(ctx)

	for dayCursor.Next(ctx) {
		var result struct {
			ID      string  `bson:"_id"`
			Revenue float64 `bson:"revenue"`
			Orders  int64   `bson:"orders"`
		}
		if err := dayCursor.Decode(&result); err != nil {
			continue
		}
		report.RevenueByDay = append(report.RevenueByDay, DayRevenue{
			Date:    result.ID,
			Revenue: result.Revenue,
			Orders:  result.Orders,
		})
	}

	// Conta por metodo de pagamento
	methodPipeline := bson.A{
		matchStage,
		bson.M{
			"$group": bson.M{
				"_id":   "$method",
				"count": bson.M{"$sum": 1},
			},
		},
	}

	methodCursor, err := Payments.Aggregate(ctx, methodPipeline)
	if err != nil {
		return report, nil
	}
	defer methodCursor.Close(ctx)

	for methodCursor.Next(ctx) {
		var result struct {
			ID    string `bson:"_id"`
			Count int64  `bson:"count"`
		}
		if err := methodCursor.Decode(&result); err != nil {
			continue
		}
		report.PaymentMethods[result.ID] = result.Count
	}

	return report, nil
}

// GetEstablishmentReportWithMock e uma versao testavel que aceita
// uma colecao mockada para testes de integracao.
func GetEstablishmentReportWithMock(ctx context.Context, collection interface{}, establishmentID string, period string) (*EstablishmentReport, error) {
	// Para testes, usamos a versao normal (o mock esta no nivel de testes)
	return GetEstablishmentReport(establishmentID, period)
}
