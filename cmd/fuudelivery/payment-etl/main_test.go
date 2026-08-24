package main

import (
	"encoding/json"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestNormalizeStatus(t *testing.T) {
	cases := map[string]string{
		"PENDING":   "pending",
		"pending":   "pending",
		"CONFIRMED": "approved",
		"APPROVED":  "approved",
		"approved":  "approved",
		"REJECTED":  "rejected",
		"rejected":  "rejected",
		"CANCELLED": "cancelled",
		"REFUNDED":  "refunded",
		"DISPUTED":  "disputed",
		"":          "pending",
		"WEIRD":     "weird",
	}
	for in, want := range cases {
		if got := normalizeStatus(in); got != want {
			t.Errorf("normalizeStatus(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMergeStatusConflictPriority(t *testing.T) {
	cases := []struct {
		a, b     string
		want     string
		conflict bool
	}{
		{"PENDING", "approved", "approved", true},  // mais avançado vence
		{"approved", "PENDING", "approved", true},  // ordem não importa
		{"approved", "refunded", "refunded", true}, // refunded é o mais avançado
		{"CONFIRMED", "rejected", "rejected", true},
		{"refunded", "disputed", "refunded", true},
		{"pending", "pending", "pending", false},
		{"", "pending", "pending", false}, // só B
		{"approved", "", "approved", false},
	}
	for _, c := range cases {
		got, conflict := mergeStatus(c.a, c.b)
		if got != c.want || conflict != c.conflict {
			t.Errorf("mergeStatus(%q, %q) = (%q, %v), want (%q, %v)",
				c.a, c.b, got, conflict, c.want, c.conflict)
		}
	}
}

func aRecord() paymentRecord {
	now := time.Now()
	confirmed := now.Add(-time.Hour)
	return paymentRecord{
		OrderID:         "order-1",
		CustomerID:      10,
		EstablishmentID: 20,
		Amount:          100.0,
		DeliveryAmount:  5.0,
		Method:          "pix",
		Status:          "approved", // veio de CONFIRMED
		PixCopyPaste:    "00020126...",
		AbacatePayID:    "abc-123",
		SplitRules:      []map[string]interface{}{{"receiver_id": int64(20), "receiver_type": "establishment", "amount": 85.0}},
		ConfirmedAt:     &confirmed,
		ApprovedBy:      "admin@fuu",
		CreatedAt:       now.Add(-2 * time.Hour),
		UpdatedAt:       now.Add(-time.Hour),
		RejectionReason: "",
	}
}

func bRecord() paymentRecord {
	now := time.Now()
	approved := now.Add(-30 * time.Minute)
	return paymentRecord{
		OrderID:           "order-1",
		CustomerID:        10,
		CustomerName:      "Cliente Teste",
		CustomerEmail:     "cliente@example.com",
		EstablishmentID:   20,
		EstablishmentName: "Restaurante Teste",
		Amount:            100.0,
		Method:            "pix",
		Status:            "pending", // BP ainda não aprovou
		RiskLevel:         "low",
		RiskScore:         12.5,
		RequiresApproval:  false,
		ApprovedAt:        &approved,
		Reference:         "REF-1",
		Metadata:          map[string]string{"origin": "legacy"},
		CreatedAt:         now.Add(-3 * time.Hour),
		UpdatedAt:         now.Add(-time.Hour),
	}
}

func TestMergePayments_AB(t *testing.T) {
	a, b := aRecord(), bRecord()
	merged := mergePayments(&a, &b, "order-1")

	if merged.Sources != "A+B" {
		t.Errorf("Sources = %q, want A+B", merged.Sources)
	}
	// gateway/cobrança de A
	if merged.PixCopyPaste != a.PixCopyPaste || merged.AbacatePayID != a.AbacatePayID {
		t.Errorf("campos de gateway deveriam vir de A")
	}
	if merged.SplitRules == nil || len(merged.SplitRules) != 1 {
		t.Errorf("split_rules deveria vir de A")
	}
	// risco/compliance de B
	if merged.CustomerName != b.CustomerName || merged.EstablishmentName != b.EstablishmentName {
		t.Errorf("campos de compliance deveriam vir de B")
	}
	if merged.RiskLevel != "low" || merged.RiskScore != 12.5 {
		t.Errorf("risk deveria vir de B")
	}
	if merged.RequiresApproval {
		t.Errorf("requires_approval deveria ser false (de B)")
	}
	if merged.Reference != "REF-1" {
		t.Errorf("reference deveria vir de B")
	}
	// status: approved (A, CONFIRMED) > pending (B) → approved + conflito
	if merged.Status != "approved" {
		t.Errorf("Status = %q, want approved", merged.Status)
	}
	if !merged.StatusConflict {
		t.Errorf("StatusConflict deveria ser true")
	}
	// created_at: o mais antigo (B, -3h)
	if !merged.CreatedAt.Equal(b.CreatedAt) {
		t.Errorf("CreatedAt deveria ser o mais antigo (B)")
	}
	// sem conflito de valor (iguais)
	if merged.AmountConflict {
		t.Errorf("AmountConflict não deveria ocorrer com valores iguais")
	}
}

func TestMergePayments_OnlyA(t *testing.T) {
	a := aRecord()
	merged := mergePayments(&a, nil, "order-1")

	if merged.Sources != "A" {
		t.Errorf("Sources = %q, want A", merged.Sources)
	}
	if merged.Status != "approved" || merged.StatusConflict {
		t.Errorf("status deve vir de A sem conflito")
	}
	if merged.ApprovedBy != "admin@fuu" {
		t.Errorf("approved_by de A deveria ser preservado")
	}
}

func TestMergePayments_AmountConflict(t *testing.T) {
	a, b := aRecord(), bRecord()
	b.Amount = 99.0
	merged := mergePayments(&a, &b, "order-1")
	if !merged.AmountConflict {
		t.Errorf("AmountConflict deveria ser true com valores diferentes")
	}
	if merged.Amount != 100.0 {
		t.Errorf("Amount = %v, want 100 (A vence)", merged.Amount)
	}
}

func TestParseAFromRaw(t *testing.T) {
	now := time.Now()
	raw := bson.M{
		"order_id":       "order-9",
		"customer_id":    int64(7),
		"amount":         42.5,
		"method":         "PIX",
		"status":         "CONFIRMED",
		"split_rules":    primitive.A{bson.M{"receiver_id": int64(1), "receiver_type": "establishment", "amount": 30.0}},
		"created_at":     now,
		"pix_copy_paste": "000201...",
	}
	r := parseA(raw)
	if r.OrderID != "order-9" || r.CustomerID != 7 || r.Amount != 42.5 {
		t.Errorf("parseA básico falhou: %+v", r)
	}
	if r.Method != "pix" || r.Status != "CONFIRMED" {
		t.Errorf("parseA deveria guardar o status cru: method=%q status=%q", r.Method, r.Status)
	}
	if len(r.SplitRules) != 1 {
		t.Errorf("split_rules deveria ter 1 item")
	}
}

func TestParseBStringIDs(t *testing.T) {
	raw := bson.M{
		"order_id":         "order-10",
		"customer_id":      "42",
		"establishment_id": "7",
		"amount":           "35.90",
		"risk_score":       15.0,
		"metadata":         bson.M{"k": "v"},
	}
	r := parseB(raw)
	if r.CustomerID != 42 || r.EstablishmentID != 7 || r.Amount != 35.90 {
		t.Errorf("parseB com IDs string falhou: %+v", r)
	}
	if r.Metadata["k"] != "v" {
		t.Errorf("metadata deveria ser extraída")
	}
}

func TestBuildRowJSON(t *testing.T) {
	a, b := aRecord(), bRecord()
	rec := mergePayments(&a, &b, "order-1")
	row := buildRow(rec)
	var meta map[string]string
	if err := json.Unmarshal(row.Metadata, &meta); err != nil || meta["origin"] != "legacy" {
		t.Errorf("metadata JSON inválida: %s (%v)", row.Metadata, err)
	}
	var split []map[string]interface{}
	if err := json.Unmarshal(row.SplitRules, &split); err != nil || len(split) != 1 {
		t.Errorf("split_rules JSON inválido: %s (%v)", row.SplitRules, err)
	}
}
