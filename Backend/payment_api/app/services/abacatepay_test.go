package services

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestCreatePIXCharge_V2Envelope verifica que o client lê o envelope v2
// ({success, data}) e extrai o base64 puro do brCodeBase64 (sem o prefixo
// data:image/png;base64,).
func TestCreatePIXCharge_V2Envelope(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/transparents/create" {
			t.Errorf("path inesperado: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("method inesperado: %s", r.Method)
		}
		// Garante que customer vazio NÃO vai no JSON (omitempty com ponteiro).
		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		data := body["data"].(map[string]interface{})
		if _, hasCustomer := data["customer"]; hasCustomer {
			t.Error("customer presente no JSON com valor vazio — gateway rejeita com 422")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"success": true,
			"data": {
				"id": "pix_char_test123",
				"amount": 100,
				"status": "PENDING",
				"brCode": "0002012658BR.GOV.BCB.PIXabc",
				"brCodeBase64": "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJ",
				"expiresAt": "2026-08-13T15:00:00Z",
				"platformFee": 5,
				"devMode": false
			},
			"error": null
		}`))
	}))
	defer mock.Close()

	client := &AbacatePayClient{APIKey: "abc_prod_test", BaseURL: mock.URL}
	req := PIXChargeRequest{}
	req.Data.Amount = 100
	req.Data.Description = "Pedido 1"

	resp, err := client.CreatePIXCharge(req)
	if err != nil {
		t.Fatalf("CreatePIXCharge falhou: %v", err)
	}

	if resp.ID != "pix_char_test123" {
		t.Errorf("ID = %q, esperava pix_char_test123", resp.ID)
	}
	if resp.Status != "PENDING" {
		t.Errorf("Status = %q, esperava PENDING", resp.Status)
	}
	if resp.CopyPaste != "0002012658BR.GOV.BCB.PIXabc" {
		t.Errorf("CopyPaste = %q, esperava brCode", resp.CopyPaste)
	}
	if resp.QRCode != "0002012658BR.GOV.BCB.PIXabc" {
		t.Errorf("QRCode = %q, esperava brCode (compatibilidade)", resp.QRCode)
	}
	wantBase64 := "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJ"
	if resp.QRCodeBase64 != wantBase64 {
		t.Errorf("QRCodeBase64 = %q, esperava base64 puro %q", resp.QRCodeBase64, wantBase64)
	}
	if resp.Amount != 100 {
		t.Errorf("Amount = %v, esperava 100", resp.Amount)
	}
	if resp.ExpiresInSeconds <= 0 {
		t.Errorf("ExpiresInSeconds = %d, esperava > 0", resp.ExpiresInSeconds)
	}
}

// TestCreatePIXCharge_Error verifica que erro do gateway é propagado.
func TestCreatePIXCharge_Error(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"success":false,"data":null,"error":"Not found"}`))
	}))
	defer mock.Close()

	client := &AbacatePayClient{APIKey: "abc_prod_test", BaseURL: mock.URL}
	_, err := client.CreatePIXCharge(PIXChargeRequest{})
	if err == nil {
		t.Fatal("esperava erro do gateway, recebeu nil")
	}
}

// TestGetCharge_V2Envelope verifica que GetCharge desembrulha o envelope
// e devolve o map do data (com status no topo).
func TestGetCharge_V2Envelope(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/transparents/check" {
			t.Errorf("path inesperado: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("id"); got != "pix_char_x" {
			t.Errorf("query id = %q, esperava pix_char_x", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"success": true,
			"data": {"id": "pix_char_x", "status": "PAID", "expiresAt": "2026-08-13T15:00:00Z"},
			"error": null
		}`))
	}))
	defer mock.Close()

	client := &AbacatePayClient{APIKey: "abc_prod_test", BaseURL: mock.URL}
	data, err := client.GetCharge("pix_char_x")
	if err != nil {
		t.Fatalf("GetCharge falhou: %v", err)
	}

	b, _ := json.Marshal(data)
	var parsed map[string]interface{}
	_ = json.Unmarshal(b, &parsed)
	if parsed["status"] != "PAID" {
		t.Errorf("status = %v, esperava PAID (no topo do map)", parsed["status"])
	}
	if parsed["id"] != "pix_char_x" {
		t.Errorf("id = %v, esperava pix_char_x", parsed["id"])
	}
}
