// Package services - wallet_service.go
// Servico de carteiras digitais (wallets).
// Gerencia o saldo, credito, debito e historico de transacoes.
// Quando um pagamento e aprovado, o valor e creditado na carteira do restaurante.
package services

import (
	"fmt"
	"log"
	"time"

	"github.com/carloshomar/vercardapio/payment/models"
	"github.com/carloshomar/vercardapio/payment/repository"
	"go.mongodb.org/mongo-driver/bson"
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
// Credita o valor liquido (valor - taxa de entrega) na carteira do restaurante.
// Idempotente: reserva atomicamente o order_id antes de creditar (ver
// repository.ClaimOrderProcessing), entao chamadas concorrentes ou
// reentregas da fila para o mesmo pedido nunca creditam duas vezes.
// Este metodo e chamado quando um pagamento e aprovado.
func (ws *WalletService) ProcessPaymentApproval(payment *models.Payment) error {
	if payment.Status != models.PaymentApproved {
		return nil
	}

	if payment.OrderID == "" {
		return fmt.Errorf("payment sem order_id: nao e possivel garantir idempotencia do credito")
	}

	// Reserva atomica: a checagem "ja existe transacao?" antiga (contar
	// depois decidir) tinha uma janela de corrida entre duas chamadas
	// simultaneas para o mesmo order_id. ClaimOrderProcessing usa o indice
	// unico do _id do MongoDB pra fechar essa janela — so uma chamada
	// consegue reservar, nao importa quantas cheguem ao mesmo tempo.
	if err := repository.ClaimOrderProcessing(payment.OrderID); err != nil {
		if err == repository.ErrOrderAlreadyProcessed {
			return nil
		}
		return fmt.Errorf("erro ao reservar idempotencia: %w", err)
	}

	establishmentID := payment.EstablishmentID
	amount := payment.Amount - payment.DeliveryAmount

	if amount <= 0 {
		return nil
	}

	// Credita o valor atomicamente com $inc
	now := time.Now()
	walletAfter, err := repository.IncrementWalletBalance(establishmentID, amount)
	if err != nil {
		return fmt.Errorf("erro ao creditar carteira: %w", err)
	}

	// Registra a transacao para auditoria com saldos reais
	balanceAfter := walletAfter.Balance
	balanceBefore := balanceAfter - amount

	tx := &models.WalletTransaction{
		WalletID:      walletAfter.ID,
		Type:          models.TransactionCredit,
		Amount:        amount,
		BalanceBefore: balanceBefore,
		BalanceAfter:  balanceAfter,
		Description:   "Payment received for order " + payment.OrderID,
		ReferenceID:   payment.OrderID,
		CreatedAt:     now,
	}

	if err := repository.CreateWalletTransaction(tx); err != nil {
		// A transacao de auditoria falhou, mas o $inc ja aconteceu.
		// Loga o warning e continua — o saldo esta correto.
		log.Printf("Warning: falha ao registrar transacao de auditoria para order %s: %v", payment.OrderID, err)
	}

	return repository.UpdatePaymentStatus(payment.ID, models.PaymentApproved, bson.M{
		"wallet_credited_at": now,
	})
}

// RequestPixWithdrawal solicita um saque Pix da carteira do usuario.
// Fluxo:
// 1. Valida o saldo e debita atomically (TryDebitWalletBalance)
// 2. Registra a transacao de debito na carteira
// 3. Envia o Pix via AbacatePay (POST /pix/create)
// 4. Salva o registro de saque para auditoria
// Em caso de falha no gateway, o valor e estornado para a carteira.
func (ws *WalletService) RequestPixWithdrawal(userID string, amount float64, pixKey, pixKeyType string) (*models.PayoutRequest, error) {
	if amount <= 0 {
		return nil, fmt.Errorf("valor de saque deve ser positivo: %.2f", amount)
	}
	if pixKey == "" {
		return nil, fmt.Errorf("chave Pix obrigatoria")
	}

	// PASSO 1: Busca carteira para saber saldo e tipo de usuario
	wallet, err := repository.GetWallet(userID)
	if err != nil {
		return nil, fmt.Errorf("carteira nao encontrada: %w", err)
	}

	// Valor minimo de saque (evita micropagamentos que consomem taxa)
	if amount < 5.00 {
		return nil, fmt.Errorf("valor minimo de saque e R$ 5,00")
	}

	// PASSO 2: Debita o valor atomicamente (checagem de saldo + debito na mesma operacao)
	walletAfter, err := repository.TryDebitWalletBalance(userID, amount)
	if err != nil {
		if err == repository.ErrInsufficientBalance {
			return nil, fmt.Errorf("saldo insuficiente: disponivel R$ %.2f, solicitado R$ %.2f", wallet.Balance, amount)
		}
		return nil, fmt.Errorf("erro ao debitar carteira: %w", err)
	}
	balanceAfter := walletAfter.Balance
	balanceBefore := balanceAfter + amount

	// PASSO 3: Registra transacao de debito para auditoria
	tx := &models.WalletTransaction{
		WalletID:      walletAfter.ID,
		Type:          models.TransactionDebit,
		Amount:        amount,
		BalanceBefore: balanceBefore,
		BalanceAfter:  balanceAfter,
		Description:   fmt.Sprintf("Saque Pix - %.2f", amount),
		ReferenceID:   "pix_" + time.Now().Format("20060102150405"),
		CreatedAt:     time.Now(),
	}
	if err := repository.CreateWalletTransaction(tx); err != nil {
		return nil, fmt.Errorf("erro ao registrar transacao: %w", err)
	}

	// PASSO 4: Cria registro de saque
	payout := &models.PayoutRequest{
		UserID:        userID,
		UserType:      wallet.UserType,
		Amount:        amount,
		PixKey:        pixKey,
		PixKeyType:    pixKeyType,
		Status:        models.PayoutPending,
		BalanceBefore: balanceBefore,
		BalanceAfter:  balanceAfter,
		TransactionID: tx.ID,
	}
	if err := repository.CreatePayout(payout); err != nil {
		// Falhou ao salvar o registro, mas o dinheiro ja foi debitado.
		// Loga warning e tenta estornar.
		log.Printf("CRITICAL: saque realizado mas registro falhou - user=%s amount=%.2f err=%v", userID, amount, err)
		if refErr := repository.IncrementWalletBalance(userID, amount); refErr != nil {
			log.Printf("CRITICAL: estorno tambem falhou - user=%s amount=%.2f err=%v", userID, amount, refErr)
		}
		return nil, fmt.Errorf("erro ao registrar solicitacao de saque: %w", err)
	}

	// PASSO 5: Dispara o Pix via gateway em background
	// O gateway e chamado de forma assincrona para nao travar a resposta HTTP.
	// O webhook do AbacatePay atualizara o status do saque.
	gateway := NewGatewayService()
	go ws.processPixWithGateway(gateway, payout, amount)

	return payout, nil
}

// processPixWithGateway envia o Pix para o gateway AbacatePay em background.
func (ws *WalletService) processPixWithGateway(gateway *GatewayService, payout *models.PayoutRequest, amount float64) {
	// Atualiza status para processing
	if err := repository.UpdatePayoutStatus(payout.ID, models.PayoutProcessing, "", ""); err != nil {
		log.Printf("Warning: falha ao atualizar status do saque %s: %v", payout.ID.Hex(), err)
	}

	resp, err := gateway.SendPixTransfer(&SendPixTransferRequest{
		Amount:  amount,
		PixKey:  payout.PixKey,
		PixType: payout.PixKeyType,
		Name:    payout.UserID, // Idealmente viria do cadastro do usuario
	})
	if err != nil {
		log.Printf("Warning: falha ao enviar Pix para saque %s: %v", payout.ID.Hex(), err)
		// Estorna o valor para a carteira
		if _, refErr := repository.IncrementWalletBalance(payout.UserID, amount); refErr != nil {
			log.Printf("CRITICAL: estorno apos falha no Pix falhou - payout=%s err=%v", payout.ID.Hex(), refErr)
		}
		// Atualiza status para failed
		if upErr := repository.UpdatePayoutStatus(payout.ID, models.PayoutFailed, "", err.Error()); upErr != nil {
			log.Printf("Warning: falha ao atualizar status de falha: %v", upErr)
		}
		return
	}

	// Sucesso — atualiza status e gateway_id
	if err := repository.UpdatePayoutStatus(payout.ID, models.PayoutCompleted, resp.ID, ""); err != nil {
		log.Printf("Warning: falha ao atualizar status de sucesso: %v", err)
	}
}
