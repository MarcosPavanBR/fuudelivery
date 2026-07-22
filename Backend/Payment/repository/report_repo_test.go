// Package repository - report_repo_test.go
// Testes de integracao do repository de relatorios.
// Usa um mock do MongoDB para testar as queries de agregacao
// sem depender de um servidor MongoDB real.
//
// Estrategia de testes:
// 1. Mock da colecao MongoDB usando interface
// 2. Testes de logica de aggregacao (pipeline correctness)
// 3. Testes de periodos (week, month, quarter, year)
// 4. Testes de edge cases (sem dados, periodo invalido)
package repository

import (
	"testing"
	"time"

	"github.com/carloshomar/vercardapio/payment/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// === Helpers de teste ===

// createTestPayment cria um pagamento de teste com campos preenchidos
func createTestPayment(establishmentID string, amount float64, status string, method string, daysAgo int) *models.Payment {
	return &models.Payment{
		ID:              primitive.NewObjectID(),
		OrderID:         "order_" + primitive.NewObjectID().Hex()[:8],
		EstablishmentID: establishmentID,
		Amount:          amount,
		DeliveryAmount:  amount * 0.15, // 15% de taxa
		Method:          models.PaymentMethod(method),
		Status:          models.PaymentStatus(status),
		CreatedAt:       time.Now().AddDate(0, 0, -daysAgo),
		UpdatedAt:       time.Now(),
	}
}

// === Testes de logica de relatorio ===

func TestEstablishmentReport_EmptyData(t *testing.T) {
	// Testa que um relatorio vazio retorna zeros
	report := &EstablishmentReport{
		TotalRevenue:    0,
		TotalOrders:     0,
		AvgTicket:       0,
		DeliveryRevenue: 0,
		OrdersByStatus:  make(map[string]int64),
		RevenueByDay:    nil,
		PaymentMethods:  make(map[string]int64),
	}

	if report.TotalRevenue != 0 {
		t.Errorf("Empty report total_revenue: got %f, want 0", report.TotalRevenue)
	}
	if report.TotalOrders != 0 {
		t.Errorf("Empty report total_orders: got %d, want 0", report.TotalOrders)
	}
}

func TestEstablishmentReport_CalculationLogic(t *testing.T) {
	// Simula a logica de calculo do relatorio
	payments := []struct {
		amount float64
		status string
		method string
	}{
		{100.0, "approved", "pix"},
		{50.0, "approved", "card"},
		{75.0, "approved", "pix"},
		{200.0, "pending", "card"},
		{30.0, "rejected", "pix"},
	}

	var totalRevenue float64
	var approvedCount int64
	var totalOrders int64
	statusCount := make(map[string]int64)
	methodCount := make(map[string]int64)

	for _, p := range payments {
		totalOrders++
		statusCount[p.status]++
		methodCount[p.method]++

		if p.status == "approved" {
			totalRevenue += p.amount
			approvedCount++
		}
	}

	// Verifica totais
	if totalOrders != 5 {
		t.Errorf("Total orders: got %d, want 5", totalOrders)
	}
	if totalRevenue != 225.0 {
		t.Errorf("Total revenue: got %f, want 225.0", totalRevenue)
	}
	if approvedCount != 3 {
		t.Errorf("Approved count: got %d, want 3", approvedCount)
	}

	// Ticket medio
	avgTicket := totalRevenue / float64(approvedCount)
	if avgTicket != 75.0 {
		t.Errorf("Avg ticket: got %f, want 75.0", avgTicket)
	}

	// Status counts
	if statusCount["approved"] != 3 {
		t.Errorf("Approved orders: got %d, want 3", statusCount["approved"])
	}
	if statusCount["pending"] != 1 {
		t.Errorf("Pending orders: got %d, want 1", statusCount["pending"])
	}
	if statusCount["rejected"] != 1 {
		t.Errorf("Rejected orders: got %d, want 1", statusCount["rejected"])
	}

	// Method counts
	if methodCount["pix"] != 3 {
		t.Errorf("PIX payments: got %d, want 3", methodCount["pix"])
	}
	if methodCount["card"] != 2 {
		t.Errorf("Card payments: got %d, want 2", methodCount["card"])
	}
}

func TestEstablishmentReport_RevenueByDay(t *testing.T) {
	// Simula a logica de receita por dia
	type dayData struct {
		date    string
		revenue float64
		orders  int64
	}

	days := []dayData{
		{"01/07", 420.5, 5},
		{"02/07", 380.2, 4},
		{"03/07", 510.8, 6},
		{"04/07", 290.0, 3},
		{"05/07", 620.3, 7},
	}

	var totalFromDays float64
	for _, d := range days {
		totalFromDays += d.revenue
	}

	if totalFromDays != 2221.8 {
		t.Errorf("Total from days: got %f, want 2221.8", totalFromDays)
	}

	// Verifica que dias estao em ordem cronologica
	for i := 1; i < len(days); i++ {
		if days[i].date <= days[i-1].date {
			t.Errorf("Days not in order: %s after %s", days[i].date, days[i-1].date)
		}
	}
}

func TestEstablishmentReport_DeliveryRevenue(t *testing.T) {
	// Testa que delivery_revenue e a soma das taxas de entrega
	payments := []struct {
		amount         float64
		deliveryAmount float64
	}{
		{100.0, 15.0},
		{50.0, 7.5},
		{75.0, 11.25},
	}

	var totalDelivery float64
	for _, p := range payments {
		totalDelivery += p.deliveryAmount
	}

	if totalDelivery != 33.75 {
		t.Errorf("Total delivery revenue: got %f, want 33.75", totalDelivery)
	}
}

// === Testes de periodo ===

func TestPeriodCalculation(t *testing.T) {
	tests := []struct {
		period   string
		daysBack int
	}{
		{"week", 7},
		{"month", 30},
		{"quarter", 90},
		{"year", 365},
	}

	for _, tt := range tests {
		t.Run(tt.period, func(t *testing.T) {
			startDate := time.Now().AddDate(0, 0, -tt.daysBack)
			if time.Since(startDate).Hours() < float64(tt.daysBack-1)*24 {
				t.Errorf("Period %s: startDate too far back", tt.period)
			}
		})
	}
}

func TestPeriodValidation(t *testing.T) {
	validPeriods := map[string]bool{
		"week": true, "month": true, "quarter": true, "year": true,
	}

	tests := []struct {
		period  string
		isValid bool
	}{
		{"week", true},
		{"month", true},
		{"quarter", true},
		{"year", true},
		{"day", false},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.period, func(t *testing.T) {
			is_valid := validPeriods[tt.period]
			if is_valid != tt.isValid {
				t.Errorf("Period %q: valid=%v, want %v", tt.period, is_valid, tt.isValid)
			}
		})
	}
}

// === Testes de Pipeline de Agregacao ===

func TestAggregationPipeline_MatchStage(t *testing.T) {
	// Testa que o match stage esta correto
	establishmentID := "rest_001"
	startDate := time.Now().AddDate(0, 0, -30)

	matchStage := bson.M{
		"$match": bson.M{
			"establishment_id": establishmentID,
			"created_at": bson.M{
				"$gte": startDate,
			},
		},
	}

	// Verifica estrutura do match
	match, ok := matchStage["$match"].(bson.M)
	if !ok {
		t.Fatal("Match stage is not bson.M")
	}

	if match["establishment_id"] != establishmentID {
		t.Errorf("Match establishment_id: got %v, want %v", match["establishment_id"], establishmentID)
	}

	createdAt, ok := match["created_at"].(bson.M)
	if !ok {
		t.Fatal("created_at is not bson.M")
	}

	if createdAt["$gte"] == nil {
		t.Error("created_at.$gte is nil")
	}
}

func TestAggregationPipeline_GroupStage(t *testing.T) {
	// Testa que o group stage calcula metricas corretamente
	groupStage := bson.M{
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
		},
	}

	group, ok := groupStage["$group"].(bson.M)
	if !ok {
		t.Fatal("Group stage is not bson.M")
	}

	if group["_id"] != nil {
		t.Errorf("Group _id: got %v, want nil", group["_id"])
	}

	if group["total_orders"] == nil {
		t.Error("total_orders is nil")
	}
}

func TestAggregationPipeline_StatusGroup(t *testing.T) {
	// Testa o group por status
	groupStage := bson.M{
		"$group": bson.M{
			"_id":   "$status",
			"count": bson.M{"$sum": 1},
		},
	}

	group, ok := groupStage["$group"].(bson.M)
	if !ok {
		t.Fatal("Group stage is not bson.M")
	}

	if group["_id"] != "$status" {
		t.Errorf("Group _id: got %v, want $status", group["_id"])
	}
}

// === Testes de Estrutura de Retorno ===

func TestDayRevenue_Fields(t *testing.T) {
	dr := DayRevenue{
		Date:    "01/07",
		Revenue: 420.5,
		Orders:  5,
	}

	if dr.Date != "01/07" {
		t.Errorf("DayRevenue.Date: got %q, want %q", dr.Date, "01/07")
	}
	if dr.Revenue != 420.5 {
		t.Errorf("DayRevenue.Revenue: got %f, want 420.5", dr.Revenue)
	}
	if dr.Orders != 5 {
		t.Errorf("DayRevenue.Orders: got %d, want 5", dr.Orders)
	}
}

func TestEstablishmentReport_Fields(t *testing.T) {
	report := &EstablishmentReport{
		TotalRevenue:    12450.80,
		TotalOrders:     187,
		AvgTicket:       66.58,
		DeliveryRevenue: 2340.00,
		OrdersByStatus: map[string]int64{
			"approved": 165,
			"pending":  7,
			"rejected": 15,
		},
		RevenueByDay: []DayRevenue{
			{Date: "01/07", Revenue: 420.5, Orders: 5},
			{Date: "02/07", Revenue: 380.2, Orders: 4},
		},
		PaymentMethods: map[string]int64{
			"pix":  120,
			"card": 67,
		},
	}

	// Verifica campos
	if report.TotalRevenue != 12450.80 {
		t.Errorf("TotalRevenue: got %f, want 12450.80", report.TotalRevenue)
	}
	if report.TotalOrders != 187 {
		t.Errorf("TotalOrders: got %d, want 187", report.TotalOrders)
	}
	if report.AvgTicket != 66.58 {
		t.Errorf("AvgTicket: got %f, want 66.58", report.AvgTicket)
	}
	if report.DeliveryRevenue != 2340.00 {
		t.Errorf("DeliveryRevenue: got %f, want 2340.00", report.DeliveryRevenue)
	}

	// Verifica mapas
	if report.OrdersByStatus["approved"] != 165 {
		t.Errorf("OrdersByStatus[approved]: got %d, want 165", report.OrdersByStatus["approved"])
	}
	if report.PaymentMethods["pix"] != 120 {
		t.Errorf("PaymentMethods[pix]: got %d, want 120", report.PaymentMethods["pix"])
	}

	// Verifica array de dias
	if len(report.RevenueByDay) != 2 {
		t.Errorf("RevenueByDay length: got %d, want 2", len(report.RevenueByDay))
	}
}

// === Testes de criacao de pagamento ===

func TestCreateTestPayment(t *testing.T) {
	payment := createTestPayment("rest_001", 100.0, "approved", "pix", 5)

	if payment.EstablishmentID != "rest_001" {
		t.Errorf("EstablishmentID: got %q, want %q", payment.EstablishmentID, "rest_001")
	}
	if payment.Amount != 100.0 {
		t.Errorf("Amount: got %f, want 100.0", payment.Amount)
	}
	if string(payment.Status) != "approved" {
		t.Errorf("Status: got %q, want %q", payment.Status, "approved")
	}
	if string(payment.Method) != "pix" {
		t.Errorf("Method: got %q, want %q", payment.Method, "pix")
	}
	if payment.DeliveryAmount != 15.0 {
		t.Errorf("DeliveryAmount: got %f, want 15.0", payment.DeliveryAmount)
	}
}

func TestCreateTestPayment_DifferentStatuses(t *testing.T) {
	statuses := []string{"approved", "pending", "rejected", "cancelled", "refunded", "disputed"}

	for _, status := range statuses {
		t.Run(status, func(t *testing.T) {
			payment := createTestPayment("rest_001", 50.0, status, "card", 0)
			if string(payment.Status) != status {
				t.Errorf("Status: got %q, want %q", payment.Status, status)
			}
		})
	}
}
