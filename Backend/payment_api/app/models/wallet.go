package models

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ============================================================================
// Carteiras e ledger — CORTE 4 banco-único.
//
// Wallet mapeia a tabela `wallets` e WalletTxn a tabela `wallet_transactions`
// (sql/03_dominio_pagamentos.sql). O ledger é o registro contábil imutável:
// NUNCA faça UPDATE/DELETE nele — estorno é um lançamento de sinal contrário.
//
// A coluna extra `kind` (ex.: "withdrawal") foi adicionada pelo script
// sql/09_wallet_ledger_kind.sql para preservar a classificação dos débitos
// que existia na collection legada wallet_ledger do Mongo.
// ============================================================================

// Wallet — tabela `wallets`. UNIQUE(user_id, user_type).
type Wallet struct {
	ID          int64     `gorm:"primaryKey;column:id" json:"id"`
	UserID      int64     `gorm:"column:user_id;uniqueIndex:idx_wallet_user" json:"user_id"`
	UserType    string    `gorm:"column:user_type;uniqueIndex:idx_wallet_user" json:"user_type"` // restaurant | delivery | establishment | customer
	Balance     float64   `gorm:"column:balance" json:"balance"`
	Currency    string    `gorm:"column:currency;default:BRL" json:"currency"`
	Status      string    `gorm:"column:status;default:active" json:"status"`
	LastUpdated time.Time `gorm:"column:updated_at" json:"last_updated"`
}

// TableName fixa o nome da tabela.
func (Wallet) TableName() string { return "wallets" }

// WalletTxn — lançamento do ledger (tabela `wallet_transactions`).
// Equivalente ao documento antigo da collection wallet_ledger do Mongo:
//   - ReferenceID guarda o payment_id ou order_id de origem;
//   - Kind classifica o débito ("" normal | "withdrawal" saque);
//   - Destination guarda a chave PIX do saque (coluna do script 10).
type WalletTxn struct {
	ID            int64     `gorm:"primaryKey;column:id" json:"id"`
	WalletID      int64     `gorm:"column:wallet_id" json:"wallet_id"`
	Type          string    `gorm:"column:type" json:"type"` // credit | debit
	Kind          string    `gorm:"column:kind" json:"kind,omitempty"`
	Amount        float64   `gorm:"column:amount" json:"amount"`
	BalanceBefore float64   `gorm:"column:balance_before" json:"balance_before"`
	BalanceAfter  float64   `gorm:"column:balance_after" json:"balance_after"`
	Description   string    `gorm:"column:description" json:"description,omitempty"`
	ReferenceID   string    `gorm:"column:reference_id" json:"payment_id,omitempty"`
	Destination   string    `gorm:"column:destination" json:"destination,omitempty"`
	UserID        int64     `gorm:"-" json:"user_id"`            // preenchido para leitura (join feito no handler)
	OrderRef      string    `gorm:"-" json:"order_id,omitempty"` // alias quando a referência era um pedido
	CreatedAt     time.Time `gorm:"column:created_at" json:"created_at"`
}

// TableName fixa o nome da tabela.
func (WalletTxn) TableName() string { return "wallet_transactions" }

var ErrInsufficientBalance = errors.New("saldo insuficiente")

// GetOrCreateWallet devolve a carteira do usuário, criando-a com saldo zero
// se ainda não existir. Usa ON CONFLICT para evitar corrida entre requests.
func GetOrCreateWallet(db *gorm.DB, userID int64, userType string) (*Wallet, error) {
	wallet := &Wallet{UserID: userID, UserType: userType, Balance: 0, Currency: "BRL", Status: "active"}
	err := db.
		Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "user_id"}, {Name: "user_type"}}, DoNothing: true}).
		Create(wallet).Error
	if err != nil {
		return nil, fmt.Errorf("criar carteira %d/%s: %w", userID, userType, err)
	}
	if err := db.Where("user_id = ? AND user_type = ?", userID, userType).First(wallet).Error; err != nil {
		return nil, fmt.Errorf("carregar carteira %d/%s: %w", userID, userType, err)
	}
	return wallet, nil
}

// AdjustWalletBalance aplica um crédito ou débito ATOMICAMENTE:
// abre transação, trava a linha da carteira (SELECT ... FOR UPDATE),
// valida saldo no débito, atualiza e insere o lançamento no ledger.
// Toda movimentação de dinheiro DEVE passar por aqui — nunca atualize
// ErrDuplicateCredit indica que já existe lançamento de crédito para a
// referência (violacao de uq_wallet_txns_credit_ref) — o crédito já foi
// aplicado antes e a operação atual é um replay idempotente.
var ErrDuplicateCredit = errors.New("lançamento de crédito duplicado para a referência")

// isUniqueViolation detecta erro 23505 do Postgres, opcionalmente filtrando
// pelo nome da constraint.
func isUniqueViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505" && (constraint == "" || pgErr.ConstraintName == constraint)
	}
	return false
}

// balance direto com UPDATE solto.
//
// referenceID identifica a origem (payment_id ou order_id) e é usado como
// chave de idempotência pelos chamadores (checagem prévia no ledger).
func AdjustWalletBalance(db *gorm.DB, userID int64, userType, txnType, kind string, amount float64, referenceID, description, destination string) (*Wallet, error) {
	if amount <= 0 {
		return nil, errors.New("valor precisa ser positivo")
	}

	var updated *Wallet
	err := db.Transaction(func(tx *gorm.DB) error {
		wallet, wErr := GetOrCreateWallet(tx, userID, userType)
		if wErr != nil {
			return wErr
		}

		// Trava a linha para nenhuma outra request ler saldo velho.
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", wallet.ID).First(wallet).Error; err != nil {
			return err
		}

		newBalance := wallet.Balance
		switch txnType {
		case "credit":
			newBalance += amount
		case "debit":
			if wallet.Balance < amount {
				return ErrInsufficientBalance
			}
			newBalance -= amount
		default:
			return fmt.Errorf("tipo de lançamento inválido: %s", txnType)
		}

		before := wallet.Balance
		if err := tx.Model(wallet).Updates(map[string]interface{}{
			"balance":    newBalance,
			"updated_at": time.Now(),
		}).Error; err != nil {
			return err
		}
		wallet.Balance = newBalance

		entry := WalletTxn{
			WalletID:      wallet.ID,
			Type:          txnType,
			Kind:          kind,
			Amount:        amount,
			BalanceBefore: before,
			BalanceAfter:  newBalance,
			Description:   description,
			ReferenceID:   referenceID,
			Destination:   destination,
		}
		if err := tx.Create(&entry).Error; err != nil {
			// Crédito duplicado = replay de webhook/top-up concorrente. A tx
			// inteira (incluindo o UPDATE do saldo) é desfeita pelo rollback.
			if txnType == "credit" && isUniqueViolation(err, "uq_wallet_txns_credit_ref") {
				return ErrDuplicateCredit
			}
			return err
		}

		updated = wallet
		return nil
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// HasLedgerEntry checa idempotência: já existe lançamento deste tipo para a
// referência? Usado para o crédito de split não duplicar num reprocesso do
// webhook (mesmo papel do CountDocuments antigo no wallet_ledger).
func HasLedgerEntry(db *gorm.DB, referenceID, txnType string) bool {
	var count int64
	err := db.Model(&WalletTxn{}).
		Joins("JOIN wallets ON wallets.id = wallet_transactions.wallet_id").
		Where("wallet_transactions.reference_id = ? AND wallet_transactions.type = ?", referenceID, txnType).
		Count(&count).Error
	if err != nil {
		log.Printf("[LEDGER] WARNING: falha ao checar idempotência ref=%s: %v", referenceID, err)
		return false // em caso de erro, permite tentar (comportamento fail-open igual ao legado)
	}
	return count > 0
}
