// Package services - wallet_service.go
// Servico de carteiras digitais (wallets).
// Gerencia o saldo, credito, debito e historico de transacoes.
// Quando um pagamento e aprovado, o valor e creditado na carteira do restaurante.
package services

import (
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

// CreditWallet credita um valor na carteira do usuario.
// Registra a transacao com saldo antes/depois para auditoria.
// Usado quando um pagamento e aprovado.
func (ws *WalletService) CreditWallet(userID string, amount float64, description string, referenceID string) error {
	// Busca a carteira do usuario
	wallet, err := repository.GetWallet(userID)
	if err != nil {
		return err
	}

	// Calcula o novo saldo
	balanceBefore := wallet.Balance
	balanceAfter := balanceBefore + amount

	// Atualiza o saldo no banco
	if err := repository.UpdateWalletBalance(userID, balanceAfter); err != nil {
		return err
	}

	// Registra a transacao para auditoria
	tx := &models.WalletTransaction{
		WalletID:      wallet.ID,
		Type:          models.TransactionCredit,
		Amount:        amount,
		BalanceBefore: balanceBefore,
		BalanceAfter:  balanceAfter,
		Description:   description,
		ReferenceID:   referenceID,
	}

	return repository.CreateWalletTransaction(tx)
}

// DebitWallet debita um valor da carteira do usuario.
// Verifica se ha saldo suficiente antes de debitar.
// Usado para estornos ou correcoes.
func (ws *WalletService) DebitWallet(userID string, amount float64, description string, referenceID string) error {
	// Busca a carteira do usuario
	wallet, err := repository.GetWallet(userID)
	if err != nil {
		return err
	}

	// Verifica saldo suficiente
	if wallet.Balance < amount {
		return nil // Saldo insuficiente: nao debita
	}

	// Calcula o novo saldo
	balanceBefore := wallet.Balance
	balanceAfter := balanceBefore - amount

	// Atualiza o saldo no banco
	if err := repository.UpdateWalletBalance(userID, balanceAfter); err != nil {
		return err
	}

	// Registra a transacao para auditoria
	tx := &models.WalletTransaction{
		WalletID:      wallet.ID,
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
