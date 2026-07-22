// Package services - gateway_service.go
// Servico de integracao com o gateway de pagamento AbacatePay.
// Fornece metodos para criar cobrancas PIX e consultar status.
package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// GatewayService gerencia a comunicacao com a API do AbacatePay.
type GatewayService struct {
	APIKey     string       // Chave de API do AbacatePay
	APIURL     string       // URL base da API
	HTTPClient *http.Client // Cliente HTTP com timeout
}

// NewGatewayService cria uma nova instancia do servico de gateway.
// Le as configuracoes de API key e URL das variaveis de ambiente.
func NewGatewayService() *GatewayService {
	return &GatewayService{
		APIKey: getEnv("ABACATE_PAY_API_KEY", ""),
		APIURL: getEnv("ABACATE_PAY_API_URL", "https://api.abacatepay.com"),
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// getEnv retorna o valor da variavel de ambiente ou o fallback.
func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

// CreatePixRequest representa os dados para criar uma cobranca PIX.
type CreatePixRequest struct {
	Amount        float64 `json:"amount"`         // Valor em R$
	CustomerID    string  `json:"customer_id"`    // ID do cliente
	CustomerName  string  `json:"customer_name"`  // Nome do cliente
	CustomerEmail string  `json:"customer_email"` // Email do cliente
	CustomerPhone string  `json:"customer_phone,omitempty"` // Telefone (opcional)
	OrderID       string  `json:"order_id"`       // ID do pedido
}

// CreatePixResponse representa a resposta apos criar uma cobranca PIX.
type CreatePixResponse struct {
	ID           string `json:"id"`             // ID da cobranca no AbacatePay
	Status       string `json:"status"`         // Status da cobranca
	PixQRCode    string `json:"pix_qr_code"`   // QR Code em base64
	PixCopyPaste string `json:"pix_copy_paste"` // Codigo PIX para copiar/colar
}

// CreatePixPayment cria uma nova cobranca PIX via API do AbacatePay.
// Retorna o QR Code e o codigo PIX para o cliente efetuar o pagamento.
func (gs *GatewayService) CreatePixPayment(req *CreatePixRequest) (*CreatePixResponse, error) {
	// Serializa o request para JSON
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Cria a requisicao HTTP POST
	httpReq, err := http.NewRequest("POST", gs.APIURL+"/v1/billing/create", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Configura headers (JSON + autenticacao)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+gs.APIKey)

	// Envia a requisicao
	resp, err := gs.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Le o corpo da resposta
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Verifica se a resposta e um erro
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error: %s", string(respBody))
	}

	// Deserializa a resposta
	var pixResp CreatePixResponse
	if err := json.Unmarshal(respBody, &pixResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &pixResp, nil
}

// GetPixStatusResponse representa a resposta ao consultar status de uma cobranca.
type GetPixStatusResponse struct {
	ID     string `json:"id"`     // ID da cobranca
	Status string `json:"status"` // Status: pending, paid, expired, etc
}

// GetPixStatus consulta o status de uma cobranca PIX no AbacatePay.
// Usado para verificar se o pagamento foi efetuado pelo cliente.
func (gs *GatewayService) GetPixStatus(id string) (*GetPixStatusResponse, error) {
	// Cria a requisicao HTTP GET
	httpReq, err := http.NewRequest("GET", gs.APIURL+"/v1/billing/get?id="+id, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Configura header de autenticacao
	httpReq.Header.Set("Authorization", "Bearer "+gs.APIKey)

	// Envia a requisicao
	resp, err := gs.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Le e deserializa a resposta
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var statusResp GetPixStatusResponse
	if err := json.Unmarshal(respBody, &statusResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &statusResp, nil
}
