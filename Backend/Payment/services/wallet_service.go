// Package services - wallet_service.go
// Servico de carteiras digitais (wallets).
// Gerencia o saldo, credito, debito e historico de transacoes.
// Quando um pagamento e aprovado, o valor e creditado na carteira do restaurante.
package services

import (
	"fmt"
	"time"

	"github.com/carloshomar/vercardapio/payment/models"
	"github.com/carloshomar/vercardapio/payment/repository"
)

// WalletService e responsavel pelas operacoes de carteira.
type WalletService struct{}

// NewWalletService cria uma nova instancia do servico de carteiras.
func NewWalletService() *WalletService {
	return &WalletService{}
}

// GetOrCreateWallet busca a carteira de um usuario.
// Se nao existir, cria uma nova com saldo zero.
// Retorna a carteira (existente ou criada).
func (ws *WalletService) GetOrCreateWallet(userID, userType string) (*models.Wallet, error) {
	// Tenta buscar carteira existente
	wallet, err := repository.GetWallet(userID)
	if err == nil {
		return wallet, nil
	}

	// Carteira nao existe: cria uma nova
	wallet = &models.Wallet{
		UserID:   userID,
		UserType: userType,
		Balance:  0,
		Currency: "BRL",
		Status:   "active",
	}

	if err := repository.CreateWallet(wallet); err != nil {
		return nil, err
	}

	return wallet, nil
}

// CreditWallet credita um valor na carteira do usuario de forma atomica
// ($inc no MongoDB, via repository.IncrementWalletBalance) — nao ha mais
// leitura-depois-escrita separadas, entao duas chamadas concorrentes para
// o mesmo usuario nunca perdem uma atualizacao.
// Registra a transacao com saldo antes/depois para auditoria.
// Usado quando um pagamento e aprovado.
func (ws *WalletService) CreditWallet(userID string, amount float64, description string, referenceID string) error {
	if amount <= 0 {
		return fmt.Errorf("valor de credito deve ser positivo: %.2f", amount)
	}

	walletAfter, err := repository.IncrementWalletBalance(userID, amount)
	if err != nil {
		return err
	}
	balanceAfter := walletAfter.Balance
	balanceBefore := balanceAfter - amount

	// Registra a transacao para auditoria
	tx := &models.WalletTransaction{
		WalletID:      walletAfter.ID,
		Type:          models.TransactionCredit,
		Amount:        amount,
		BalanceBefore: balanceBefore,
		BalanceAfter:  balanceAfter,
		Description:   description,
		ReferenceID:   referenceID,
	}

	return repository.CreateWalletTransaction(tx)
}

// DebitWallet debita um valor da carteira do usuario de forma atomica.
// A checagem de saldo suficiente e o desconto acontecem na mesma operacao
// do banco (repository.TryDebitWalletBalance), entao duas chamadas
// concorrentes nunca conseguem debitar mais do que a carteira tem.
//
// IMPORTANTE: se o saldo for insuficiente, este metodo retorna
// repository.ErrInsufficientBalance — NAO retorna nil. Chamadores devem
// tratar erro explicitamente; um retorno nil aqui sempre significa que o
// debito realmente aconteceu.
func (ws *WalletService) DebitWallet(userID string, amount float64, description string, referenceID string) error {
	if amount <= 0 {
		return fmt.Errorf("valor de debito deve ser positivo: %.2f", amount)
	}

	walletAfter, err := repository.TryDebitWalletBalance(userID, amount)
	if err != nil {
		// Inclui repository.ErrInsufficientBalance quando nao ha saldo —
		// o chamador decide o que fazer (bloquear pedido, avisar usuario etc.),
		// mas o erro nunca e engolido silenciosamente.
		return err
	}
	balanceAfter := walletAfter.Balance
	balanceBefore := balanceAfter + amount

	// Registra a transacao para auditoria
	tx := &models.WalletTransaction{
		WalletID:      walletAfter.ID,
		Type:          models.TransactionDebit,
		Amount:        amount,
		BalanceBefore: balanceBefore,
		BalanceAfter:  balanceAfter,
		Description:   description,
		ReferenceID:   referenceID,
	}

	return repository.CreateWalletTransaction(tx)
}

// GetBalance retorna o saldo atual da carteira de um usuario.
func (ws *WalletService) GetBalance(userID string) (float64, error) {
	wallet, err := repository.GetWallet(userID)
	if err != nil {
		return 0, err
	}
	return wallet.Balance, nil
}

// GetTransactions retorna o historico de transacoes da carteira.
func (ws *WalletService) GetTransactions(userID string, limit int) ([]models.WalletTransaction, error) {
	wallet, err := repository.GetWallet(userID)
	if err != nil {
		return nil, err
	}

	return repository.GetWalletTransactions(wallet.ID, limit)
}

// ProcessPaymentApproval processa a aprovacao de um pagamento.
// Credit o valor liquido (valor - taxa de entrega) na carteira do restaurante.
// Este metodo e chamado quando um pagamento e aprovado.
func (ws *WalletService) ProcessPaymentApproval(payment *models.Payment) error {
	// So processa pagamentos aprovados
	if payment.Status != models.PaymentApproved {
		return nil
	}

	// Calcula o valor liquido (descontando taxa de entrega)
	establishmentID := payment.EstablishmentID
	amount := payment.Amount - payment.DeliveryAmount

	// Credit na carteira do restaurante
	if err := ws.CreditWallet(establishmentID, amount, "Payment received for order "+payment.OrderID, payment.OrderID); err != nil {
		return err
	}

	// Registra quando o credito foi feito
	now := time.Now()
	return repository.UpdatePaymentStatus(payment.ID, models.PaymentApproved, map[string]interface{}{
		"wallet_credited_at": now,
	})
}
