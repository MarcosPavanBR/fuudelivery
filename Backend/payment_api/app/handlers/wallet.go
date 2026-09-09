package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/carloshomar/fuudelivery/auth_api/app/middlewares"
	"github.com/carloshomar/fuudelivery/payment_api/app/dto"
	"github.com/carloshomar/fuudelivery/payment_api/app/models"
	"github.com/gofiber/fiber/v2"
)

// Referências de débito no ledger são namespaceadas por operação E POR DONO.
//
// Motivo original: o índice de sql/18 era UNIQUE só em reference_id, sem
// wallet_id, o que fazia do reference_id um espaço GLOBAL. Uma referência não
// escopada ficava atacável — mandar um order_id/chave que outra pessoa já
// usou provocava violação do índice e, com o handler tratando isso como
// "replay", ele respondia sucesso sem mover dinheiro nenhum: falso sucesso
// para o chamador e bloqueio da operação alheia.
//
// sql/20 reescopou o índice para (wallet_id, reference_id), então hoje o
// banco já isola por carteira. O namespacing continua porque as duas camadas
// protegem coisas diferentes: o índice garante unicidade DENTRO da carteira,
// e o prefixo garante que um saque (`wd:`) nunca colida com uma dedução de
// pedido (`ord:`) da MESMA carteira por reusar o mesmo identificador.
func withdrawRef(estID int64, idemKey string) string {
	return fmt.Sprintf("wd:%d:%s", estID, idemKey)
}

func deductRef(userID int64, orderID string) string {
	return fmt.Sprintf("ord:%d:%s", userID, orderID)
}

// withdrawDedupWindow é a janela do fallback de idempotência para clientes
// que não mandam Idempotency-Key. Curta de propósito: pega duplo clique e
// retry automático, sem bloquear um segundo saque legítimo do mesmo valor
// para a mesma chave PIX minutos depois.
const withdrawDedupWindow = 60 * time.Second

// derivedWithdrawKey monta uma chave determinística por janela de tempo.
// Duas requisições idênticas dentro da mesma janela produzem a mesma chave e
// a segunda esbarra no índice de idempotência.
//
// destination é normalizado (trim + minúsculas) porque um espaço a mais na
// chave PIX geraria uma chave diferente e deixaria o duplo clique passar.
func derivedWithdrawKey(estID int64, amount float64, destination string, now time.Time) string {
	return derivedWithdrawKeyForBucket(estID, amount, destination,
		now.Unix()/int64(withdrawDedupWindow.Seconds()))
}

func derivedWithdrawKeyForBucket(estID int64, amount float64, destination string, bucket int64) string {
	dest := strings.ToLower(strings.TrimSpace(destination))
	raw := fmt.Sprintf("%d|%.2f|%s|%d", estID, amount, dest, bucket)
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:16])
}

// derivedWithdrawKeyCandidates devolve a chave da janela atual e a da janela
// anterior.
//
// Motivo: o bucket é fixo (unix/60), não deslizante. Duas requisições
// separadas por 1 segundo podem cair em buckets diferentes (ex.: 59.9s e
// 60.1s) e as duas passariam. Testando também o bucket anterior, o duplo
// clique é barrado mesmo em cima da virada.
func derivedWithdrawKeyCandidates(estID int64, amount float64, destination string, now time.Time) []string {
	bucket := now.Unix() / int64(withdrawDedupWindow.Seconds())
	return []string{
		derivedWithdrawKeyForBucket(estID, amount, destination, bucket),
		derivedWithdrawKeyForBucket(estID, amount, destination, bucket-1),
	}
}

// ============================================================================
// Carteiras — corte 4: todas as movimentações passam por
// models.AdjustWalletBalance (transação + SELECT FOR UPDATE + ledger),
// garantindo atomicidade que o Mongo legado não tinha. Saldo legado do
// Mongo é semeado na primeira movimentação (ensureWalletSeeded).
// ============================================================================

// GetBalance retorna o saldo da carteira do próprio usuário autenticado.
func GetBalance(c *fiber.Ctx) error {
	tokenUserID, err := middlewares.GetUserIDFromToken(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid token"})
	}

	userIDStr := c.Params("user_id")

	var reqUserID int64
	if _, scanErr := fmt.Sscanf(userIDStr, "%d", &reqUserID); scanErr != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid user_id"})
	}

	if tokenUserID != reqUserID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Cannot view another user's balance"})
	}

	wallet, wErr := ensureWalletSeeded(models.DB, reqUserID, "customer")
	if wErr != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Wallet not found", "balance": 0})
	}

	return c.Status(200).JSON(fiber.Map{
		"user_id":      wallet.UserID,
		"balance":      wallet.Balance,
		"last_updated": wallet.LastUpdated,
	})
}

// TopUp credita na carteira do cliente o valor de um pagamento CONFIRMADO.
// Idempotente: claim atômico de wallet_credited_at + UNIQUE no ledger
// (uq_wallet_txns_credit_ref) — replays concorrentes recebem 409.
func TopUp(c *fiber.Ctx) error {
	tokenUserID, err := middlewares.GetUserIDFromToken(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid token"})
	}

	var req dto.WalletTopUpRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if req.Amount <= 0 {
		return c.Status(400).JSON(fiber.Map{"error": "Amount must be greater than zero"})
	}

	if req.PaymentID == "" {
		return c.Status(400).JSON(fiber.Map{"error": "payment_id is required for wallet top-up"})
	}

	if tokenUserID != req.UserID {
		log.Printf("[WALLET] TopUp rejected: token user %d != body user %d", tokenUserID, req.UserID)
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Cannot top up another user's wallet"})
	}

	payment, pErr := findPaymentByAbacatePayID(req.PaymentID)
	if pErr != nil {
		log.Printf("[WALLET] TopUp rejected: payment %s not found (user=%d)", req.PaymentID, req.UserID)
		return c.Status(404).JSON(fiber.Map{"error": "Payment not found"})
	}

	if payment.Status != "CONFIRMED" {
		log.Printf("[WALLET] TopUp rejected: payment %s status=%s, expected CONFIRMED", req.PaymentID, payment.Status)
		return c.Status(402).JSON(fiber.Map{"error": "Payment not confirmed", "status": payment.Status})
	}

	if payment.CustomerID != req.UserID {
		log.Printf("[WALLET] TopUp rejected: payment %s belongs to user %d, requested by user %d", req.PaymentID, payment.CustomerID, req.UserID)
		return c.Status(403).JSON(fiber.Map{"error": "Payment does not belong to this user"})
	}

	// Claim atômico: só UMA requisição consegue marcar wallet_credited_at
	// (UPDATE ... WHERE IS NULL). Replays concorrentes do mesmo pagamento
	// caem no RowsAffected == 0 abaixo, sem depender de checagem prévia.
	res := models.DB.Model(payment).
		Where("id = ? AND wallet_credited_at IS NULL", payment.ID).
		Update("wallet_credited_at", time.Now())
	if res.Error != nil {
		log.Printf("[WALLET] TopUp failed: claim de %s: %v", req.PaymentID, res.Error)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to top up wallet"})
	}
	if res.RowsAffected == 0 {
		log.Printf("[WALLET] TopUp rejected: payment %s already used for wallet credit", req.PaymentID)
		return c.Status(409).JSON(fiber.Map{"error": "Payment already used for wallet top-up"})
	}

	newWallet, aErr := models.AdjustWalletBalance(
		models.DB, req.UserID, "customer",
		"credit", "", payment.Amount,
		req.PaymentID,
		"Wallet top-up via confirmed payment",
		"",
	)
	if errors.Is(aErr, models.ErrDuplicateCredit) {
		return c.Status(409).JSON(fiber.Map{"error": "Payment already used for wallet top-up"})
	}
	if aErr != nil {
		log.Printf("[WALLET] TopUp failed: user=%d payment=%s: %v", req.UserID, req.PaymentID, aErr)
		// Libera o claim para permitir nova tentativa (o crédito não entrou).
		models.DB.Model(payment).Where("id = ?", payment.ID).Update("wallet_credited_at", nil)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to top up wallet"})
	}
	wallet := newWallet

	log.Printf("[WALLET] TopUp OK: user=%d amount=%.2f payment=%s new_balance=%.2f", req.UserID, payment.Amount, req.PaymentID, wallet.Balance)

	return c.Status(200).JSON(fiber.Map{
		"user_id":      req.UserID,
		"balance":      wallet.Balance,
		"amount_added": payment.Amount,
		"message":      "Wallet topped up successfully",
	})
}

// DeductFromWallet debita um valor da carteira do usuário para pagar pedido.
// Atômico com guarda de saldo (nunca fica negativo).
func DeductFromWallet(c *fiber.Ctx) error {
	tokenUserID, err := middlewares.GetUserIDFromToken(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid token"})
	}

	var req struct {
		UserID  int64   `json:"user_id"`
		Amount  float64 `json:"amount"`
		OrderID string  `json:"order_id,omitempty"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if req.Amount <= 0 {
		return c.Status(400).JSON(fiber.Map{"error": "Amount must be greater than zero"})
	}

	if tokenUserID != req.UserID {
		log.Printf("[WALLET] Deduct rejected: token user %d != body user %d", tokenUserID, req.UserID)
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Cannot deduct from another user's wallet"})
	}

	// order_id é a chave de idempotência do débito. Vazio significa cair fora
	// do índice parcial uq_wallet_txns_debit_ref_wallet e permitir débito duplicado
	// no mesmo pedido — além de gerar lançamento sem origem rastreável no
	// ledger. Melhor recusar do que debitar sem proteção.
	if strings.TrimSpace(req.OrderID) == "" {
		return c.Status(400).JSON(fiber.Map{"error": "order_id é obrigatório para debitar da carteira"})
	}

	walletType := walletTypeForUser(req.UserID)
	ref := deductRef(req.UserID, req.OrderID)

	// Compatibilidade com lançamentos anteriores ao namespacing.
	//
	// Antes desta mudança a referência gravada era o order_id puro. Um débito
	// feito ANTES do deploy e reenviado DEPOIS geraria `ord:<uid>:<pedido>`,
	// que não colide com a linha antiga — e o mesmo pedido seria debitado duas
	// vezes, exatamente o que a idempotência existe para impedir. A janela é
	// estreita (um retry atravessando o deploy), mas é dinheiro.
	//
	// Some sozinho quando não houver mais lançamento em formato antigo.
	if legacy, lErr := models.FindLedgerEntry(models.DB, req.OrderID, "debit", req.UserID); lErr == nil {
		if !models.SameAmount(legacy.Amount, req.Amount) {
			log.Printf("[WALLET] Deduct: pedido %s já debitado (formato legado) com valor diferente: registrado=%.2f pedido=%.2f",
				req.OrderID, legacy.Amount, req.Amount)
			return c.Status(409).JSON(fiber.Map{
				"error":           "Pedido já debitado com valor diferente",
				"amount_recorded": legacy.Amount,
			})
		}
		log.Printf("[WALLET] Deduct idempotente (lançamento legado): user=%d order=%s", req.UserID, req.OrderID)
		wallet, wErr := models.GetWallet(models.DB, req.UserID, walletType)
		if wErr != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to deduct from wallet"})
		}
		return c.Status(200).JSON(fiber.Map{
			"user_id":         req.UserID,
			"balance":         wallet.Balance,
			"amount_deducted": req.Amount,
			"message":         "Amount deducted successfully",
			"idempotent":      true,
		})
	}

	newWallet, dErr := models.AdjustWalletBalance(
		models.DB, req.UserID, walletType,
		"debit", "", req.Amount,
		ref,
		"Wallet deduction",
		"",
	)
	if dErr == models.ErrInsufficientBalance {
		return c.Status(400).JSON(fiber.Map{"error": "Insufficient balance or wallet not found"})
	}
	if errors.Is(dErr, models.ErrDuplicateDebit) {
		// Replay do mesmo pedido: o saldo não foi debitado de novo (rollback).
		//
		// Antes de responder sucesso, confirma que o lançamento existente é
		// DESTA carteira. A referência já é namespaceada por usuário, então
		// isso deveria ser sempre verdade — mas "deveria ser verdade por
		// construção" é exatamente o tipo de suposição que transforma colisão
		// de índice em falso sucesso. Se não bater, é erro, não replay.
		entry, eErr := models.FindLedgerEntry(models.DB, ref, "debit", req.UserID)
		if eErr != nil {
			log.Printf("[WALLET] Deduct: violação de idempotência sem lançamento próprio: user=%d ref=%s: %v", req.UserID, ref, eErr)
			return c.Status(500).JSON(fiber.Map{"error": "Failed to deduct from wallet"})
		}
		// A chave de idempotência é o pedido, e ela NÃO carrega o valor. Sem
		// esta comparação, debitar 0,01 e depois 100,00 no mesmo order_id
		// devolvia 200 "debitado com sucesso" com amount_deducted=100,00 e
		// nada saía da carteira — o consumidor daria o pedido por pago.
		if !models.SameAmount(entry.Amount, req.Amount) {
			log.Printf("[WALLET] Deduct: mesmo order_id com valor diferente: user=%d order=%s registrado=%.2f pedido=%.2f",
				req.UserID, req.OrderID, entry.Amount, req.Amount)
			return c.Status(409).JSON(fiber.Map{
				"error":           "Pedido já debitado com valor diferente",
				"amount_recorded": entry.Amount,
			})
		}
		log.Printf("[WALLET] Deduct idempotente (replay): user=%d order=%s", req.UserID, req.OrderID)
		// Leitura pura: o replay não pode criar carteira que não existia.
		wallet, wErr := models.GetWallet(models.DB, req.UserID, walletType)
		if wErr != nil {
			log.Printf("[WALLET] Replay de deduct: falha ao ler saldo: user=%d: %v", req.UserID, wErr)
			return c.Status(500).JSON(fiber.Map{"error": "Failed to deduct from wallet"})
		}
		return c.Status(200).JSON(fiber.Map{
			"user_id":         req.UserID,
			"balance":         wallet.Balance,
			"amount_deducted": req.Amount,
			"message":         "Amount deducted successfully",
			"idempotent":      true,
		})
	}
	if dErr != nil {
		log.Printf("[WALLET] Deduct failed: user=%d: %v", req.UserID, dErr)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to deduct from wallet"})
	}

	log.Printf("[WALLET] Deduct OK: user=%d amount=%.2f new_balance=%.2f", req.UserID, req.Amount, newWallet.Balance)

	return c.Status(200).JSON(fiber.Map{
		"user_id":         req.UserID,
		"balance":         newWallet.Balance,
		"amount_deducted": req.Amount,
		"message":         "Amount deducted successfully",
	})
}

// establishmentLedgerTotals soma os créditos (total ganho) e os débitos de
// saque (kind == "withdrawal") do ledger de um estabelecimento via SQL.
func establishmentLedgerTotals(estID int64) (earned, withdrawn float64) {
	var res struct {
		Earned    float64
		Withdrawn float64
	}
	err := models.DB.Model(&models.WalletTxn{}).
		Joins("JOIN wallets ON wallets.id = wallet_transactions.wallet_id").
		Where("wallets.user_id = ? AND wallets.user_type = ?", estID, "establishment").
		Select(`COALESCE(SUM(CASE WHEN wallet_transactions.type = 'credit' THEN wallet_transactions.amount ELSE 0 END), 0) AS earned,
		       COALESCE(SUM(CASE WHEN wallet_transactions.kind = 'withdrawal' THEN wallet_transactions.amount ELSE 0 END), 0) AS withdrawn`).
		Scan(&res).Error
	if err != nil {
		log.Printf("[WALLET] Falha ao somar ledger do estabelecimento %d: %v", estID, err)
		return 0, 0
	}
	return res.Earned, res.Withdrawn
}

// GetEstablishmentWallet retorna o saldo da carteira do estabelecimento
// autenticado (papel restaurante no WebRestaurant).
// GET /wallet/establishment/balance
//
// pending/blocked são 0: o modelo atual só tem saldo disponível;
// total_earned/total_withdrawn vêm do ledger.
func GetEstablishmentWallet(c *fiber.Ctx) error {
	estID, err := middlewares.GetEstablishmentIDFromToken(c)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Establishment ID not found in token"})
	}

	wallet, wErr := ensureWalletSeeded(models.DB, estID, "establishment")
	available := 0.0
	var lastUpdated time.Time
	if wErr == nil {
		available = wallet.Balance
		lastUpdated = wallet.LastUpdated
	}

	earned, withdrawn := establishmentLedgerTotals(estID)

	return c.JSON(fiber.Map{
		"user_id":         estID,
		"available":       available,
		"pending":         0.0,
		"blocked":         0.0,
		"total_earned":    earned,
		"total_withdrawn": withdrawn,
		"last_updated":    lastUpdated,
	})
}

// GetEstablishmentTransactions lista o extrato do estabelecimento autenticado,
// paginado por cursor (ID numérico em string — antes era ObjectID hex; o
// frontend trata cursor como opaco, então a troca é transparente).
// GET /wallet/establishment/transactions?limit=20&cursor=...
//
// Retorna { data, next_cursor, has_more }. Cada item expõe type em
// MAIÚSCULAS (CREDIT/DEBIT/WITHDRAWAL) para o frontend colorir o extrato.
func GetEstablishmentTransactions(c *fiber.Ctx) error {
	estID, err := middlewares.GetEstablishmentIDFromToken(c)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Establishment ID not found in token"})
	}

	limit := 20
	if l := c.QueryInt("limit"); l > 0 && l <= 100 {
		limit = l
	}

	q := models.DB.Model(&models.WalletTxn{}).
		Joins("JOIN wallets ON wallets.id = wallet_transactions.wallet_id").
		Where("wallets.user_id = ? AND wallets.user_type = ?", estID, "establishment")

	if cursorStr := c.Query("cursor"); cursorStr != "" {
		cursorID, cErr := strconv.ParseInt(cursorStr, 10, 64)
		if cErr != nil || cursorID <= 0 {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid cursor"})
		}
		q = q.Where("wallet_transactions.id < ?", cursorID)
	}

	var rows []models.WalletTxn
	if err := q.Order("wallet_transactions.id DESC").Limit(limit + 1).Find(&rows).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to list transactions"})
	}

	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}

	data := make([]map[string]interface{}, 0, len(rows))
	for _, r := range rows {
		item := map[string]interface{}{
			"id":          strconv.FormatInt(r.ID, 10),
			"description": r.Description,
			"amount":      r.Amount,
			"balance":     r.BalanceAfter,
			"created_at":  r.CreatedAt.Format(time.RFC3339),
			"payment_ref": r.ReferenceID,
		}
		switch {
		case r.Kind == "withdrawal":
			item["type"] = "WITHDRAWAL"
		case r.Type == "credit":
			item["type"] = "CREDIT"
		default:
			item["type"] = "DEBIT"
		}
		data = append(data, item)
	}

	nextCursor := ""
	if hasMore && len(rows) > 0 {
		nextCursor = strconv.FormatInt(rows[len(rows)-1].ID, 10)
	}

	return c.JSON(fiber.Map{
		"data":        data,
		"next_cursor": nextCursor,
		"has_more":    hasMore,
	})
}

// EstablishmentWithdraw processa um saque da carteira do estabelecimento
// autenticado (papel restaurante). Débito atômico com guarda de saldo e
// lançamento no ledger (type=debit, kind=withdrawal, destination=chave PIX).
// POST /wallet/establishment/withdraw  body: {amount, destination, method}
func EstablishmentWithdraw(c *fiber.Ctx) error {
	estID, err := middlewares.GetEstablishmentIDFromToken(c)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Establishment ID not found in token"})
	}

	var req struct {
		Amount         float64 `json:"amount"`
		Destination    string  `json:"destination"`
		Method         string  `json:"method"`
		IdempotencyKey string  `json:"idempotency_key"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if req.Amount < 10.0 {
		return c.Status(400).JSON(fiber.Map{"error": "Valor mínimo para saque: R$ 10,00"})
	}
	if req.Destination == "" || len(req.Destination) < 10 {
		return c.Status(400).JSON(fiber.Map{"error": "Informe uma chave PIX ou dados bancários válidos"})
	}
	if req.Method == "" {
		req.Method = "PIX"
	}

	// Chave de idempotência: sem ela o saque passava "" como reference_id e
	// ficava FORA do índice parcial uq_wallet_txns_debit_ref_wallet (que exclui
	// referência vazia) — duplo submit sacava duas vezes, limitado só pelo
	// saldo. Cliente novo manda Idempotency-Key; cliente antigo cai no
	// fallback derivado, que barra o duplo clique acidental sem impedir um
	// segundo saque legítimo depois da janela.
	idemKey := strings.TrimSpace(c.Get("Idempotency-Key"))
	if idemKey == "" {
		idemKey = strings.TrimSpace(req.IdempotencyKey)
	}
	if idemKey == "" {
		// Fallback derivado: checa a janela atual e a anterior antes de
		// debitar. Só o índice do banco não basta aqui porque a chave da
		// janela seguinte é diferente — sem esta checagem, um duplo clique em
		// cima da virada do bucket passaria com duas chaves distintas.
		//
		// Colisão aqui NÃO é replay idempotente — ao contrário da colisão de
		// Idempotency-Key (validada abaixo contra valor E destino, que prova
		// ser a MESMA requisição lógica), o fallback derivado só hashia
		// (estabelecimento, valor, destino, bucket). Responder "Saque
		// solicitado com sucesso" sem mover dinheiro fazia o dono acreditar
		// que sacou duas vezes: falso sucesso é o pior resultado possível em
		// operação financeira. A resposta honesta é 409 — o cliente sabe que
		// o segundo saque NÃO aconteceu e que o primeiro está em andamento.
		for _, candidate := range derivedWithdrawKeyCandidates(estID, req.Amount, req.Destination, time.Now()) {
			if models.HasLedgerEntry(models.DB, withdrawRef(estID, candidate), "debit", estID) {
				log.Printf("[WALLET] Saque duplicado na janela derivada (sem Idempotency-Key): establishment=%d", estID)
				return c.Status(fiber.StatusConflict).JSON(fiber.Map{
					"error":               "Saque idêntico já solicitado há menos de 1 minuto. Aguarde a confirmação ou envie uma Idempotency-Key para repetir de forma segura.",
					"retry_after_seconds": 60,
				})
			}
		}
		idemKey = derivedWithdrawKey(estID, req.Amount, req.Destination, time.Now())
	}

	description := fmt.Sprintf("Saque via %s para %s", req.Method, req.Destination)
	ref := withdrawRef(estID, idemKey)
	newWallet, dErr := models.AdjustWalletBalance(
		models.DB, estID, "establishment",
		"debit", "withdrawal", req.Amount,
		ref,
		description,
		req.Destination,
	)
	if dErr == models.ErrInsufficientBalance {
		return c.Status(400).JSON(fiber.Map{"error": "Saldo insuficiente para este saque"})
	}
	if errors.Is(dErr, models.ErrDuplicateDebit) {
		// Replay: este saque já foi registrado antes. O rollback da transação
		// garantiu que o saldo não foi debitado de novo, então a resposta
		// correta é sucesso com o saldo atual — não 500.
		//
		// Confirma antes que o lançamento é DESTE estabelecimento: a
		// referência já inclui o estID, mas se por algum motivo a colisão vier
		// de outro dono, responder "saque efetuado" seria mentira — e o saque
		// real teria sido silenciosamente descartado.
		entry, eErr := models.FindLedgerEntry(models.DB, ref, "debit", estID)
		if eErr != nil {
			log.Printf("[WALLET] Saque: violação de idempotência sem lançamento próprio: establishment=%d: %v", estID, eErr)
			return c.Status(500).JSON(fiber.Map{"error": "Falha ao processar saque"})
		}
		// A Idempotency-Key é escolhida pelo cliente e não carrega valor nem
		// destino. Reusá-la com outro valor (ou outra chave PIX) recebia
		// "Saque solicitado com sucesso" sem que saque nenhum acontecesse.
		// O fallback derivado não tem esse furo porque já hasheia os dois.
		sameDestination := strings.EqualFold(strings.TrimSpace(entry.Destination), strings.TrimSpace(req.Destination))
		if !models.SameAmount(entry.Amount, req.Amount) || !sameDestination {
			log.Printf("[WALLET] Saque: Idempotency-Key reusada com dados diferentes: establishment=%d registrado=%.2f pedido=%.2f",
				estID, entry.Amount, req.Amount)
			return c.Status(409).JSON(fiber.Map{
				"error":           "Idempotency-Key já usada para um saque diferente",
				"amount_recorded": entry.Amount,
			})
		}
		log.Printf("[WALLET] Saque idempotente (replay): establishment=%d", estID)
		// Leitura pura: o replay não pode criar carteira que não existia.
		wallet, wErr := models.GetWallet(models.DB, estID, "establishment")
		if wErr != nil {
			log.Printf("[WALLET] Replay de saque: falha ao ler saldo: establishment=%d: %v", estID, wErr)
			return c.Status(500).JSON(fiber.Map{"error": "Falha ao processar saque"})
		}
		return c.JSON(fiber.Map{
			"message":    "Saque solicitado com sucesso",
			"balance":    wallet.Balance,
			"idempotent": true,
		})
	}
	if dErr != nil {
		log.Printf("[WALLET] Saque falhou: establishment=%d: %v", estID, dErr)
		return c.Status(500).JSON(fiber.Map{"error": "Falha ao processar saque"})
	}

	log.Printf("[WALLET] Saque OK: establishment=%d amount=%.2f method=%s novo_saldo=%.2f", estID, req.Amount, req.Method, newWallet.Balance)

	return c.JSON(fiber.Map{
		"message": "Saque solicitado com sucesso",
		"balance": newWallet.Balance,
	})
}
