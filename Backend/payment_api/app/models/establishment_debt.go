package models

import (
	"errors"
	"fmt"
	"math"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ============================================================================
// EstablishmentDebt — contas a receber do restaurante (modelo de repasse).
//
// Quando a loja recebe o pedido DIRETO (na conta dela), ela passa a DEVER à
// plataforma o frete (dinheiro do entregador) + a comissão. Cada linha é uma
// dívida por pedido. Mapeia a tabela establishment_debts (sql/27).
//
// O risco aqui é de crédito: a loja pode não repassar. Por isso OpenDebtTotal
// existe (a trava de crédito soma o que a loja deve em aberto) e a criação é
// idempotente por order_id (settle reprocessado não duplica o débito).
// ============================================================================

// Estados da dívida (espelham o CHECK de sql/27).
const (
	DebtOpen   = "open"
	DebtPaid   = "paid"
	DebtWaived = "waived"
)

// EstablishmentDebt — linha da tabela establishment_debts.
type EstablishmentDebt struct {
	ID               int64      `gorm:"primaryKey;column:id" json:"id"`
	EstablishmentID  int64      `gorm:"column:establishment_id" json:"establishment_id"`
	OrderID          string     `gorm:"column:order_id;uniqueIndex:uq_establishment_debts_order" json:"order_id"`
	PaymentID        int64      `gorm:"column:payment_id" json:"payment_id,omitempty"`
	DeliveryAmount   float64    `gorm:"column:delivery_amount" json:"delivery_amount"`
	CommissionAmount float64    `gorm:"column:commission_amount" json:"commission_amount"`
	TotalAmount      float64    `gorm:"column:total_amount" json:"total_amount"`
	Status           string     `gorm:"column:status" json:"status"`
	CreatedAt        time.Time  `gorm:"column:created_at" json:"created_at"`
	SettledAt        *time.Time `gorm:"column:settled_at" json:"settled_at,omitempty"`
	SettledBy        string     `gorm:"column:settled_by" json:"settled_by,omitempty"`
}

// TableName fixa o nome da tabela.
func (EstablishmentDebt) TableName() string { return "establishment_debts" }

// CreateDebt registra o que a loja deve por um pedido. Idempotente por order_id:
// o UNIQUE uq_establishment_debts_order (sql/27) faz o segundo registro do mesmo
// pedido não criar linha nova — reprocessar o settle não duplica a dívida.
// Devolve (created=false) quando a dívida já existia.
func CreateDebt(db *gorm.DB, d *EstablishmentDebt) (created bool, err error) {
	if d.Status == "" {
		d.Status = DebtOpen
	}
	if d.CreatedAt.IsZero() {
		d.CreatedAt = time.Now()
	}
	// total = frete + comissão, sempre — não confia num total passado solto.
	d.TotalAmount = roundMoney(d.DeliveryAmount + d.CommissionAmount)

	res := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "order_id"}},
		DoNothing: true,
	}).Create(d)
	if res.Error != nil {
		return false, fmt.Errorf("criar dívida do pedido %s: %w", d.OrderID, res.Error)
	}
	return res.RowsAffected == 1, nil
}

// OpenDebtTotalCents soma, em centavos, o que a loja deve em aberto. É a
// consulta que a trava de crédito usa. Centavos (int64) para o gate comparar
// sem erro de float.
func OpenDebtTotalCents(db *gorm.DB, establishmentID int64) (int64, error) {
	var totalReais float64
	err := db.Model(&EstablishmentDebt{}).
		Where("establishment_id = ? AND status = ?", establishmentID, DebtOpen).
		Select("COALESCE(SUM(total_amount), 0)").
		Scan(&totalReais).Error
	if err != nil {
		return 0, fmt.Errorf("somar dívida em aberto da loja %d: %w", establishmentID, err)
	}
	return int64(math.Round(totalReais * 100)), nil
}

var ErrDebtNotFound = errors.New("dívida não encontrada")

// MarkDebtPaid quita uma dívida em aberto (repasse recebido). settledBy é quem
// registrou (admin ou o processo de cobrança). Só mexe em dívida 'open'.
func MarkDebtPaid(db *gorm.DB, debtID int64, settledBy string) error {
	now := time.Now()
	res := db.Model(&EstablishmentDebt{}).
		Where("id = ? AND status = ?", debtID, DebtOpen).
		Updates(map[string]interface{}{
			"status":     DebtPaid,
			"settled_at": now,
			"settled_by": settledBy,
		})
	if res.Error != nil {
		return fmt.Errorf("quitar dívida %d: %w", debtID, res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrDebtNotFound
	}
	return nil
}

// roundMoney arredonda reais para 2 casas (mesma intenção do roundCents do
// split_calculator, replicada aqui para o pacote models não depender de
// services).
func roundMoney(v float64) float64 {
	return float64(int64(v*100+0.5)) / 100
}
