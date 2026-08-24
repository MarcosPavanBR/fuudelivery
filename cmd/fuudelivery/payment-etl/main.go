// Command payment-etl reconcilia os pagamentos duplicados entre os DOIS
// bancos MongoDB de pagamento e os consolida na tabela unificada `payments`
// do Postgres (schema sql/03_dominio_pagamentos.sql), casando por order_id.
//
// Fontes:
//   A = payment_api        (DB default "fuudelivery_payments", status MAIÚSCULAS)
//   B = Backend/Payment    (DB default "payment", status minúsculas enum)
//
// Ambos usam a collection "payments" e falam do MESMO pagamento — cada um
// com metade dos campos. Este ETL une as duas metades numa linha só:
//   - Campos de gateway/cobrança (PIX, cartão, split, créditos)  ← de A
//   - Campos de risco/aprovação/compliance (risk, review, admins) ← de B
//   - status: normaliza e, em conflito, vale o MAIS AVANÇADO do ciclo de
//     vida (refunded > disputed > cancelled > rejected > approved > pending)
//
// Seguro por padrão: sem -apply roda em DRY-RUN (só imprime o plano e o
// relatório, não escreve nada). Com -apply faz upsert idempotente por
// order_id (pode rodar quantas vezes quiser).
//
// Pré-requisito: a tabela `payments` JÁ DEVE EXISTIR no Postgres de destino
// (aplicar sql/03_dominio_pagamentos.sql antes). O ETL não cria/migra schema.
//
// Uso:
//   PAYMENT_API_MONGO_URI=...  PAYMENT_API_MONGO_DB=fuudelivery_payments \
//   LEGACY_PAYMENT_MONGO_URI=... LEGACY_PAYMENT_MONGO_DB=payment \
//   DB_CONNECTION_STRING=postgres://... \
//   go run ./payment-etl            # dry-run
//   go run ./payment-etl -apply     # grava no Postgres

package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var (
	applyFlag bool
	limitFlag int
)

// ---------------------------------------------------------------------------
// Config
// ---------------------------------------------------------------------------

type config struct {
	apiMongoURI string // A: payment_api
	apiMongoDB  string
	legMongoURI string // B: Backend/Payment
	legMongoDB  string
	collection  string
	dbDSN       string // Postgres de destino (tabela payments)
	limit       int
}

func loadConfig() config {
	return config{
		apiMongoURI: envOr("PAYMENT_API_MONGO_URI", "mongodb://localhost:27017"),
		apiMongoDB:  envOr("PAYMENT_API_MONGO_DB", "fuudelivery_payments"),
		legMongoURI: envOr("LEGACY_PAYMENT_MONGO_URI", "mongodb://localhost:27017"),
		legMongoDB:  envOr("LEGACY_PAYMENT_MONGO_DB", "payment"),
		collection:  envOr("ETL_COLLECTION", "payments"),
		dbDSN:       os.Getenv("DB_CONNECTION_STRING"),
		limit:       0,
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// ---------------------------------------------------------------------------
// Registro intermediário (superset dos dois structs originais)
// ---------------------------------------------------------------------------

type paymentRecord struct {
	OrderID string

	// de payment_api (A)
	CustomerID              int64
	CustomerPhone           string
	EstablishmentID         int64
	Amount                  float64
	DeliveryAmount          float64
	Method                  string
	PixQRCode               string
	PixCopyPaste            string
	QRCodeBase64            string
	TicketURL               string
	MPPaymentID             int64
	MPStatus                string
	AbacatePayID            string
	CardLastDigits          string
	CardToken               string
	Installments            int
	SplitRules              []map[string]interface{}
	ConfirmedAt             *time.Time
	WalletCreditedAt        *time.Time
	EstablishmentCreditedAt *time.Time
	RefundedAt              *time.Time
	ApprovedBy              string

	// de Backend/Payment (B)
	CustomerName      string
	CustomerEmail     string
	EstablishmentName string
	RiskLevel         string
	RiskScore         float64
	RequiresApproval  bool
	ApprovedAt        *time.Time
	RejectedBy        string
	RejectedAt        *time.Time
	RejectionReason   string
	Reference         string
	GatewayStatus     string
	Metadata          map[string]string

	Status    string // status CRU da fonte (normalizado no merge)
	CreatedAt time.Time
	UpdatedAt time.Time

	Sources        string // "A" | "B" | "A+B"
	StatusConflict bool   // conflito de status resolvido (mais avançado venceu)
	AmountConflict bool   // A e B divergem no valor (A venceu)
}

// statusPriority: quanto maior, mais "avançado" no ciclo de vida.
var statusPriority = map[string]int{
	"pending":   1,
	"approved":  2,
	"rejected":  3,
	"cancelled": 4,
	"disputed":  5,
	"refunded":  6,
}

// normalizeStatus mapeia o status de QUALQUER fonte para o enum da tabela
// unificada (minúsculas): PENDING/CONFIRMED/REJECTED do payment_api viram
// pending/approved/rejected; o Backend/Payment já usa o enum.
func normalizeStatus(s string) string {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "", "PENDING":
		return "pending"
	case "CONFIRMED", "APPROVED":
		return "approved"
	case "REJECTED":
		return "rejected"
	case "CANCELLED", "CANCELED":
		return "cancelled"
	case "REFUNDED":
		return "refunded"
	case "DISPUTED":
		return "disputed"
	default:
		return strings.ToLower(strings.TrimSpace(s))
	}
}

// mergeStatus resolve o conflito de status: vale o mais avançado.
func mergeStatus(a, b string) (status string, conflict bool) {
	sa, sb := normalizeStatus(a), normalizeStatus(b)
	if sa == sb {
		return sa, false
	}
	// Status vazio = fonte não registrou status (sem conflito: usa o outro).
	if a == "" && b == "" {
		return "pending", false
	}
	if a == "" {
		return sb, false
	}
	if b == "" {
		return sa, false
	}
	// Ambos têm status e divergem → o mais avançado do ciclo de vida vence.
	if statusPriority[sa] >= statusPriority[sb] {
		return sa, true
	}
	return sb, true
}

// ---------------------------------------------------------------------------
// Leitura das fontes Mongo (raw bson.M → paymentRecord)
// ---------------------------------------------------------------------------

func connectMongo(ctx context.Context, uri string) (*mongo.Client, error) {
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri).SetServerSelectionTimeout(15*time.Second))
	if err != nil {
		return nil, err
	}
	if err := client.Ping(ctx, nil); err != nil {
		_ = client.Disconnect(ctx)
		return nil, fmt.Errorf("ping: %w", err)
	}
	return client, nil
}

// loadSource lê TODOS os pagamentos de uma collection e indexa por order_id.
func loadSource(ctx context.Context, coll *mongo.Collection, limit int) (map[string]bson.M, int, error) {
	opts := options.Find()
	if limit > 0 {
		opts.SetLimit(int64(limit))
	}
	cur, err := coll.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cur.Close(ctx)

	byOrder := make(map[string]bson.M)
	skip := 0
	for cur.Next(ctx) {
		var raw bson.M
		if err := cur.Decode(&raw); err != nil {
			log.Printf("  [skip] doc ilegível: %v", err)
			skip++
			continue
		}
		orderID := getString(raw, "order_id")
		if orderID == "" {
			skip++
			continue
		}
		byOrder[orderID] = raw
	}
	return byOrder, skip, cur.Err()
}

// getString / getInt64 / getFloat64 / getTime / getBool: leitura tolerante
// (int32/int64/float64/string, DateTime/time.Time) — os dois bancos gravam
// os mesmos campos com tipos ligeiramente diferentes.
func getString(m bson.M, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case primitive.ObjectID:
		return t.Hex()
	case fmt.Stringer:
		return t.String()
	default:
		return fmt.Sprintf("%v", t)
	}
}

func getInt64(m bson.M, key string) int64 {
	v, ok := m[key]
	if !ok || v == nil {
		return 0
	}
	switch t := v.(type) {
	case int64:
		return t
	case int32:
		return int64(t)
	case int:
		return int64(t)
	case float64:
		return int64(t)
	case string:
		var out int64
		fmt.Sscanf(t, "%d", &out)
		return out
	default:
		return 0
	}
}

func getFloat64(m bson.M, key string) float64 {
	v, ok := m[key]
	if !ok || v == nil {
		return 0
	}
	switch t := v.(type) {
	case float64:
		return t
	case int64:
		return float64(t)
	case int32:
		return float64(t)
	case int:
		return float64(t)
	case string:
		var out float64
		fmt.Sscanf(t, "%f", &out)
		return out
	default:
		return 0
	}
}

func getTime(m bson.M, key string) *time.Time {
	v, ok := m[key]
	if !ok || v == nil {
		return nil
	}
	switch t := v.(type) {
	case time.Time:
		tt := t
		return &tt
	case primitive.DateTime:
		tt := t.Time()
		return &tt
	case string:
		for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05"} {
			if parsed, err := time.Parse(layout, t); err == nil {
				return &parsed
			}
		}
	}
	return nil
}

func getBool(m bson.M, key string) bool {
	if b, ok := m[key].(bool); ok {
		return b
	}
	return false
}

// splitRulesRaw extrai split_rules (lista de mapas) preservando o JSON.
func splitRulesRaw(m bson.M) []map[string]interface{} {
	items, ok := m["split_rules"].(primitive.A)
	if !ok || len(items) == 0 {
		return nil
	}
	out := make([]map[string]interface{}, 0, len(items))
	for _, it := range items {
		if mm, ok := it.(bson.M); ok {
			out = append(out, mm)
		}
	}
	return out
}

func metadataRaw(m bson.M) map[string]string {
	v, ok := m["metadata"]
	if !ok || v == nil {
		return nil
	}
	switch t := v.(type) {
	case bson.M:
		out := make(map[string]string, len(t))
		for k, val := range t {
			out[k] = fmt.Sprintf("%v", val)
		}
		return out
	case map[string]string:
		return t
	}
	return nil
}

// parseA decodifica um doc do payment_api.
func parseA(raw bson.M) paymentRecord {
	return paymentRecord{
		OrderID:                 getString(raw, "order_id"),
		CustomerID:              getInt64(raw, "customer_id"),
		CustomerPhone:           getString(raw, "customer_phone"),
		EstablishmentID:         getInt64(raw, "establishment_id"),
		Amount:                  getFloat64(raw, "amount"),
		DeliveryAmount:          getFloat64(raw, "delivery_amount"),
		Method:                  strings.ToLower(getString(raw, "method")),
		Status:                  getString(raw, "status"),
		PixQRCode:               getString(raw, "pix_qr_code"),
		PixCopyPaste:            getString(raw, "pix_copy_paste"),
		QRCodeBase64:            getString(raw, "qr_code_base64"),
		TicketURL:               getString(raw, "ticket_url"),
		MPPaymentID:             getInt64(raw, "mp_payment_id"),
		MPStatus:                getString(raw, "mp_status"),
		AbacatePayID:            getString(raw, "abacatepay_id"),
		CardLastDigits:          getString(raw, "card_last_digits"),
		CardToken:               getString(raw, "card_token"),
		Installments:            int(getInt64(raw, "installments")),
		SplitRules:              splitRulesRaw(raw),
		ConfirmedAt:             getTime(raw, "confirmed_at"),
		WalletCreditedAt:        getTime(raw, "wallet_credited_at"),
		EstablishmentCreditedAt: getTime(raw, "establishment_credited_at"),
		RefundedAt:              getTime(raw, "refunded_at"),
		ApprovedBy:              getString(raw, "approved_by"),
		RejectedAt:              getTime(raw, "rejected_at"),
		RejectedBy:              getString(raw, "rejected_by"),
		RejectionReason:         getString(raw, "rejection_reason"),
		CreatedAt:               firstTime(getTime(raw, "created_at"), time.Now()),
		UpdatedAt:               firstTime(getTime(raw, "updated_at"), time.Now()),
	}
}

// parseB decodifica um doc do Backend/Payment.
func parseB(raw bson.M) paymentRecord {
	return paymentRecord{
		OrderID:           getString(raw, "order_id"),
		CustomerID:        getInt64(raw, "customer_id"),
		CustomerName:      getString(raw, "customer_name"),
		CustomerEmail:     getString(raw, "customer_email"),
		CustomerPhone:     getString(raw, "customer_phone"),
		EstablishmentID:   getInt64(raw, "establishment_id"),
		EstablishmentName: getString(raw, "establishment_name"),
		Amount:            getFloat64(raw, "amount"),
		DeliveryAmount:    getFloat64(raw, "delivery_amount"),
		Method:            strings.ToLower(getString(raw, "method")),
		Status:            getString(raw, "status"),
		RiskLevel:         strings.ToLower(getString(raw, "risk_level")),
		RiskScore:         getFloat64(raw, "risk_score"),
		RequiresApproval:  getBool(raw, "requires_approval"),
		ApprovedAt:        getTime(raw, "approved_at"),
		RejectedBy:        getString(raw, "rejected_by"),
		RejectedAt:        getTime(raw, "rejected_at"),
		RejectionReason:   getString(raw, "rejection_reason"),
		Reference:         getString(raw, "reference"),
		GatewayStatus:     getString(raw, "gateway_status"),
		Metadata:          metadataRaw(raw),
		CreatedAt:         firstTime(getTime(raw, "created_at"), time.Now()),
		UpdatedAt:         firstTime(getTime(raw, "updated_at"), time.Now()),
	}
}

func firstTime(t *time.Time, def time.Time) time.Time {
	if t != nil {
		return *t
	}
	return def
}

// ---------------------------------------------------------------------------
// Merge A + B → registro unificado
// ---------------------------------------------------------------------------

func mergePayments(a, b *paymentRecord, orderID string) paymentRecord {
	out := paymentRecord{OrderID: orderID}
	switch {
	case a != nil && b != nil:
		out.Sources = "A+B"
	case a != nil:
		out.Sources = "A"
	default:
		out.Sources = "B"
	}

	// Campos de gateway/cobrança ← A (serviço ativo, fonte de verdade da
	// cobrança); fallback para B quando A não existe.
	if a != nil {
		out.CustomerID = a.CustomerID
		out.CustomerPhone = a.CustomerPhone
		out.EstablishmentID = a.EstablishmentID
		out.Amount = a.Amount
		out.DeliveryAmount = a.DeliveryAmount
		out.Method = a.Method
		out.PixQRCode = a.PixQRCode
		out.PixCopyPaste = a.PixCopyPaste
		out.QRCodeBase64 = a.QRCodeBase64
		out.TicketURL = a.TicketURL
		out.MPPaymentID = a.MPPaymentID
		out.MPStatus = a.MPStatus
		out.AbacatePayID = a.AbacatePayID
		out.CardLastDigits = a.CardLastDigits
		out.CardToken = a.CardToken
		out.Installments = a.Installments
		out.SplitRules = a.SplitRules
		out.ConfirmedAt = a.ConfirmedAt
		out.WalletCreditedAt = a.WalletCreditedAt
		out.EstablishmentCreditedAt = a.EstablishmentCreditedAt
		out.RefundedAt = a.RefundedAt
		out.ApprovedBy = a.ApprovedBy
		out.RejectedAt = a.RejectedAt
		out.RejectedBy = a.RejectedBy
		out.RejectionReason = a.RejectionReason
	}
	if b != nil {
		if out.CustomerID == 0 {
			out.CustomerID = b.CustomerID
		}
		if out.CustomerPhone == "" {
			out.CustomerPhone = b.CustomerPhone
		}
		if out.EstablishmentID == 0 {
			out.EstablishmentID = b.EstablishmentID
		}
		if out.Amount == 0 {
			out.Amount = b.Amount
		}
		if out.DeliveryAmount == 0 {
			out.DeliveryAmount = b.DeliveryAmount
		}
		if out.Method == "" {
			out.Method = b.Method
		}
		if out.ApprovedBy == "" {
			out.ApprovedBy = b.ApprovedBy
		}
	}

	// Campos de risco/aprovação/compliance ← B (autoridade de risco).
	if b != nil {
		out.CustomerName = b.CustomerName
		out.CustomerEmail = b.CustomerEmail
		out.EstablishmentName = b.EstablishmentName
		out.RiskLevel = b.RiskLevel
		out.RiskScore = b.RiskScore
		out.RequiresApproval = b.RequiresApproval
		out.ApprovedAt = b.ApprovedAt
		out.RejectedBy = b.RejectedBy
		out.RejectedAt = b.RejectedAt
		out.RejectionReason = b.RejectionReason
		out.Reference = b.Reference
		out.GatewayStatus = b.GatewayStatus
		out.Metadata = b.Metadata
	}

	// status: normalizado; conflito → mais avançado vence
	var sa, sb string
	if a != nil {
		sa = a.Status
	}
	if b != nil {
		sb = b.Status
	}
	out.Status, out.StatusConflict = mergeStatus(sa, sb)

	// created_at: o mais antigo; updated_at: o mais recente
	var createdAt, updatedAt time.Time
	if a != nil {
		createdAt = a.CreatedAt
		updatedAt = a.UpdatedAt
	}
	if b != nil {
		if createdAt.IsZero() || b.CreatedAt.Before(createdAt) {
			createdAt = b.CreatedAt
		}
		if b.UpdatedAt.After(updatedAt) {
			updatedAt = b.UpdatedAt
		}
	}
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	if updatedAt.IsZero() {
		updatedAt = createdAt
	}
	out.CreatedAt = createdAt
	out.UpdatedAt = updatedAt

	// Conflito de valor: A manda (gateway), mas reporta
	if a != nil && b != nil && a.Amount != 0 && b.Amount != 0 && a.Amount != b.Amount {
		out.AmountConflict = true
	}
	return out
}

// ---------------------------------------------------------------------------
// Escrita no Postgres (upsert idempotente por order_id)
// ---------------------------------------------------------------------------

// paymentRow mapeia a tabela unificada (sql/03). O ETL NÃO cria/migra a
// tabela: ela deve existir (aplicar sql/03_dominio_pagamentos.sql antes).
type paymentRow struct {
	ID                      int64      `gorm:"column:id;primaryKey"`
	OrderID                 string     `gorm:"column:order_id;size:100;not null"`
	CustomerID              int64      `gorm:"column:customer_id;not null"`
	CustomerName            string     `gorm:"column:customer_name;size:255"`
	CustomerEmail           string     `gorm:"column:customer_email;size:255"`
	CustomerPhone           string     `gorm:"column:customer_phone;size:30"`
	EstablishmentID         int64      `gorm:"column:establishment_id;not null"`
	EstablishmentName       string     `gorm:"column:establishment_name;size:255"`
	Amount                  float64    `gorm:"column:amount;not null"`
	DeliveryAmount          float64    `gorm:"column:delivery_amount"`
	Method                  string     `gorm:"column:method;size:20;not null"`
	Status                  string     `gorm:"column:status;size:20;not null;default:pending"`
	RiskLevel               string     `gorm:"column:risk_level;size:20"`
	RiskScore               float64    `gorm:"column:risk_score"`
	RequiresApproval        bool       `gorm:"column:requires_approval;not null;default:false"`
	ApprovedBy              string     `gorm:"column:approved_by;size:255"`
	ApprovedAt              *time.Time `gorm:"column:approved_at"`
	RejectedBy              string     `gorm:"column:rejected_by;size:255"`
	RejectedAt              *time.Time `gorm:"column:rejected_at"`
	RejectionReason         string     `gorm:"column:rejection_reason"`
	PixQRCode               string     `gorm:"column:pix_qr_code"`
	PixCopyPaste            string     `gorm:"column:pix_copy_paste"`
	QRCodeBase64            string     `gorm:"column:qr_code_base64"`
	TicketURL               string     `gorm:"column:ticket_url"`
	MPPaymentID             int64      `gorm:"column:mp_payment_id"`
	MPStatus                string     `gorm:"column:mp_status;size:30"`
	AbacatePayID            string     `gorm:"column:abacatepay_id;size:100"`
	GatewayStatus           string     `gorm:"column:gateway_status;size:50"`
	CardLastDigits          string     `gorm:"column:card_last_digits;size:4"`
	CardToken               string     `gorm:"column:card_token"`
	Installments            int        `gorm:"column:installments"`
	Reference               string     `gorm:"column:reference;size:100"`
	Metadata                jsonRaw    `gorm:"column:metadata;type:jsonb"`
	ConfirmedAt             *time.Time `gorm:"column:confirmed_at"`
	WalletCreditedAt        *time.Time `gorm:"column:wallet_credited_at"`
	EstablishmentCreditedAt *time.Time `gorm:"column:establishment_credited_at"`
	RefundedAt              *time.Time `gorm:"column:refunded_at"`
	SplitRules              jsonRaw    `gorm:"column:split_rules;type:jsonb"`
	CreatedAt               time.Time  `gorm:"column:created_at"`
	UpdatedAt               time.Time  `gorm:"column:updated_at"`
}

// jsonRaw serializa para jsonb sem dependência externa.
type jsonRaw []byte

func (j jsonRaw) Value() (interface{}, error) {
	if len(j) == 0 {
		return "{}", nil
	}
	return string(j), nil
}

func (j *jsonRaw) Scan(v interface{}) error {
	switch t := v.(type) {
	case nil:
		*j = nil
	case []byte:
		*j = append((*j)[:0], t...)
	case string:
		*j = []byte(t)
	}
	return nil
}

// upsertPayment insere ou atualiza a linha por order_id (idempotente).
func upsertPayment(db *gorm.DB, rec paymentRecord) (string, error) {
	row := buildRow(rec)

	var existing paymentRow
	err := db.Where("order_id = ?", rec.OrderID).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if err := db.Create(&row).Error; err != nil {
			return "", err
		}
		return "inserted", nil
	}
	if err != nil {
		return "", err
	}
	row.ID = existing.ID
	if err := db.Save(&row).Error; err != nil {
		return "", err
	}
	return "updated", nil
}

func buildRow(rec paymentRecord) paymentRow {
	row := paymentRow{
		OrderID:                 rec.OrderID,
		CustomerID:              rec.CustomerID,
		CustomerName:            rec.CustomerName,
		CustomerEmail:           rec.CustomerEmail,
		CustomerPhone:           rec.CustomerPhone,
		EstablishmentID:         rec.EstablishmentID,
		EstablishmentName:       rec.EstablishmentName,
		Amount:                  rec.Amount,
		DeliveryAmount:          rec.DeliveryAmount,
		Method:                  rec.Method,
		Status:                  rec.Status,
		RiskLevel:               rec.RiskLevel,
		RiskScore:               rec.RiskScore,
		RequiresApproval:        rec.RequiresApproval,
		ApprovedBy:              rec.ApprovedBy,
		ApprovedAt:              rec.ApprovedAt,
		RejectedBy:              rec.RejectedBy,
		RejectedAt:              rec.RejectedAt,
		RejectionReason:         rec.RejectionReason,
		PixQRCode:               rec.PixQRCode,
		PixCopyPaste:            rec.PixCopyPaste,
		QRCodeBase64:            rec.QRCodeBase64,
		TicketURL:               rec.TicketURL,
		MPPaymentID:             rec.MPPaymentID,
		MPStatus:                rec.MPStatus,
		AbacatePayID:            rec.AbacatePayID,
		GatewayStatus:           rec.GatewayStatus,
		CardLastDigits:          rec.CardLastDigits,
		CardToken:               rec.CardToken,
		Installments:            rec.Installments,
		Reference:               rec.Reference,
		ConfirmedAt:             rec.ConfirmedAt,
		WalletCreditedAt:        rec.WalletCreditedAt,
		EstablishmentCreditedAt: rec.EstablishmentCreditedAt,
		RefundedAt:              rec.RefundedAt,
		CreatedAt:               rec.CreatedAt,
		UpdatedAt:               rec.UpdatedAt,
	}
	if rec.Metadata != nil {
		row.Metadata, _ = json.Marshal(rec.Metadata)
	} else {
		row.Metadata = []byte("{}")
	}
	if rec.SplitRules != nil {
		row.SplitRules, _ = json.Marshal(rec.SplitRules)
	} else {
		row.SplitRules = []byte("[]")
	}
	return row
}

// ---------------------------------------------------------------------------
// main
// ---------------------------------------------------------------------------

func main() {
	flag.BoolVar(&applyFlag, "apply", false, "grava no Postgres (padrão: dry-run)")
	flag.IntVar(&limitFlag, "limit", 0, "limita o número de docs lidos por fonte (0 = todos)")
	flag.Parse()

	cfg := loadConfig()
	cfg.limit = limitFlag
	if cfg.dbDSN == "" {
		log.Fatal("DB_CONNECTION_STRING é obrigatório (Postgres de destino)")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// ---- Conexões ----
	log.Printf("Conectando A=payment_api (%s/%s) ...", cfg.apiMongoURI, cfg.apiMongoDB)
	apiClient, err := connectMongo(ctx, cfg.apiMongoURI)
	if err != nil {
		log.Fatalf("payment_api Mongo: %v", err)
	}
	defer apiClient.Disconnect(ctx)

	log.Printf("Conectando B=Backend/Payment (%s/%s) ...", cfg.legMongoURI, cfg.legMongoDB)
	legClient, err := connectMongo(ctx, cfg.legMongoURI)
	if err != nil {
		log.Fatalf("Backend/Payment Mongo: %v", err)
	}
	defer legClient.Disconnect(ctx)

	db, err := gorm.Open(postgres.Open(cfg.dbDSN), &gorm.Config{PrepareStmt: false})
	if err != nil {
		log.Fatalf("Postgres destino: %v", err)
	}

	// Verifica que a tabela destino existe (schema do sql/03 aplicado).
	var one int
	if err := db.Raw("SELECT 1 FROM payments LIMIT 1").Scan(&one).Error; err != nil {
		log.Fatalf("tabela 'payments' não acessível no Postgres destino: %v\n"+
			"Aplique antes: psql $DB_CONNECTION_STRING -f sql/03_dominio_pagamentos.sql", err)
	}

	// ---- Carga ----
	log.Printf("Lendo A (collection %q, limit=%d) ...", cfg.collection, cfg.limit)
	apiDocs, apiSkip, err := loadSource(ctx, apiClient.Database(cfg.apiMongoDB).Collection(cfg.collection), cfg.limit)
	if err != nil {
		log.Fatalf("ler payment_api: %v", err)
	}
	log.Printf("Lendo B (collection %q, limit=%d) ...", cfg.collection, cfg.limit)
	legDocs, legSkip, err := loadSource(ctx, legClient.Database(cfg.legMongoDB).Collection(cfg.collection), cfg.limit)
	if err != nil {
		log.Fatalf("ler Backend/Payment: %v", err)
	}

	// ---- Reconciliação ----
	orderIDs := make(map[string]struct{}, len(apiDocs)+len(legDocs))
	for k := range apiDocs {
		orderIDs[k] = struct{}{}
	}
	for k := range legDocs {
		orderIDs[k] = struct{}{}
	}
	sorted := make([]string, 0, len(orderIDs))
	for k := range orderIDs {
		sorted = append(sorted, k)
	}
	sort.Strings(sorted)

	stats := struct {
		onlyA, onlyB, both int
		inserted, updated  int
		statusConflicts    int
		amountConflicts    int
	}{}

	if !applyFlag {
		fmt.Println("\n=== DRY-RUN (nada será gravado — rode com -apply para gravar) ===")
	}

	var plan []string
	for _, orderID := range sorted {
		var a, b *paymentRecord
		if raw, ok := apiDocs[orderID]; ok {
			pa := parseA(raw)
			a = &pa
		}
		if raw, ok := legDocs[orderID]; ok {
			pb := parseB(raw)
			b = &pb
		}
		rec := mergePayments(a, b, orderID)

		switch rec.Sources {
		case "A+B":
			stats.both++
		case "A":
			stats.onlyA++
		case "B":
			stats.onlyB++
		}
		if rec.StatusConflict {
			stats.statusConflicts++
		}
		if rec.AmountConflict {
			stats.amountConflicts++
		}

		if applyFlag {
			action, err := upsertPayment(db, rec)
			if err != nil {
				log.Printf("[erro] order %s: %v", orderID, err)
				continue
			}
			if action == "inserted" {
				stats.inserted++
			} else {
				stats.updated++
			}
		} else {
			action := "insert"
			var existing paymentRow
			if err := db.Where("order_id = ?", orderID).First(&existing).Error; err == nil {
				action = "update"
			}
			plan = append(plan, fmt.Sprintf("  %-8s %-6s order=%-22s status=%-10s A=%s B=%s%s",
				action, rec.Sources, orderID, rec.Status,
				yesNo(a != nil), yesNo(b != nil),
				conflictSuffix(rec.StatusConflict, rec.AmountConflict)))
		}
	}

	// ---- Relatório ----
	fmt.Printf("\n=== RELATÓRIO DE RECONCILIAÇÃO ===\n")
	fmt.Printf("  payment_api (A):      %d docs (%d sem order_id ignorados)\n", len(apiDocs), apiSkip)
	fmt.Printf("  Backend/Payment (B):  %d docs (%d sem order_id ignorados)\n", len(legDocs), legSkip)
	fmt.Printf("  order_ids únicos:     %d\n", len(sorted))
	fmt.Printf("  só em A:              %d\n", stats.onlyA)
	fmt.Printf("  só em B:              %d\n", stats.onlyB)
	fmt.Printf("  em ambos (merged):    %d\n", stats.both)
	fmt.Printf("  conflitos de status:  %d (resolvido: mais avançado vence)\n", stats.statusConflicts)
	fmt.Printf("  conflitos de valor:   %d (resolvido: payment_api vence)\n", stats.amountConflicts)
	if applyFlag {
		fmt.Printf("  gravados:             %d insert + %d update\n", stats.inserted, stats.updated)
	} else {
		fmt.Printf("  plano:                %d registros (inserir/atualizar) — DRY-RUN\n", len(plan))
		if len(plan) > 0 {
			fmt.Println("\nPlano (primeiros 50):")
			for i, p := range plan {
				if i == 50 {
					fmt.Printf("  ... e mais %d\n", len(plan)-50)
					break
				}
				fmt.Println(p)
			}
		}
	}
	fmt.Println("\nPróximos passos sugeridos: rodar com -apply, conferir o relatório, e só então")
	fmt.Println("apontar o código do monolito para a tabela unificada payments (sql/03).")
}

func yesNo(b bool) string {
	if b {
		return "sim"
	}
	return "não"
}

func conflictSuffix(status, amount bool) string {
	var parts []string
	if status {
		parts = append(parts, "status-conflito")
	}
	if amount {
		parts = append(parts, "valor-conflito")
	}
	if len(parts) == 0 {
		return ""
	}
	return "  [⚠ " + strings.Join(parts, ", ") + "]"
}
