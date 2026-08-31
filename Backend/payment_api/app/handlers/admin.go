package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/carloshomar/fuudelivery/auth_api/app/middlewares"
	authModels "github.com/carloshomar/fuudelivery/auth_api/app/models"
	"github.com/carloshomar/fuudelivery/payment_api/app/models"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// ============================================================================
// Painel admin — corte 4: todas as consultas agora vêm do Postgres.
// Histórico anterior ao corte só aparece depois de rodar cmd/etl-payments.
// ============================================================================

// ListAllPayments lista os últimos 500 pagamentos (mais recentes primeiro),
// enriquecidos com o nome do cliente (tabela users, consulta em lote).
func ListAllPayments(c *fiber.Ctx) error {
	var payments []models.Payment
	if err := models.DB.Order("created_at DESC").Limit(500).Find(&payments).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Falha ao buscar pagamentos"})
	}

	// Converte para map preservando o contrato JSON antigo (mesmos campos).
	out := make([]map[string]interface{}, 0, len(payments))
	for i := range payments {
		p := paymentToMap(&payments[i])
		out = append(out, p)
	}

	// Enriquece cada pagamento com o nome do cliente (user.nome) lido da
	// tabela users via customer_id — evita N+1 com consulta em lote.
	enrichPaymentsWithUsers(out)

	return c.JSON(out)
}

// paymentToMap serializa o pagamento mantendo as chaves que o WebAdmin
// consome (json tags do modelo já são snake_case legado).
// Campos sensíveis (card_token) são removidos antes de serializar.
func paymentToMap(p *models.Payment) map[string]interface{} {
	b, err := json.Marshal(p)
	if err != nil {
		return map[string]interface{}{}
	}
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		return map[string]interface{}{}
	}
	delete(m, "card_token")
	delete(m, "card_cvv")
	delete(m, "card_expiry")
	return m
}

// enrichPaymentsWithUsers anexa user: {id, nome, phone} a cada pagamento
// consultando os usuarios em lote no Postgres. Se o DB nao estiver disponivel
// ou o usuario nao for encontrado, o campo user e simplesmente omitido — a
// tela faz fallback para customer_phone.
func enrichPaymentsWithUsers(payments []map[string]interface{}) {
	if len(payments) == 0 {
		return
	}
	db := authModels.DB
	if db == nil {
		return
	}

	// Coleta customer_ids unicos
	seen := make(map[int64]bool)
	var ids []int64
	for _, p := range payments {
		cid := paymentCustomerID(p)
		if cid <= 0 || seen[cid] {
			continue
		}
		seen[cid] = true
		ids = append(ids, cid)
	}
	if len(ids) == 0 {
		return
	}

	var users []struct {
		ID    uint
		Name  string
		Phone string
	}
	if err := db.Table("users").Select("id, name, phone").Where("id IN ?", ids).Find(&users).Error; err != nil {
		log.Printf("[PAYMENTS] Falha ao buscar nomes dos clientes: %v", err)
		return
	}

	byID := make(map[int64]struct {
		ID    uint
		Name  string
		Phone string
	}, len(users))
	for _, u := range users {
		byID[int64(u.ID)] = u
	}

	for _, p := range payments {
		cid := paymentCustomerID(p)
		u, ok := byID[cid]
		if !ok || u.Name == "" {
			continue
		}
		p["user"] = map[string]interface{}{
			"id":    u.ID,
			"nome":  u.Name,
			"phone": u.Phone,
		}
	}
}

// paymentCustomerID extrai o customer_id de um pagamento já decodificado
// (JSON numérico chega como float64 após round-trip de interface{}).
func paymentCustomerID(p map[string]interface{}) int64 {
	switch v := p["customer_id"].(type) {
	case int64:
		return v
	case int32:
		return int64(v)
	case float64:
		return int64(v)
	case json.Number:
		if n, err := v.Int64(); err == nil {
			return n
		}
	}
	return 0
}

// GetPaymentStats retorna estatísticas agregadas dos pagamentos (admin).
// GET /payments/stats — tudo via SQL no Postgres (sem varrer linhas na app).
func GetPaymentStats(c *fiber.Ctx) error {
	type counts struct {
		Total, Pending, Confirmed, Rejected int64
		TotalAmount                         float64
	}
	var cs counts

	if err := models.DB.Model(&models.Payment{}).Count(&cs.Total).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Falha nas estatísticas"})
	}
	models.DB.Model(&models.Payment{}).Where("status = ?", "PENDING").Count(&cs.Pending)
	models.DB.Model(&models.Payment{}).Where("status = ?", "CONFIRMED").Count(&cs.Confirmed)
	models.DB.Model(&models.Payment{}).Where("status = ?", "REJECTED").Count(&cs.Rejected)
	// SUM ignora NULL automaticamente; sem linhas retorna NULL → coalesce.
	// ⚠️ Destino separado: o Scan do GORM zera TODA a struct de destino
	// (inclusive campos sem coluna correspondente), então não pode reaproveitar
	// `cs` — senão os counts acima voltam a 0 (bug real pego pelo teste E2E).
	var sumRow struct {
		TotalAmount float64
	}
	if err := models.DB.Model(&models.Payment{}).
		Select("COALESCE(SUM(amount), 0) as total_amount").
		Scan(&sumRow).Error; err == nil {
		cs.TotalAmount = sumRow.TotalAmount
	}

	return c.JSON(fiber.Map{
		"total":        cs.Total,
		"total_amount": cs.TotalAmount,
		"pending":      cs.Pending,
		"confirmed":    cs.Confirmed,
		"rejected":     cs.Rejected,
		"status_counts": fiber.Map{
			"PENDING":   cs.Pending,
			"CONFIRMED": cs.Confirmed,
			"REJECTED":  cs.Rejected,
		},
	})
}

// parsePaymentID aceita somente ID numérico (Postgres BIGSERIAL desde o corte 4).
func parsePaymentID(id string) (int64, error) {
	return strconv.ParseInt(id, 10, 64)
}

// ApprovePayment aprova manualmente um pagamento pendente (admin).
// POST /payments/:id/approve
func ApprovePayment(c *fiber.Ctx) error {
	id := c.Params("id")
	paymentID, err := parsePaymentID(id)
	if err != nil || paymentID <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid payment ID"})
	}

	result := models.DB.Model(&models.Payment{}).
		Where("id = ? AND status = ?", paymentID, "PENDING").
		Updates(map[string]interface{}{
			"status":       "CONFIRMED",
			"confirmed_at": time.Now(),
			"approved_by":  adminIDFromToken(c),
		})
	if result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to approve payment"})
	}
	if result.RowsAffected == 0 {
		// Distingui inexistente de não-pendente para a resposta correta.
		var count int64
		models.DB.Model(&models.Payment{}).Where("id = ?", paymentID).Count(&count)
		if count == 0 {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Payment not found"})
		}
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "Payment is not pending"})
	}

	// Aprovação manual dispara o MESMO fluxo do webhook (split, crédito da
	// carteira, loyalty, fila) — antes o admin aprovava e o estabelecimento
	// nunca era creditado. publishPaymentApproved é idempotente: créditos
	// duplicados são barrados pelo UNIQUE uq_wallet_txns_credit_ref.
	var payment models.Payment
	if err := models.DB.First(&payment, paymentID).Error; err == nil && payment.AbacatePayID != "" {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[ADMIN] Panic in publishPaymentApproved goroutine: %v", r)
				}
			}()
			publishPaymentApproved(payment.AbacatePayID)
		}()
	} else if err != nil {
		log.Printf("[ADMIN] Aprovado %s mas falha ao recarregar pagamento p/ split: %v", id, err)
	}

	return c.JSON(fiber.Map{"message": "Payment approved", "payment_id": id, "status": "CONFIRMED"})
}

// RejectPayment rejeita manualmente um pagamento pendente (admin).
// POST /payments/:id/reject  body: {"reason": "..."}
func RejectPayment(c *fiber.Ctx) error {
	id := c.Params("id")
	paymentID, err := parsePaymentID(id)
	if err != nil || paymentID <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid payment ID"})
	}

	var body struct {
		Reason string `json:"reason"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	result := models.DB.Model(&models.Payment{}).
		Where("id = ? AND status = ?", paymentID, "PENDING").
		Updates(map[string]interface{}{
			"status":           "REJECTED",
			"rejected_at":      time.Now(),
			"rejected_by":      adminIDFromToken(c),
			"rejection_reason": body.Reason,
		})
	if result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to reject payment"})
	}
	if result.RowsAffected == 0 {
		var count int64
		models.DB.Model(&models.Payment{}).Where("id = ?", paymentID).Count(&count)
		if count == 0 {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Payment not found"})
		}
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "Payment is not pending"})
	}

	return c.JSON(fiber.Map{"message": "Payment rejected", "payment_id": id, "status": "REJECTED"})
}

// adminIDFromToken devolve o id do admin autenticado como string ("" se ausente).
func adminIDFromToken(c *fiber.Ctx) string {
	if uid, err := middlewares.GetUserIDFromToken(c); err == nil {
		return fmt.Sprintf("%d", uid)
	}
	return ""
}

// ListWallets lista todas as carteiras (admin), mais recentes primeiro.
func ListWallets(c *fiber.Ctx) error {
	var wallets []models.Wallet
	if err := models.DB.Order("updated_at DESC").Limit(500).Find(&wallets).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Falha ao listar carteiras"})
	}

	out := make([]map[string]interface{}, 0, len(wallets))
	for _, w := range wallets {
		m := map[string]interface{}{
			"id":           w.ID,
			"user_id":      w.UserID,
			"user_type":    w.UserType,
			"owner_type":   w.UserType, // alias usado pelo painel Financeiro
			"balance":      w.Balance,
			"currency":     w.Currency,
			"status":       w.Status,
			"last_updated": w.LastUpdated,
		}
		out = append(out, m)
	}
	return c.JSON(out)
}

// ledgerFilter representa os filtros de query de /chargebacks, reaproveitados
// tanto pela listagem quanto pelo resumo agregado.
type ledgerFilter struct {
	Type       string
	UserID     int64
	HasUserID  bool
	PaymentRef string
	Limit      int
}

// parseLedgerFilter extrai os filtros comuns da querystring.
func parseLedgerFilter(c *fiber.Ctx) ledgerFilter {
	f := ledgerFilter{Limit: 100}
	f.Type = c.Query("type")
	if uid := c.Query("user_id"); uid != "" {
		if id, err := strconv.ParseInt(uid, 10, 64); err == nil {
			f.UserID = id
			f.HasUserID = true
		}
	}
	f.PaymentRef = c.Query("payment_id")
	if l := c.QueryInt("limit"); l > 0 && l <= 500 {
		f.Limit = l
	}
	return f
}

// applyLedgerFilter aplica os filtros numa query de WalletTxn. O user_id vive
// na tabela wallets (join), e reference_id guarda payment_id/order_id de origem.
func applyLedgerFilter(q *gorm.DB, f ledgerFilter) *gorm.DB {
	q = q.Model(&models.WalletTxn{}).
		Joins("JOIN wallets ON wallets.id = wallet_transactions.wallet_id")
	if f.Type != "" {
		q = q.Where("wallet_transactions.type = ?", f.Type)
	}
	if f.HasUserID {
		q = q.Where("wallets.user_id = ?", f.UserID)
	}
	if f.PaymentRef != "" {
		q = q.Where("wallet_transactions.reference_id = ?", f.PaymentRef)
	}
	return q
}

// ListChargebacks lista os lançamentos (créditos/débitos) do ledger para o
// painel Financeiro do WebAdmin (admin).
// GET /chargebacks?type=debit&user_id=42&payment_id=...&limit=100
//
// Cada lançamento é enriquecido com o owner_type da carteira (join direto).
func ListChargebacks(c *fiber.Ctx) error {
	f := parseLedgerFilter(c)

	var rows []struct {
		models.WalletTxn
		WalletUserID int64
		OwnerType    string
	}
	if err := applyLedgerFilter(models.DB.
		Select(`wallet_transactions.*, wallets.user_id AS wallet_user_id, wallets.user_type AS owner_type`), f).
		Order("wallet_transactions.created_at DESC").
		Limit(f.Limit).
		Scan(&rows).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Falha ao listar lançamentos do ledger"})
	}

	entries := make([]map[string]interface{}, 0, len(rows))
	for _, r := range rows {
		e := map[string]interface{}{
			"id":         strconv.FormatInt(r.ID, 10),
			"type":       r.Type,
			"amount":     r.Amount,
			"owner_type": r.OwnerType,
			"user_id":    r.WalletUserID,
		}
		if r.Kind == "withdrawal" {
			e["kind"] = r.Kind
			e["destination"] = r.Destination
		}
		if r.Description != "" {
			e["description"] = r.Description
		}
		if r.ReferenceID != "" {
			e["payment_id"] = r.ReferenceID
		}
		e["balance_after"] = r.BalanceAfter
		e["created_at"] = r.CreatedAt.Format(time.RFC3339)
		entries = append(entries, e)
	}

	// Resumo agregado com os MESMOS filtros da listagem (ignora o limit).
	summary := computeLedgerSummary(f)

	return c.JSON(fiber.Map{
		"chargebacks": entries,
		"count":       len(entries),
		"summary":     summary,
	})
}

// computeLedgerSummary agrega o ledger pelos filtros informados e retorna os
// totais de créditos, débitos e saldo líquido (SQL GROUP BY, sem carregar rows).
func computeLedgerSummary(f ledgerFilter) fiber.Map {
	type agg struct {
		CreditTotal float64
		DebitTotal  float64
	}
	var a agg
	err := applyLedgerFilter(models.DB.
		Select(`COALESCE(SUM(CASE WHEN wallet_transactions.type = 'credit' THEN wallet_transactions.amount ELSE 0 END), 0) AS credit_total,
		       COALESCE(SUM(CASE WHEN wallet_transactions.type = 'debit' THEN wallet_transactions.amount ELSE 0 END), 0) AS debit_total`), f).
		Scan(&a).Error
	if err != nil {
		log.Printf("[LEDGER] Falha no resumo agregado: %v", err)
		return fiber.Map{"credit_total": 0.0, "debit_total": 0.0, "net": 0.0}
	}

	return fiber.Map{
		"credit_total": a.CreditTotal,
		"debit_total":  a.DebitTotal,
		"net":          a.CreditTotal - a.DebitTotal,
	}
}
