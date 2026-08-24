// ============================================================================
// etl-payments — migração de dados legados do MongoDB para o Postgres
// (CORTE 4 banco-único — docs/ARQUITETURA-BANCO-UNICO.md).
//
// O que faz (one-shot, idempotente — pode rodar mais de uma vez):
//
//  1. payments: copia cada documento da collection "payments" para a tabela
//     payments. Dedup por abacatepay_id (upsert manual) e por order_id+amount
//     para registros sem ID externo.
//  2. wallets: cria no Postgres qualquer carteira que exista só no Mongo.
//     NUNCA sobrescreve saldo já existente no Postgres (o Postgres manda).
//     O saldo importado ganha lançamento de auditoria no ledger
//     (reference_id = "etl-legacy-import").
//  3. wallet_ledger: importa os lançamentos antigos para wallet_transactions,
//     vinculando à carteira do usuário no Postgres. Dedup pela tupla
//     (wallet_id, type, kind, amount, reference_id, created_at).
//
// Como rodar (as env vars são as mesmas do monolito):
//
//	DB_CONNECTION_STRING="postgres://..." \
//	MONGO_URI="mongodb+srv://..." \
//	PAYMENT_MONGO_DATABASE="fuudelivery_payments" \
//	go run ./cmd/etl-payments
//
// Segurança: NÃO apaga nada em nenhum dos bancos. Rode antes de desligar o
// Mongo Atlas e confira os totais impressos ao final.
// ============================================================================
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/carloshomar/fuudelivery/payment_api/app/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"gorm.io/gorm"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// --- Conexões ---
	mongoURI := os.Getenv("MONGO_URI")
	if mongoURI == "" {
		log.Fatal("MONGO_URI não configurado — informe o Mongo Atlas legado")
	}
	dbName := os.Getenv("PAYMENT_MONGO_DATABASE")
	if dbName == "" {
		dbName = "fuudelivery_payments"
	}

	mc, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		log.Fatalf("conectar no Mongo: %v", err)
	}
	defer mc.Disconnect(ctx)
	if err := mc.Ping(ctx, nil); err != nil {
		log.Fatalf("ping no Mongo falhou: %v", err)
	}
	legacy := mc.Database(dbName)
	// NUNCA logar a URI inteira: contém usuário/senha do Atlas (P0 de segurança).
	log.Printf("[ETL] Mongo legado: db=%s (URI configurada)", dbName)

	models.ConnectPostgresDatabase() // mesmo padrão de retry do monolito
	db := models.DB

	// --- 1. Pagamentos ---
	payments := legacy.Collection("payments")
	cursor, err := payments.Find(ctx, bson.M{})
	if err != nil {
		log.Fatalf("buscar pagamentos: %v", err)
	}
	var imported, skipped int
	for cursor.Next(ctx) {
		var doc bson.M
		if err := cursor.Decode(&doc); err != nil {
			log.Printf("[ETL] pagamento inválido ignorado: %v", err)
			continue
		}
		p := paymentFromDoc(doc)
		if p.OrderID == "" && p.AbacatePayID == "" {
			skipped++
			continue
		}

		// Dedup: já existe no Postgres?
		q := db.Model(&models.Payment{})
		if p.AbacatePayID != "" {
			q = q.Where("abacatepay_id = ?", p.AbacatePayID)
		} else {
			q = q.Where("order_id = ? AND amount = ?", p.OrderID, p.Amount)
		}
		var count int64
		q.Count(&count)
		if count > 0 {
			continue
		}
		if err := db.Create(p).Error; err != nil {
			log.Printf("[ETL] WARNING: falha ao inserir pagamento %s/%s: %v", p.AbacatePayID, p.OrderID, err)
			continue
		}
		imported++
	}
	cursor.Close(ctx)
	log.Printf("[ETL] Pagamentos: %d importados, %d inválidos ignorados", imported, skipped)

	// --- 2 + 3. Carteiras e ledger ---
	wallets := legacy.Collection("wallets")
	wCursor, err := wallets.Find(ctx, bson.M{})
	if err != nil {
		log.Fatalf("buscar carteiras: %v", err)
	}
	var walletsCreated, ledgerImported int
	for wCursor.Next(ctx) {
		var doc bson.M
		if err := wCursor.Decode(&doc); err != nil {
			continue
		}
		userID := toI64(doc["user_id"])
		if userID <= 0 {
			continue
		}
		userType, _ := doc["user_type"].(string)
		balance := toF64(doc["balance"])

		// Carteira já existe no Postgres? (nunca sobrescreve saldo — o Postgres manda)
		var wallet models.Wallet
		err := db.Where("user_id = ? AND user_type = ?", userID, userType).First(&wallet).Error
		if err == nil {
			// Já migrada: só garante o ledger histórico desta carteira.
			ledgerImported += importLedger(ctx, legacy, db, &wallet)
			continue
		}

		wallet = models.Wallet{
			UserID: userID, UserType: userType,
			Balance: balance, Currency: "BRL", Status: "active",
			LastUpdated: time.Now(),
		}
		if err := db.Create(&wallet).Error; err != nil {
			log.Printf("[ETL] WARNING: carteira %d/%s: %v", userID, userType, err)
			continue
		}
		walletsCreated++

		// Lançamento de auditoria do saldo semeado (se houver saldo).
		if balance > 0 {
			entry := models.WalletTxn{
				WalletID: wallet.ID, Type: "credit",
				Amount: balance, BalanceAfter: balance,
				Description: "ETL banco-único: saldo legado importado do MongoDB",
				ReferenceID: "etl-legacy-import",
				CreatedAt:   time.Now(),
			}
			if err := db.Create(&entry).Error; err != nil {
				log.Printf("[ETL] WARNING: ledger de migração wallet=%d: %v", wallet.ID, err)
			}
		}

		ledgerImported += importLedger(ctx, legacy, db, &wallet)
	}
	wCursor.Close(ctx)

	fmt.Println("\n===== RESUMO DO ETL =====")
	fmt.Printf("Pagamentos importados:      %d\n", imported)
	fmt.Printf("Carteiras criadas:          %d\n", walletsCreated)
	fmt.Printf("Lançamentos de ledger:      %d\n", ledgerImported)
	fmt.Println("==========================")
	fmt.Println("Nada foi apagado. Após validar os números acima, o MongoDB pode ser pausado/desligado.")
}

// importLedger copia os lançamentos antigos do usuário (collection
// wallet_ledger, filtro user_id) para wallet_transactions, deduplicando pela
// tupla (type, kind, amount, reference_id, created_at). Retorna quantos
// entraram. Best-effort: erro individual é logado e continua.
func importLedger(ctx context.Context, legacy *mongo.Database, db *gorm.DB, wallet *models.Wallet) int {
	col := legacy.Collection("wallet_ledger")
	cursor, err := col.Find(ctx, bson.M{"user_id": wallet.UserID})
	if err != nil {
		log.Printf("[ETL] WARNING: ledger do user %d: %v", wallet.UserID, err)
		return 0
	}
	defer cursor.Close(ctx)

	imported := 0
	for cursor.Next(ctx) {
		var doc bson.M
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		typ, _ := doc["type"].(string)
		kind, _ := doc["kind"].(string)
		amount := toF64(doc["amount"])
		refID, _ := doc["payment_id"].(string)
		if refID == "" {
			refID, _ = doc["order_id"].(string)
		}
		description, _ := doc["description"].(string)
		destination, _ := doc["destination"].(string)
		balanceAfter := toF64(doc["balance_after"])

		var createdAt time.Time
		switch v := doc["created_at"].(type) {
		case time.Time:
			createdAt = v
		default:
			createdAt = time.Now()
		}

		// Dedup pela tupla característica do lançamento.
		q := db.Model(&models.WalletTxn{}).
			Where("wallet_id = ? AND type = ? AND amount = ? AND created_at = ?",
				wallet.ID, typ, amount, createdAt)
		if kind != "" {
			q = q.Where("kind = ?", kind)
		} else {
			q = q.Where("(kind IS NULL OR kind = '')")
		}
		if refID != "" {
			q = q.Where("reference_id = ?", refID)
		}
		var count int64
		q.Count(&count)
		if count > 0 {
			continue
		}

		entry := models.WalletTxn{
			WalletID: wallet.ID, Type: typ, Kind: kind,
			Amount:       amount,
			BalanceAfter: balanceAfter,
			Description:  description, ReferenceID: refID, Destination: destination,
			CreatedAt: createdAt,
		}
		if err := db.Create(&entry).Error; err != nil {
			log.Printf("[ETL] WARNING: lançamento wallet=%d: %v", wallet.ID, err)
			continue
		}
		imported++
	}
	return imported
}

// paymentFromDoc converte o documento legado em modelo GORM (mesma lógica do
// lazy ETL dos handlers — mantenha as duas em sincronia se mudar campos).
func paymentFromDoc(doc bson.M) *models.Payment {
	p := &models.Payment{Status: "PENDING"}
	getStr := func(k string) string { s, _ := doc[k].(string); return s }
	getTime := func(k string) *time.Time {
		switch v := doc[k].(type) {
		case time.Time:
			return &v
		}
		return nil
	}

	p.OrderID = getStr("order_id")
	p.CustomerPhone = getStr("customer_phone")
	p.Method = getStr("method")
	if s := getStr("status"); s != "" {
		p.Status = s
	}
	p.Amount = toF64(doc["amount"])
	p.DeliveryAmount = toF64(doc["delivery_amount"])
	p.CustomerID = toI64(doc["customer_id"])
	p.EstablishmentID = toI64(doc["establishment_id"])
	p.PixQRCode = getStr("pix_qr_code")
	p.PixCopyPaste = getStr("pix_copy_paste")
	p.QRCodeBase64 = getStr("qr_code_base64")
	p.TicketURL = getStr("ticket_url")
	p.AbacatePayID = getStr("abacatepay_id")
	p.CardLastDigits = getStr("card_last_digits")
	// CardToken NÃO é migrado: PAN/dados sensíveis de cartão não podem ser
	// copiados para o novo banco (PCI). Mantém apenas os últimos dígitos.
	if n := toI64(doc["installments"]); n > 0 {
		p.Installments = int(n)
	}
	if t := getTime("created_at"); t != nil {
		p.CreatedAt = *t
	} else {
		p.CreatedAt = time.Now()
	}
	p.ConfirmedAt = getTime("confirmed_at")
	p.WalletCreditedAt = getTime("wallet_credited_at")
	p.EstablishmentCreditedAt = getTime("establishment_credited_at")
	p.RefundedAt = getTime("refunded_at")

	if raw, ok := doc["split_rules"].(bson.A); ok {
		for _, item := range raw {
			if m, ok := item.(bson.M); ok {
				rule := models.SplitRule{
					ReceiverID:   toI64(m["receiver_id"]),
					ReceiverType: getStr2(m["receiver_type"]),
					Amount:       toF64(m["amount"]),
					Percentage:   toF64(m["percentage"]),
				}
				p.SplitRules = append(p.SplitRules, rule)
			}
		}
	}
	return p
}

func getStr2(v interface{}) string { s, _ := v.(string); return s }

func toF64(v interface{}) float64 {
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

func toI64(v interface{}) int64 {
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
