package handlers

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/carloshomar/fuudelivery/payment_api/app/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
	"gorm.io/gorm"
)

// ============================================================================
// Helpers da migração banco-único (CORTE 4 — docs/ARQUITETURA-BANCO-UNICO.md).
//
// Estratégia adotada para os pagamentos (dinheiro!):
//   1. Escrita: Postgres é a fonte da verdade; toda escrita relevante é
//      espelhada no Mongo legado (dual-write best-effort) para que um
//      rollback de deploy não perca dados novos.
//   2. Leitura pontual (por abacatepay_id): Postgres primeiro; se não
//      achar, lê do Mongo e COPIA preguiçosamente para o Postgres
//      ("lazy ETL") — cobre pagamentos criados antes do corte.
//   3. Carteiras: na primeira movimentação pós-corte, o saldo existente
//      no Mongo é semeado no Postgres com lançamento de auditoria
//      (ensureWalletSeeded) — evita "saldo zerado" para quem já tinha
//      carteira antes da migração.
//   4. Histórico em massa (relatórios/painel admin): somente Postgres.
//      Rodar cmd/etl-payments uma vez para trazer o histórico completo.
//
// Para desligar o Mongo definitivamente: remover os blocos marcados com
// "DUAL-WRITE LEGADO", os helpers dualWrite* deste arquivo e a chamada
// ConnectMongoDatabase em cmd/fuudelivery/main.go.
// ============================================================================

// mongoCtx devolve um contexto com timeout para as operações legadas de
// dual-write no MongoDB. (Definição única — movida para cá no corte 4;
// admin.go usa a mesma função.)
//
// O cancelamento é agendado via time.AfterFunc em vez de defer: o helper
// retorna o contexto para o chamador, então o cancel precisa disparar
// DEPOIS que a operação Mongo rodar — o timer de 5s cuida disso sozinho.
func mongoCtx() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(5*time.Second, cancel)
	return ctx
}

// walletTypeForUser descobre o user_type da carteira existente do usuário
// (qualquer tipo). Default "customer" quando não há carteira ainda — usado
// nos fluxos de estorno, onde o tipo real vem da carteira já semeada.
func walletTypeForUser(userID int64) string {
	var wallet models.Wallet
	if err := models.DB.Where("user_id = ?", userID).First(&wallet).Error; err == nil && wallet.UserType != "" {
		return wallet.UserType
	}
	return "customer"
}

// dualWritePaymentUpsert espelha o pagamento na collection legada "payments"
// do Mongo (filtro por abacatepay_id). Best-effort: erro é logado, nunca
// quebra o fluxo principal — o Postgres é quem manda.
func dualWritePaymentUpsert(p *models.Payment) {
	if models.MongoDabase == nil || p.AbacatePayID == "" {
		return
	}
	update := bson.M{"$set": bson.M{
		"order_id":           p.OrderID,
		"customer_id":        p.CustomerID,
		"customer_phone":     p.CustomerPhone,
		"establishment_id":   p.EstablishmentID,
		"amount":             p.Amount,
		"delivery_amount":    p.DeliveryAmount,
		"method":             p.Method,
		"status":             p.Status,
		"pix_qr_code":        p.PixQRCode,
		"pix_copy_paste":     p.PixCopyPaste,
		"qr_code_base64":     p.QRCodeBase64,
		"abacatepay_id":      p.AbacatePayID,
		"card_last_digits":   p.CardLastDigits,
		"installments":       p.Installments,
		"split_rules":        p.SplitRules,
		"confirmed_at":       p.ConfirmedAt,
		"wallet_credited_at": p.WalletCreditedAt,
		"refunded_at":        p.RefundedAt,
	}}
	if _, err := models.MongoDabase.Collection("payments").
		UpdateOne(mongoCtx(), bson.M{"abacatepay_id": p.AbacatePayID}, update, options.Update().SetUpsert(true)); err != nil {
		log.Printf("[DUAL-WRITE] Mongo payments %s: %v (ignorado)", p.AbacatePayID, err)
	}
}

// findPaymentByAbacatePayID localiza o pagamento pelo ID externo do gateway.
// Postgres primeiro; em caso de "não encontrado", tenta o Mongo legado e
// copia o registro para o Postgres (lazy ETL) para que as próximas leituras
// e atualizações já aconteçam na fonte nova.
func findPaymentByAbacatePayID(abacatepayID string) (*models.Payment, error) {
	var payment models.Payment
	err := models.DB.Where("abacatepay_id = ?", abacatepayID).First(&payment).Error
	if err == nil {
		return &payment, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, err
	}

	// Fallback legado: decodifica o documento Mongo campo a campo.
	if models.MongoDabase != nil {
		var doc bson.M
		findErr := models.MongoDabase.Collection("payments").
			FindOne(mongoCtx(), bson.M{"abacatepay_id": abacatepayID}).Decode(&doc)
		if findErr == nil {
			p := paymentFromLegacyDoc(doc)
			if createErr := models.DB.Create(p).Error; createErr != nil {
				log.Printf("[LAZY-ETL] Pagamento %s já existe no Postgres ou falhou ao copiar: %v", abacatepayID, createErr)
				// Recarrega: outra request pode ter copiado em paralelo.
				if reloadErr := models.DB.Where("abacatepay_id = ?", abacatepayID).First(&payment).Error; reloadErr == nil {
					return &payment, nil
				}
			}
			log.Printf("[LAZY-ETL] Pagamento %s migrado do Mongo para o Postgres", abacatepayID)
			return p, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

// getF64FromAny extrai float64 de tipos numéricos do BSON.
func getF64FromAny(v interface{}) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int32:
		return float64(n)
	case int64:
		return float64(n)
	}
	return 0
}

// getI64FromAny extrai int64 de tipos numéricos do BSON.
func getI64FromAny(v interface{}) int64 {
	switch n := v.(type) {
	case int32:
		return int64(n)
	case int64:
		return n
	case float64:
		return int64(n)
	}
	return 0
}

// paymentFromLegacyDoc converte um documento legado do Mongo (bson.M) para o
// modelo GORM. Campos ausentes ficam com o valor zero — o importante é
// preservar identidade, valores, status e os carimbos de tempo usados nas
// regras de negócio (confirmação, crédito de carteira, estorno).
func paymentFromLegacyDoc(doc bson.M) *models.Payment {
	p := &models.Payment{Method: "", Status: "PENDING"}
	getStr := func(key string) string { s, _ := doc[key].(string); return s }
	getF64 := func(key string) float64 { f, _ := doc[key].(float64); return f }
	getI64 := func(key string) int64 {
		switch v := doc[key].(type) {
		case int32:
			return int64(v)
		case int64:
			return v
		case float64:
			return int64(v)
		}
		return 0
	}
	getTime := func(key string) *time.Time {
		switch v := doc[key].(type) {
		case time.Time:
			return &v
		case primitive.DateTime:
			t := v.Time()
			return &t
		}
		return nil
	}

	p.OrderID = getStr("order_id")
	p.CustomerPhone = getStr("customer_phone")
	p.Method = getStr("method")
	if s := getStr("status"); s != "" {
		p.Status = s
	}
	p.Amount = getF64("amount")
	p.DeliveryAmount = getF64("delivery_amount")
	p.CustomerID = getI64("customer_id")
	p.EstablishmentID = getI64("establishment_id")
	p.PixQRCode = getStr("pix_qr_code")
	p.PixCopyPaste = getStr("pix_copy_paste")
	p.QRCodeBase64 = getStr("qr_code_base64")
	p.TicketURL = getStr("ticket_url")
	p.AbacatePayID = getStr("abacatepay_id")
	p.CardLastDigits = getStr("card_last_digits")
	p.CardToken = getStr("card_token")
	p.Installments = int(getI64("installments"))
	if t := getTime("created_at"); t != nil {
		p.CreatedAt = *t
	} else {
		p.CreatedAt = time.Now()
	}
	p.ConfirmedAt = getTime("confirmed_at")
	p.WalletCreditedAt = getTime("wallet_credited_at")
	p.EstablishmentCreditedAt = getTime("establishment_credited_at")
	p.RefundedAt = getTime("refunded_at")

	// Split rules legadas: array de documentos {receiver_id, receiver_type, amount, percentage}.
	if raw, ok := doc["split_rules"].(bson.A); ok {
		for _, item := range raw {
			if m, ok := item.(bson.M); ok {
				rule := models.SplitRule{
					ReceiverID:   getI64FromAny(m["receiver_id"]),
					ReceiverType: getStringFromAny(m["receiver_type"]),
					Amount:       getF64FromAny(m["amount"]),
					Percentage:   getF64FromAny(m["percentage"]),
				}
				p.SplitRules = append(p.SplitRules, rule)
			}
		}
	}
	return p
}

// getStringFromAny extrai string de um valor BSON.
func getStringFromAny(v interface{}) string {
	s, _ := v.(string)
	return s
}

// ensureWalletSeeded garante que a carteira exista no Postgres ANTES da
// primeira movimentação pós-corte. Se ela ainda não existe aqui mas existe
// no Mongo legado, o saldo é semeado com um lançamento de auditoria
// (reference_id = "legacy-migration") — nunca inventamos saldo: ele vem do
// documento legado. Retorna a carteira pronta para uso.
func ensureWalletSeeded(db *gorm.DB, userID int64, userType string) (*models.Wallet, error) {
	var wallet models.Wallet
	err := db.Where("user_id = ? AND user_type = ?", userID, userType).First(&wallet).Error
	if err == nil {
		return &wallet, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, err
	}

	legacyBalance := 0.0
	hasLegacy := false
	if models.MongoDabase != nil {
		var doc bson.M
		// O legado gravou user_id ora como número ora como string — testa os dois.
		findErr := models.MongoDabase.Collection("wallets").
			FindOne(mongoCtx(), bson.M{"user_id": userID}).Decode(&doc)
		if findErr != nil {
			findErr = models.MongoDabase.Collection("wallets").
				FindOne(mongoCtx(), bson.M{"user_id": fmt.Sprintf("%d", userID)}).Decode(&doc)
		}
		if findErr == nil {
			if b, ok := doc["balance"].(float64); ok && b > 0 {
				legacyBalance = b
				hasLegacy = true
			}
		}
	}

	wallet = models.Wallet{
		UserID:      userID,
		UserType:    userType,
		Balance:     legacyBalance,
		Currency:    "BRL",
		Status:      "active",
		LastUpdated: time.Now(),
	}
	if err := db.Create(&wallet).Error; err != nil {
		// Corrida com outra request: recarrega.
		if reloadErr := db.Where("user_id = ? AND user_type = ?", userID, userType).First(&wallet).Error; reloadErr == nil {
			return &wallet, nil
		}
		return nil, err
	}

	if hasLegacy {
		entry := models.WalletTxn{
			WalletID:      wallet.ID,
			Type:          "credit",
			Kind:          "",
			Amount:        legacyBalance,
			BalanceBefore: 0,
			BalanceAfter:  legacyBalance,
			Description:   "Migração banco-único: saldo legado importado do MongoDB",
			ReferenceID:   "legacy-migration",
		}
		if err := db.Create(&entry).Error; err != nil {
			// Saldo já criado; falha de ledger é logada mas não trava o uso.
			log.Printf("[LAZY-ETL] WARNING: falha ao gravar lançamento de migração wallet=%d: %v", wallet.ID, err)
		}
		log.Printf("[LAZY-ETL] Carteira %d/%s semeada com saldo legado %.2f", userID, userType, legacyBalance)
	}
	return &wallet, nil
}

// dualWriteWallet espelha o estado final da carteira no Mongo legado após
// uma movimentação bem-sucedida no Postgres (best-effort).
func dualWriteWallet(w *models.Wallet) {
	if models.MongoDabase == nil {
		return
	}
	_, err := models.MongoDabase.Collection("wallets").UpdateOne(
		mongoCtx(),
		bson.M{"user_id": w.UserID},
		bson.M{"$set": bson.M{
			"user_id":      w.UserID,
			"user_type":    w.UserType,
			"balance":      w.Balance,
			"last_updated": time.Now(),
		}}, options.Update().SetUpsert(true),
	)
	if err != nil {
		log.Printf("[DUAL-WRITE] Mongo wallets %d: %v (ignorado)", w.UserID, err)
	}
}

// dualWriteLedgerEntry espelha um lançamento do ledger no Mongo legado
// (collection wallet_ledger) no formato antigo dos documentos (best-effort).
func dualWriteLedgerEntry(userID int64, txnType, kind string, amount, balanceAfter float64, refID, description, destination string) {
	if models.MongoDabase == nil {
		return
	}
	entry := bson.M{
		"user_id":       userID,
		"type":          txnType,
		"amount":        amount,
		"payment_id":    refID,
		"balance_after": balanceAfter,
		"description":   description,
		"created_at":    time.Now(),
	}
	if kind != "" {
		entry["kind"] = kind
	}
	if destination != "" {
		entry["destination"] = destination
	}
	if _, err := models.MongoDabase.Collection("wallet_ledger").InsertOne(mongoCtx(), entry); err != nil {
		log.Printf("[DUAL-WRITE] Mongo wallet_ledger user=%d: %v (ignorado)", userID, err)
	}
}
