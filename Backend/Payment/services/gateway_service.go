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

type GatewayService struct {
	APIKey     string
	APIURL     string
	HTTPClient *http.Client
}

func NewGatewayService() *GatewayService {
	return &GatewayService{
		APIKey: getEnv("ABACATE_PAY_API_KEY", ""),
		APIURL: getEnv("ABACATE_PAY_API_URL", "https://api.abacatepay.com"),
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

type CreatePixRequest struct {
	Amount   float64 `json:"amount"`
	CustomerID string `json:"customer_id"`
	CustomerName string `json:"customer_name"`
	CustomerEmail string `json:"customer_email"`
	CustomerPhone string `json:"customer_phone,omitempty"`
	OrderID  string  `json:"order_id"`
}

type CreatePixResponse struct {
	ID       string `json:"id"`
	Status   string `json:"status"`
	PixQRCode string `json:"pix_qr_code"`
	PixCopyPaste string `json:"pix_copy_paste"`
}

func (gs *GatewayService) CreatePixPayment(req *CreatePixRequest) (*CreatePixResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", gs.APIURL+"/v1/billing/create", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+gs.APIKey)

	resp, err := gs.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error: %s", string(respBody))
	}

	var pixResp CreatePixResponse
	if err := json.Unmarshal(respBody, &pixResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &pixResp, nil
}

type GetPixStatusResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

func (gs *GatewayService) GetPixStatus(id string) (*GetPixStatusResponse, error) {
	httpReq, err := http.NewRequest("GET", gs.APIURL+"/v1/billing/get?id="+id, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+gs.APIKey)

	resp, err := gs.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

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
