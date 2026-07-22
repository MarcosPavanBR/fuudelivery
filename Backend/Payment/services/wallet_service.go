package services

import (
	"time"

	"github.com/carloshomar/vercardapio/payment/models"
	"github.com/carloshomar/vercardapio/payment/repository"
)

type WalletService struct{}

func NewWalletService() *WalletService {
	return &WalletService{}
}

func (ws *WalletService) GetOrCreateWallet(userID, userType string) (*models.Wallet, error) {
	wallet, err := repository.GetWallet(userID)
	if err == nil {
		return wallet, nil
	}

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

func (ws *WalletService) CreditWallet(userID string, amount float64, description string, referenceID string) error {
	wallet, err := repository.GetWallet(userID)
	if err != nil {
		return err
	}

	balanceBefore := wallet.Balance
	balanceAfter := balanceBefore + amount

	if err := repository.UpdateWalletBalance(userID, balanceAfter); err != nil {
		return err
	}

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

func (ws *WalletService) DebitWallet(userID string, amount float64, description string, referenceID string) error {
	wallet, err := repository.GetWallet(userID)
	if err != nil {
		return err
	}

	if wallet.Balance < amount {
		return nil
	}

	balanceBefore := wallet.Balance
	balanceAfter := balanceBefore - amount

	if err := repository.UpdateWalletBalance(userID, balanceAfter); err != nil {
		return err
	}

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

func (ws *WalletService) GetBalance(userID string) (float64, error) {
	wallet, err := repository.GetWallet(userID)
	if err != nil {
		return 0, err
	}
	return wallet.Balance, nil
}

func (ws *WalletService) GetTransactions(userID string, limit int) ([]models.WalletTransaction, error) {
	wallet, err := repository.GetWallet(userID)
	if err != nil {
		return nil, err
	}

	return repository.GetWalletTransactions(wallet.ID, limit)
}

func (ws *WalletService) ProcessPaymentApproval(payment *models.Payment) error {
	if payment.Status != models.PaymentApproved {
		return nil
	}

 establishmentID := payment.EstablishmentID
 amount := payment.Amount - payment.DeliveryAmount

	if err := ws.CreditWallet(establishmentID, amount, "Payment received for order "+payment.OrderID, payment.OrderID); err != nil {
		return err
	}

	now := time.Now()
	return repository.UpdatePaymentStatus(payment.ID, models.PaymentApproved, map[string]interface{}{
		"wallet_credited_at": now,
	})
}
