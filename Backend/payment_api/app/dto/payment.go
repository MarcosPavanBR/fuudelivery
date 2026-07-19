package dto

import "github.com/carloshomar/vercardapio/app/models"

type PaymentRequest struct {
	OrderID          string  `json:"order_id"`
	CustomerID       int64   `json:"customer_id"`
	EstablishmentID  int64   `json:"establishment_id"`
	Amount           float64 `json:"amount"`
	Method           string  `json:"method"`
	CardToken        string  `json:"card_token,omitempty"`
	Installments     int     `json:"installments,omitempty"`
	CustomerName     string  `json:"customer_name,omitempty"`
	CustomerEmail    string  `json:"customer_email,omitempty"`
	CustomerPhone    string  `json:"customer_phone,omitempty"`
}

type PaymentResponse struct {
	PaymentID     string  `json:"payment_id"`
	Status        string  `json:"status"`
	PixQRCode     string  `json:"pix_qr_code,omitempty"`
	PixCopyPaste  string  `json:"pix_copy_paste,omitempty"`
	QRCodeBase64  string  `json:"qr_code_base64,omitempty"`
	TicketURL     string  `json:"ticket_url,omitempty"`
	MPPaymentID   int64   `json:"mp_payment_id,omitempty"`
	Message       string  `json:"message"`
}

type PIXGenerateResponse struct {
	QRCode    string `json:"qr_code"`
	CopyPaste string `json:"copy_paste"`
	ExpiresIn int    `json:"expires_in"`
}

type CardTokenizeRequest struct {
	CardNumber     string `json:"card_number"`
	CardHolderName string `json:"card_holder_name"`
	ExpMonth       int    `json:"exp_month"`
	ExpYear        int    `json:"exp_year"`
	CardCVV        string `json:"card_cvv"`
}

type WalletTopUpRequest struct {
	UserID int64   `json:"user_id"`
	Amount float64 `json:"amount"`
	Method string  `json:"method"`
}

type SplitPaymentRequest struct {
	PaymentID string            `json:"payment_id"`
	Rules     []models.SplitRule `json:"rules"`
}
