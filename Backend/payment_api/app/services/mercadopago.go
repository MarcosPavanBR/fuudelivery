package services

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

type MPCredentials struct {
	AccessToken string
	PublicKey   string
}

func GetMPCredentials() MPCredentials {
	return MPCredentials{
		AccessToken: os.Getenv("MERCADO_PAGO_ACCESS_TOKEN"),
		PublicKey:   os.Getenv("MERCADO_PAGO_PUBLIC_KEY"),
	}
}

type MPPaymentRequest struct {
	TransactionAmount float64          `json:"transaction_amount"`
	Description       string           `json:"description"`
	PaymentMethodID   string           `json:"payment_method_id"`
	Installments      int              `json:"installments"`
	Payer             MPPayer          `json:"payer"`
	NotificationURL   string           `json:"notification_url"`
	Token             string           `json:"token,omitempty"`
}

type MPPayer struct {
	Email     string  `json:"email"`
	FirstName string  `json:"first_name,omitempty"`
	LastName  string  `json:"last_name,omitempty"`
	Phone     MPPhone `json:"phone,omitempty"`
}

type MPPhone struct {
	AreaCode string `json:"area_code,omitempty"`
	Number   string `json:"number,omitempty"`
}

type MPPaymentResponse struct {
	ID                 int64                `json:"id"`
	Status             string               `json:"status"`
	StatusDetail       string               `json:"status_detail"`
	TransactionAmount  float64              `json:"transaction_amount"`
	PointOfInteraction *MPPointOfInteraction `json:"point_of_interaction,omitempty"`
}

type MPPointOfInteraction struct {
	TransactionData *MPTransactionData `json:"transaction_data,omitempty"`
}

type MPTransactionData struct {
	QRCodeBase64 string `json:"qr_code_base64,omitempty"`
	QRCode       string `json:"qr_code,omitempty"`
	TicketURL    string `json:"ticket_url,omitempty"`
	CopyPaste    string `json:"copy_paste,omitempty"`
}

func CreatePIXPayment(amount float64, description, email, name string) (*MPPaymentResponse, error) {
	creds := GetMPCredentials()

	payload := MPPaymentRequest{
		TransactionAmount: amount,
		Description:       description,
		PaymentMethodID:   "pix",
		Payer: MPPayer{
			Email:     email,
			FirstName: name,
		},
		NotificationURL: os.Getenv("API_BASE_URL") + "/api/payment/webhook",
	}

	body, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", "https://api.mercadopago.com/v1/payments", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+creds.AccessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error calling MP API: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	var paymentResp MPPaymentResponse
	if err := json.Unmarshal(respBody, &paymentResp); err != nil {
		return nil, fmt.Errorf("error parsing MP response: %w", err)
	}

	return &paymentResp, nil
}

func CreateCardPayment(amount float64, description, token, email string, installments int, paymentMethodID string, idempotencyKey string) (*MPPaymentResponse, error) {
	creds := GetMPCredentials()

	payload := MPPaymentRequest{
		TransactionAmount: amount,
		Description:       description,
		PaymentMethodID:   paymentMethodID,
		Installments:      installments,
		Token:             token,
		Payer: MPPayer{
			Email: email,
		},
		NotificationURL: os.Getenv("API_BASE_URL") + "/api/payment/webhook",
	}

	body, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", "https://api.mercadopago.com/v1/payments", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+creds.AccessToken)
	req.Header.Set("X-Idempotency-Key", idempotencyKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error calling MP API: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	var paymentResp MPPaymentResponse
	if err := json.Unmarshal(respBody, &paymentResp); err != nil {
		return nil, fmt.Errorf("error parsing MP response: %w", err)
	}

	return &paymentResp, nil
}

func GetPaymentStatus(paymentID int64) (*MPPaymentResponse, error) {
	creds := GetMPCredentials()

	url := fmt.Sprintf("https://api.mercadopago.com/v1/payments/%d", paymentID)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+creds.AccessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	var paymentResp MPPaymentResponse
	if err := json.Unmarshal(respBody, &paymentResp); err != nil {
		return nil, err
	}

	return &paymentResp, nil
}

func CancelPayment(paymentID int64) error {
	creds := GetMPCredentials()

	payload := map[string]string{"status": "cancelled"}
	body, _ := json.Marshal(payload)

	url := fmt.Sprintf("https://api.mercadopago.com/v1/payments/%d", paymentID)
	req, _ := http.NewRequest("PUT", url, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+creds.AccessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

func RefundPayment(paymentID int64) error {
	creds := GetMPCredentials()

	url := fmt.Sprintf("https://api.mercadopago.com/v1/payments/%d/refunds", paymentID)
	req, _ := http.NewRequest("POST", url, nil)
	req.Header.Set("Authorization", "Bearer "+creds.AccessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

func generateUUID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}
