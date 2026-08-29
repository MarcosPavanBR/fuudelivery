package services

// ============================================================================
// gateway_adapter.go — Camada de adaptação entre pkg/gateway e handlers
//
// Este arquivo inicializa o Router e expõe funções que os handlers podem
// chamar sem saber qual gateway está sendo usado.
//
// SECURITY:
//   - Credenciais são lidas de env vars, nunca commitadas
//   - O router usa circuit breaker para evitar cascata de falhas
//   - Webhooks são validados via HMAC por gateway
// ============================================================================

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/carloshomar/fuudelivery/pkg/gateway"
	"github.com/carloshomar/fuudelivery/pkg/gateway/abacatepay"
	"github.com/carloshomar/fuudelivery/pkg/gateway/asaas"
	"github.com/carloshomar/fuudelivery/pkg/gateway/mercadopago"
	"github.com/carloshomar/fuudelivery/pkg/gateway/pagarme"
)

var globalRouter *gateway.Router

// InitGatewayRouter inicializa o Router com base nas variáveis de ambiente.
func InitGatewayRouter() {
	primaryName := strings.ToLower(os.Getenv("PAYMENT_GATEWAY_PRIMARY"))
	if primaryName == "" {
		primaryName = "abacatepay"
	}

	var gateways []gateway.Gateway

	if os.Getenv("PAGARME_API_KEY") != "" {
		if pg, err := pagarme.NewGateway(); err == nil {
			gateways = append(gateways, pg)
			log.Printf("[GATEWAY] Pagar.me registrado")
		} else {
			log.Printf("[GATEWAY] Pagar.me falhou: %v", err)
		}
	}

	if os.Getenv("ASAAS_API_KEY") != "" {
		if ag, err := asaas.NewGateway(); err == nil {
			gateways = append(gateways, ag)
			log.Printf("[GATEWAY] Asaas registrado")
		} else {
			log.Printf("[GATEWAY] Asaas falhou: %v", err)
		}
	}

	if os.Getenv("ABACATE_PAY_API_KEY") != "" {
		if ap, err := abacatepay.NewGateway(); err == nil {
			gateways = append(gateways, ap)
			log.Printf("[GATEWAY] AbacatePay registrado (PIX only)")
		} else {
			log.Printf("[GATEWAY] AbacatePay falhou: %v", err)
		}
	}

	if os.Getenv("MERCADOPAGO_ACCESS_TOKEN") != "" {
		if mp, err := mercadopago.NewGateway(); err == nil {
			gateways = append(gateways, mp)
			log.Printf("[GATEWAY] Mercado Pago registrado")
		} else {
			log.Printf("[GATEWAY] Mercado Pago falhou: %v", err)
		}
	}

	if len(gateways) == 0 {
		log.Printf("[GATEWAY] Nenhum gateway configurado — usando caminho legado")
		return
	}

	globalRouter = gateway.NewRouter(gateways...)
	log.Printf("[GATEWAY] Router com %d gateways (primary=%s)", len(gateways), primaryName)
}

// IsGatewayEnabled retorna true se o router está ativo.
func IsGatewayEnabled() bool { return globalRouter != nil }

// ============================================================================
// Tipos para handlers
// ============================================================================

type GatewayPaymentRequest struct {
	OrderID         string
	CustomerID      int64
	CustomerPhone   string
	EstablishmentID int64
	Amount          float64
	DeliveryAmount  float64
	Method          string
	CardToken       string
	Installments    int
	CustomerName    string
	CustomerEmail   string
	Description     string
}

type GatewayPaymentResult struct {
	GatewayName    string
	GatewayID      string
	Status         string
	PixQRCode      string
	PixCopyPaste   string
	QRCodeBase64   string
	CardLastDigits string
	Installments   int
	Requires3DS    bool
	ThreeDSURL     string
}

// ProcessPaymentViaGateway processa pagamento via router multi-gateway.
func ProcessPaymentViaGateway(ctx context.Context, req GatewayPaymentRequest) (*GatewayPaymentResult, bool) {
	if globalRouter == nil {
		return nil, false
	}

	orderID, _ := strconv.ParseInt(req.OrderID, 10, 64)
	method := parsePaymentMethod(req.Method)
	amountCents := toCents(req.Amount)

	txReq := &gateway.TransactionRequest{
		OrderID:       orderID,
		Amount:        amountCents,
		Currency:      "BRL",
		PaymentMethod: method,
		CustomerEmail: req.CustomerEmail,
		CustomerName:  req.CustomerName,
		CustomerPhone: req.CustomerPhone,
		Description:   req.Description,
	}

	if req.CardToken != "" {
		txReq.CardData = &gateway.CardData{
			Token:        req.CardToken,
			Installments: req.Installments,
			HolderName:   req.CustomerName,
		}
	}

	if os.Getenv("PAYMENT_SPLIT_ENABLED") == "true" {
		txReq.SplitRules = []gateway.SplitRule{
			{Percentage: 5.0, RecipientID: "platform"},
			{Percentage: 85.0, RecipientID: fmt.Sprintf("%d", req.EstablishmentID)},
			{Percentage: 10.0, RecipientID: "delivery_man"},
		}
	}

	resp, err := globalRouter.CreateTransactionWithFallback(ctx, txReq)
	if err != nil {
		log.Printf("[GATEWAY] Falha: %v", err)
		return nil, false
	}

	return &GatewayPaymentResult{
		GatewayName:    resp.Gateway,
		GatewayID:      resp.GatewayID,
		Status:         string(resp.Status),
		PixQRCode:      resp.PIXQRCode,
		PixCopyPaste:   resp.PIXCopyPaste,
		QRCodeBase64:   resp.PIXQRCode,
		CardLastDigits: resp.CardLast4,
		Requires3DS:    resp.RequiresAuth,
		ThreeDSURL:     resp.AuthURL,
	}, true
}

func ValidateWebhookByGateway(body []byte, headers map[string]string) (string, bool) {
	gatewayName := detectGatewayFromHeaders(headers)
	if gatewayName != "" && globalRouter != nil {
		gw, err := globalRouter.Select(gateway.MethodPIX, false, false)
		if err == nil {
			return gatewayName, gw.ValidateWebhook(body, headers)
		}
	}
	return "", false
}

func ParseGatewayWebhook(ctx context.Context, gatewayName string, body []byte) (*gateway.WebhookEvent, error) {
	if globalRouter == nil {
		return nil, fmt.Errorf("router não inicializado")
	}
	gw, err := globalRouter.Select(gateway.MethodPIX, false, false)
	if err != nil {
		return nil, err
	}
	return gw.ParseWebhook(body)
}

func MapGatewayStatus(gatewayStatus string) string {
	switch strings.ToLower(gatewayStatus) {
	case "paid", "confirmed", "captured", "approved":
		return "CONFIRMED"
	case "pending", "waiting", "authorized":
		return "PENDING"
	case "refunded", "chargeback":
		return "REFUNDED"
	case "expired":
		return "EXPIRED"
	case "cancelled", "voided":
		return "CANCELLED"
	case "failed", "refused":
		return "REFUSED"
	default:
		return strings.ToUpper(gatewayStatus)
	}
}

func parsePaymentMethod(method string) gateway.PaymentMethod {
	switch strings.ToLower(method) {
	case "pix":
		return gateway.MethodPIX
	case "credit":
		return gateway.MethodCreditCard
	case "debit":
		return gateway.MethodDebitCard
	default:
		return gateway.MethodPIX
	}
}

func detectGatewayFromHeaders(headers map[string]string) string {
	if headers["x-pagarme-signature"] != "" {
		return "pagarme"
	}
	if headers["access-token"] != "" {
		return "asaas"
	}
	if headers["x-abacatepay-signature"] != "" {
		return "abacatepay"
	}
	if headers["x-signature"] != "" {
		return "mercadopago"
	}
	return ""
}

func toCents(amount float64) int64 {
	return int64(amount*100 + 0.5)
}

func init() {
	go func() {
		time.Sleep(100 * time.Millisecond)
		if os.Getenv("PAYMENT_GATEWAY_PRIMARY") != "" || os.Getenv("PAGARME_API_KEY") != "" || os.Getenv("ABACATE_PAY_API_KEY") != "" {
			InitGatewayRouter()
		}
	}()
}
