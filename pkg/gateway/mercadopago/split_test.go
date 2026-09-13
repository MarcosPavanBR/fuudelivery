package mercadopago

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/carloshomar/fuudelivery/pkg/gateway"
)

// clienteApontadoPara monta um Client de teste falando com o httptest server,
// usando o token da PLATAFORMA como default (o que o split precisa sobrescrever).
func clienteApontadoPara(url string) *Client {
	return &Client{
		accessToken: "PLATFORM-TOKEN",
		baseURL:     url,
		httpClient:  &http.Client{},
		maxRetries:  1,
		retryDelay:  0,
	}
}

// TestCreateTransaction_SplitNaOrigem é a garantia central da Fase 2: quando a
// cobrança carrega o token do vendedor e a application_fee, o request ao MP
// precisa (1) ir autenticado como o VENDEDOR, não a plataforma, e (2) levar
// application_fee. Errar (1) manda o dinheiro pra conta errada; errar (2) faz a
// plataforma não receber a comissão.
func TestCreateTransaction_SplitNaOrigem(t *testing.T) {
	var gotAuth string
	var gotBody map[string]interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":12345,"status":"pending",
			"point_of_interaction":{"transaction_data":{"qr_code":"PIXCODE","qr_code_base64":"b64"}}}`))
	}))
	defer srv.Close()

	gw := &MercadoPagoGateway{client: clienteApontadoPara(srv.URL)}

	_, err := gw.CreateTransaction(context.Background(), &gateway.TransactionRequest{
		OrderID:             77,
		Amount:              10000, // R$100,00
		PaymentMethod:       gateway.MethodPIX,
		SellerAccessToken:   "SELLER-TOKEN-abc",
		ApplicationFeeCents: 1500, // R$15,00 de comissão
	})
	if err != nil {
		t.Fatalf("CreateTransaction: %v", err)
	}

	// (1) autenticado como o vendedor, não a plataforma.
	if gotAuth != "Bearer SELLER-TOKEN-abc" {
		t.Errorf("Authorization: obtive %q, queria o token do vendedor", gotAuth)
	}

	// (2) application_fee em reais (15.00), não centavos.
	fee, ok := gotBody["application_fee"]
	if !ok {
		t.Fatal("application_fee ausente no request — a plataforma não receberia a comissão")
	}
	if f, _ := fee.(float64); f != 15.00 {
		t.Errorf("application_fee: obtive %v, queria 15.00", fee)
	}

	// valor total em reais.
	if amt, _ := gotBody["transaction_amount"].(float64); amt != 100.00 {
		t.Errorf("transaction_amount: obtive %v, queria 100.00", amt)
	}
}

// TestCreateTransaction_SemSeller_UsaPlataforma: sem token do vendedor, a
// cobrança segue no modelo antigo (conta da plataforma) e SEM application_fee.
// Garante que o split é opt-in e não quebra o caminho atual.
func TestCreateTransaction_SemSeller_UsaPlataforma(t *testing.T) {
	var gotAuth string
	var gotBody map[string]interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		_, _ = w.Write([]byte(`{"id":1,"status":"pending"}`))
	}))
	defer srv.Close()

	gw := &MercadoPagoGateway{client: clienteApontadoPara(srv.URL)}

	_, err := gw.CreateTransaction(context.Background(), &gateway.TransactionRequest{
		OrderID:       1,
		Amount:        5000,
		PaymentMethod: gateway.MethodPIX,
		// sem SellerAccessToken, sem ApplicationFeeCents
	})
	if err != nil {
		t.Fatalf("CreateTransaction: %v", err)
	}

	if gotAuth != "Bearer PLATFORM-TOKEN" {
		t.Errorf("sem vendedor, devia usar o token da plataforma; obtive %q", gotAuth)
	}
	if _, temFee := gotBody["application_fee"]; temFee {
		t.Error("sem split, não pode ir application_fee no request")
	}
}
