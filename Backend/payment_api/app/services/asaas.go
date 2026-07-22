// Asaas Split Payment Integration
// ================================
// 1. Sign up at https://www.asaas.com
// 2. Get your API key from https://www.asaas.com/api/v3
// 3. Set ASAAS_API_KEY and ASAAS_ENV in environment (sandbox/prod)
// 4. Create sub-accounts for each establishment/deliveryman
// 5. Use their walletId in split payments

package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

type AsaasClient struct {
	APIKey  string
	BaseURL string
}

type AsaasWalletRequest struct {
	Name        string `json:"name"`
	CpfCnpj     string `json:"cpfCnpj"`
	Email       string `json:"email"`
	Phone       string `json:"phone"`
	PersonType  string `json:"personType"` // "JURIDICA" or "FISICA"
	IncomeValue float64 `json:"incomeValue,omitempty"`
}

type AsaasWalletResponse struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Status        string `json:"status"`
	AccountNumber struct {
		Agency string `json:"agency"`
		Number string `json:"number"`
	} `json:"accountNumber"`
}

type AsaasSplitRequest struct {
	SubMerchantWalletId string  `json:"subMerchantWalletId"`
	Percentual          float64 `json:"percentual"`
	TotalFixedValue     float64 `json:"totalFixedValue,omitempty"`
}

type AsaasPaymentRequest struct {
	Customer          string             `json:"customer"`
	BillingType       string             `json:"billingType"` // "PIX", "BOLETO", "CREDIT_CARD"
	DueDate           string             `json:"dueDate"`
	ExternalReference string             `json:"externalReference"`
	Installments      int                `json:"installments,omitempty"`
	InstallmentValue  float64            `json:"installmentValue,omitempty"`
	Value             float64            `json:"value"`
	Description       string             `json:"description"`
	Split             []AsaasSplitRequest `json:"split,omitempty"`
}

type AsaasPaymentResponse struct {
	ID               string  `json:"id"`
	Status           string  `json:"status"`
	ExternalReference string `json:"externalReference"`
	DateCreated      string  `json:"dateCreated"`
	InvoiceNumber     string `json:"invoiceNumber"`
	TotalValue        float64 `json:"totalValue"`
	NetValue          float64 `json:"netValue"`
	OrderService       string `json:"orderService"`
	PixTransaction     struct {
		Payload string `json:"payload"`
		QRCode  string `json:"qrCode"`
	} `json:"pixTransaction,omitempty"`
}

func NewAsaasClient() *AsaasClient {
	env := os.Getenv("ASAAS_ENV")
	if env == "" {
		env = "sandbox"
	}
	baseURL := "https://sandbox.asaas.com/api/v3"
	if env == "production" || env == "prod" {
		baseURL = "https://api.asaas.com/api/v3"
	}
	return &AsaasClient{
		APIKey:  os.Getenv("ASAAS_API_KEY"),
		BaseURL: baseURL,
	}
}

func (c *AsaasClient) doRequest(method, path string, body interface{}) ([]byte, error) {
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
	req.Header.Set("access_token", c.APIKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("asaas request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("asaas error %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

func (c *AsaasClient) CreateSubAccount(req AsaasWalletRequest) (*AsaasWalletResponse, error) {
	body, err := c.doRequest("POST", "/subAccounts", req)
	if err != nil {
		return nil, err
	}

	var resp AsaasWalletResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

func (c *AsaasClient) CreatePayment(req AsaasPaymentRequest) (*AsaasPaymentResponse, error) {
	body, err := c.doRequest("POST", "/payments", req)
	if err != nil {
		return nil, err
	}

	var resp AsaasPaymentResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

func (c *AsaasClient) GetPayment(paymentID string) (*AsaasPaymentResponse, error) {
	body, err := c.doRequest("GET", "/payments/"+paymentID, nil)
	if err != nil {
		return nil, err
	}

	var resp AsaasPaymentResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

func (c *AsaasClient) GetSubAccount(walletID string) (*AsaasWalletResponse, error) {
	body, err := c.doRequest("GET", "/subAccounts/"+walletID, nil)
	if err != nil {
		return nil, err
	}

	var resp AsaasWalletResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}
