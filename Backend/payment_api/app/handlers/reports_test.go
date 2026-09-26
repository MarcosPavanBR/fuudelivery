package handlers

import (
	"testing"
	"time"
)

func TestAggregateOrderReport(t *testing.T) {
	loc := time.FixedZone("BRT", -3*3600)
	at := func(s string) time.Time { v, _ := time.Parse(time.RFC3339, s); return v }
	rows := []reportOrderRow{
		// 25/09 23:30 BRT = 26/09 02:30 UTC: conta no dia 25 (fuso da loja).
		{Status: "FINISHED", CreatedAt: at("2026-09-26T02:30:00Z"), Payload: []byte(`{"order_total":61.5,"deliveryValue":7,"paymentMethod":{"type":"money"}}`)},
		{Status: "FINISHED", CreatedAt: at("2026-09-26T15:00:00Z"), Payload: []byte(`{"order_total":38.5,"deliveryValue":5,"paymentMethod":{"type":"pix"}}`)},
		{Status: "PREPARING", CreatedAt: at("2026-09-26T16:00:00Z"), Payload: []byte(`{"order_total":20}`)},
		{Status: "DENIED", CreatedAt: at("2026-09-26T16:00:00Z"), Payload: []byte(`{"order_total":99}`)},
	}
	r := aggregateOrderReport(rows, loc)
	if r.TotalRevenue != 100 || r.DeliveryRevenue != 12 || r.Delivered != 2 || r.Pending != 1 || r.Cancelled != 1 || r.AvgTicket != 50 {
		t.Fatalf("totais errados: %+v", r)
	}
	if len(r.RevenueByDay) != 2 || r.RevenueByDay[0]["date"] != "2026-09-25" || r.RevenueByDay[0]["revenue"] != 61.5 {
		t.Fatalf("dias errados (fuso da loja): %+v", r.RevenueByDay)
	}
	if r.ByPayment["money"]["count"] != 1 || r.ByPayment["pix"]["revenue"] != 38.5 {
		t.Fatalf("por forma de pagamento: %+v", r.ByPayment)
	}
}
