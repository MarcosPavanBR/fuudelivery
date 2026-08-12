// AbacatePay Integration
// =====================
// 1. Sign up at https://abacatepay.com
// 2. Get your API key from Dashboard > API
// 3. Set ABACATE_PAY_API_KEY in environment
// 4. Set webhook URL in Dashboard > Webhooks: https://your-app.com/api/payment/webhook
// 5. For card tokenization, use AbacatePay JS SDK on frontend
//
// NOTA (2026-08): a API v2 usa /v2/transparents/create para cobranças PIX.
// O endpoint antigo /v1/charge/pix foi descontinuado e retorna "Not found".

package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"
)

type AbacatePayClient struct {
	APIKey  string
	BaseURL string
}

// PIXChargeRequest é o corpo da cobrança PIX (v2 /transparents/create).
// Amount em centavos. Customer é ponteiro: omitempty não omite structs
// aninhados, e o gateway rejeita "customer":{} vazio com 422.
type PIXChargeRequest struct {
	Method string `json:"method"`
	Data   struct {
		Amount      int64        `json:"amount"`
		Description string       `json:"description,omitempty"`
		ExternalID  string       `json:"externalId,omitempty"`
		Customer    *PIXCustomer `json:"customer,omitempty"`
	} `json:"data"`
}

// PIXCustomer são os dados do pagador. Para PIX é opcional; se enviado,
// todos os campos (incl. taxId/CPF válido) são obrigatórios.
type PIXCustomer struct {
	Name      string `json:"name,omitempty"`
	TaxID     string `json:"taxId,omitempty"`
	Email     string `json:"email,omitempty"`
	Cellphone string `json:"cellphone,omitempty"`
}

// PIXChargeResponse mantém os campos usados pelo monolito.
// brCode -> CopyPaste, brCodeBase64 -> QRCodeBase64 (base64 puro).
type PIXChargeResponse struct {
	ID           string  `json:"id"`
	Status       string  `json:"status"`
	QRCode       string  `json:"qr_code"`
	QRCodeBase64 string  `json:"qr_code_base64"`
	CopyPaste    string  `json:"copy_paste"`
	ExpiresAt    string  `json:"expires_at"`
	Amount       float64 `json:"amount"`
	// ExpiresInSeconds calculado a partir de ExpiresAt (conveniência).
	ExpiresInSeconds int64 `json:"expires_in"`
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
		BaseURL: "https://api.abacatepay.com/v2",
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

	client := &http.Client{Timeout: 30 * time.Second}
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

// apiEnvelope é o wrapper padrão das respostas v2: {"success": bool, "data": ...}
type apiEnvelope struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Error   *string         `json:"error"`
}

// CreatePIXCharge cria uma cobrança PIX transparente e devolve o QR Code.
// amount é em centavos.
func (c *AbacatePayClient) CreatePIXCharge(req PIXChargeRequest) (*PIXChargeResponse, error) {
	req.Method = "PIX"
	body, err := c.doRequest("POST", "/transparents/create", req)
	if err != nil {
		return nil, err
	}

	var env apiEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, err
	}
	if !env.Success {
		msg := "unknown error"
		if env.Error != nil {
			msg = *env.Error
		}
		return nil, fmt.Errorf("abacatepay create pix failed: %s", msg)
	}

	// Resposta v2: {id, amount, status, brCode, brCodeBase64, expiresAt, ...}
	var raw struct {
		ID           string  `json:"id"`
		Amount       int64   `json:"amount"`
		Status       string  `json:"status"`
		BRCode       string  `json:"brCode"`
		BRCodeBase64 string  `json:"brCodeBase64"`
		ExpiresAt    string  `json:"expiresAt"`
		PlatformFee  int64   `json:"platformFee"`
		ReceiptURL   *string `json:"receiptUrl"`
		CreatedAt    string  `json:"createdAt"`
		UpdatedAt    string  `json:"updatedAt"`
	}
	if err := json.Unmarshal(env.Data, &raw); err != nil {
		return nil, err
	}

	// brCodeBase64 vem com prefixo "data:image/png;base64," — o frontend
	// (PIXQRCode.tsx) espera o base64 puro.
	base64Pure := raw.BRCodeBase64
	if idx := strings.Index(base64Pure, "base64,"); idx >= 0 {
		base64Pure = base64Pure[idx+len("base64,"):]
	}

	resp := &PIXChargeResponse{
		ID:           raw.ID,
		Status:       raw.Status,
		QRCode:       raw.BRCode, // copia-e-cola (compatibilidade)
		CopyPaste:    raw.BRCode, // código copia-e-cola
		QRCodeBase64: base64Pure, // base64 puro da imagem
		ExpiresAt:    raw.ExpiresAt,
		Amount:       float64(raw.Amount),
	}

	if raw.ExpiresAt != "" {
		if t, perr := time.Parse(time.RFC3339, raw.ExpiresAt); perr == nil {
			resp.ExpiresInSeconds = int64(time.Until(t).Seconds())
			if resp.ExpiresInSeconds < 0 {
				resp.ExpiresInSeconds = 0
			}
		}
	}

	return resp, nil
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
	body, err := c.doRequest("GET", "/transparents/check?id="+chargeID, nil)
	if err != nil {
		return nil, err
	}

	// Desembrulha o envelope v2: {success, data: {id, status, expiresAt}, error}
	var env apiEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, err
	}
	if !env.Success {
		msg := "unknown error"
		if env.Error != nil {
			msg = *env.Error
		}
		return nil, fmt.Errorf("abacatepay check failed: %s", msg)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(env.Data, &data); err != nil {
		return nil, err
	}
	return data, nil
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
