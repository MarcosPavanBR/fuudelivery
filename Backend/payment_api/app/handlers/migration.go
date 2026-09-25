package handlers

import (
	"log"
	"time"

	"github.com/carloshomar/fuudelivery/payment_api/app/models"
	"gorm.io/gorm"
)

// findPaymentByAbacatePayID localiza o pagamento pelo ID externo do gateway.
func findPaymentByAbacatePayID(abacatepayID string) (*models.Payment, error) {
	var payment models.Payment
	err := models.DB.Where("abacatepay_id = ?", abacatepayID).First(&payment).Error
	if err != nil {
		return nil, err
	}
	return &payment, nil
}

// ensureWalletSeeded garante que a carteira exista no Postgres ANTES da
// primeira movimentação. Se já existe, retorna a carteira existente.
func ensureWalletSeeded(db *gorm.DB, userID int64, userType string) (*models.Wallet, error) {
	var wallet models.Wallet
	err := db.Where("user_id = ? AND user_type = ?", userID, userType).First(&wallet).Error
	if err == nil {
		return &wallet, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, err
	}

	wallet = models.Wallet{
		UserID:      userID,
		UserType:    userType,
		Balance:     0,
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

	log.Printf("[WALLET] Carteira %d/%s criada com saldo zero", userID, userType)
	return &wallet, nil
}
