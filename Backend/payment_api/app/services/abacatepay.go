// AbacatePay Integration
// =====================
// 1. Sign up at https://abacatepay.com
// 2. Get your API key from Dashboard > API
// 3. Set ABACATE_PAY_API_KEY in environment
// 4. Set webhook URL in Dashboard > Webhooks: https://your-app.com/api/payment/webhook
// 5. For card tokenization, use AbacatePay JS SDK on frontend

package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
)

type AbacatePayClient struct {
	APIKey  string
	BaseURL string
}

type PIXChargeRequest struct {
	Amount      float64 `json:"amount"`
	Description string  `json:"description"`
	Customer    struct {
		Name  string `json:"name"`
		Email string `json:"email"`
		Phone string `json:"phone"`
	} `json:"customer"`
}

type PIXChargeResponse struct {
	ID           string  `json:"id"`
	Status       string  `json:"status"`
	QRCode       string  `json:"qr_code"`
	QRCodeBase64 string  `json:"qr_code_base64"`
	CopyPaste    string  `json:"copy_paste"`
	ExpiresAt    string  `json:"expires_at"`
	Amount       float64 `json:"amount"`
}

type CardChargeRequest struct {
	Amount       float64 `json:"amount"`
	Description  string  `json:"description"`
	Installments int     `json:"installments"`
	CardToken    string  `json:"card_token"`
	Customer     struct {
		Name  string `json:"name"`
		Email string `json:"email"`
		Phone string `json:"phone"`
		CPF   string `json:"cpf"`
	} `json:"customer"`
}

type CardChargeResponse struct {
	ID           string  `json:"id"`
	Status       string  `json:"status"`
	Installments int     `json:"installments"`
	Amount       float64 `json:"amount"`
	LastDigits   string  `json:"last_digits"`
}

type BoletoChargeRequest struct {
	Amount      float64 `json:"amount"`
	Description string  `json:"description"`
	Customer    struct {
		Name  string `json:"name"`
		Email string `json:"email"`
		CPF   string `json:"cpf"`
	} `json:"customer"`
	ExpiresIn int `json:"expires_in_days"`
}

type BoletoChargeResponse struct {
	ID         string `json:"id"`
	Status     string `json:"status"`
	BoletoURL  string `json:"boleto_url"`
	BoletoCode string `json:"boleto_code"`
	ExpiresAt  string `json:"expires_at"`
}

type WebhookRegistration struct {
	URL    string   `json:"url"`
	Events []string `json:"events"`
}

type WebhookResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

func NewAbacatePayClient() *AbacatePayClient {
	return &AbacatePayClient{
		APIKey:  os.Getenv("ABACATE_PAY_API_KEY"),
		BaseURL: "https://api.abacatepay.com/v1",
	}
}

func (c *AbacatePayClient) doRequest(method, path string, body interface{}) ([]byte, error) {
	var reqBody []byte
	var err error

	if body != nil {
		reqBody, err = json.Marshal(body)
		if err != nil {
			return nil, err
		}
	}

	url := c.BaseURL + path
	req, err := http.NewRequest(method, url, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("User-Agent", "Fuudelivery/1.0")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("abacatepay request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("abacatepay error %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

func (c *AbacatePayClient) CreatePIXCharge(req PIXChargeRequest) (*PIXChargeResponse, error) {
	body, err := c.doRequest("POST", "/charge/pix", req)
	if err != nil {
		return nil, err
	}

	var resp PIXChargeResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

func (c *AbacatePayClient) CreateCardCharge(req CardChargeRequest) (*CardChargeResponse, error) {
	body, err := c.doRequest("POST", "/charge/card", req)
	if err != nil {
		return nil, err
	}

	var resp CardChargeResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

func (c *AbacatePayClient) CreateBoletoCharge(req BoletoChargeRequest) (*BoletoChargeResponse, error) {
	body, err := c.doRequest("POST", "/charge/boleto", req)
	if err != nil {
		return nil, err
	}

	var resp BoletoChargeResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

func (c *AbacatePayClient) GetCharge(chargeID string) (map[string]interface{}, error) {
	body, err := c.doRequest("GET", "/charge/"+chargeID, nil)
	if err != nil {
		return nil, err
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	return resp, nil
}

func (c *AbacatePayClient) RegisterWebhook(url string, events []string) (*WebhookResponse, error) {
	req := WebhookRegistration{
		URL:    url,
		Events: events,
	}

	body, err := c.doRequest("POST", "/webhook", req)
	if err != nil {
		return nil, err
	}

	var resp WebhookResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}
