package handlers

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	authModels "github.com/carloshomar/fuudelivery/auth_api/app/models"
	"github.com/carloshomar/fuudelivery/payment_api/app/models"
	"github.com/carloshomar/fuudelivery/pkg/gateway"
	"github.com/carloshomar/fuudelivery/pkg/gateway/abacatepay"
	"gorm.io/gorm"
)

// ============================================================================
// PIX automático do destaque patrocinado.
//
// A cobrança NÃO entra na tabela payments: tudo ali é liquidado como pedido
// (split, crédito na carteira da loja) — a loja receberia de volta o próprio
// pagamento do destaque. Aqui a cobrança fica ligada à reserva
// (gateway_charge_id) e o webhook do AbacatePay, que já confere HMAC e
// reconsulta o status no gateway, liga o destaque (HandlePaymentWebhook →
// settleSponsorCharge). A varredura periódica cobre webhook perdido.
//
// Gateway: AbacatePay direto, não o router — o webhook deste projeto só sabe
// verificar cobrança do AbacatePay.
// ============================================================================

// Pontos de troca para teste (o gateway real exige credencial).
var (
	createSponsorPix = func(ctx context.Context, req *gateway.TransactionRequest) (*gateway.TransactionResponse, error) {
		gw, err := abacatepay.NewGateway()
		if err != nil {
			return nil, err
		}
		return gw.CreateTransaction(ctx, req)
	}
	// checkSponsorPix devolve (pago, valor em centavos).
	checkSponsorPix = func(chargeID string) (bool, int64, error) {
		gw, err := abacatepay.NewGateway()
		if err != nil {
			return false, 0, err
		}
		charge, err := gw.GetChargeDetails(chargeID)
		if err != nil {
			return false, 0, err
		}
		status, _ := charge["status"].(string)
		return isPaidStatus(status), chargeAmountCents(charge), nil
	}
)

func isPaidStatus(s string) bool {
	switch strings.ToUpper(s) {
	case "PAID", "CONFIRMED", "APPROVED":
		return true
	}
	return false
}

// chargeAmountCents lê o valor da cobrança (o AbacatePay devolve centavos).
func chargeAmountCents(charge map[string]interface{}) int64 {
	switch v := charge["amount"].(type) {
	case float64:
		return int64(math.Round(v))
	case int64:
		return v
	}
	return -1
}

// issueSponsorPix cria a cobrança PIX da reserva e guarda o copia-e-cola.
// Falha do gateway não desfaz a reserva: ela segue pendente e o admin
// confirma à mão (o caminho antigo).
func issueSponsorPix(ctx context.Context, b *authModels.SponsorBooking) error {
	resp, err := createSponsorPix(ctx, &gateway.TransactionRequest{
		Amount:        toCents(b.Total),
		Currency:      "BRL",
		PaymentMethod: gateway.MethodPIX,
		Description:   fmt.Sprintf("Destaque patrocinado %s a %s", b.StartDay, b.EndDay),
		Capture:       true,
		// Uma cobrança por reserva: retry não gera PIX duplicado.
		IdempotencyKey: fmt.Sprintf("sponsor-%d", b.ID),
		Metadata: map[string]string{
			"order_id":         fmt.Sprintf("sponsor-%d", b.ID),
			"sponsor_booking":  fmt.Sprintf("%d", b.ID),
			"establishment_id": fmt.Sprintf("%d", b.EstablishmentID),
		},
	})
	if err != nil {
		return err
	}
	if resp.GatewayID == "" || resp.PIXCopyPaste == "" {
		return errors.New("gateway sem id/copia-e-cola")
	}
	b.GatewayChargeID = resp.GatewayID
	b.PixCopyPaste = resp.PIXCopyPaste
	b.PixQRBase64 = resp.PIXQRCodeBase64
	return models.DB.Model(b).Updates(map[string]interface{}{
		"gateway_charge_id": b.GatewayChargeID,
		"pix_copy_paste":    b.PixCopyPaste,
		"pix_qr_base64":     b.PixQRBase64,
	}).Error
}

var errSponsorSlotsGone = errors.New("vagas ocupadas")

// refundSponsorToWallet cancela a reserva (se ainda não estiver) e devolve o
// valor à carteira da loja. Idempotente pelo índice único do crédito.
func refundSponsorToWallet(id uint, now time.Time) error {
	return models.DB.Transaction(func(tx *gorm.DB) error {
		var b authModels.SponsorBooking
		if err := tx.First(&b, id).Error; err != nil {
			return err
		}
		upd := map[string]interface{}{"paid_at": now}
		if b.Status != authModels.SponsorBookingCancelled {
			upd["status"] = authModels.SponsorBookingCancelled
			upd["cancelled_at"] = now
		}
		if b.PaidAt != nil {
			delete(upd, "paid_at")
		}
		if err := tx.Model(&b).Updates(upd).Error; err != nil {
			return err
		}
		desc := fmt.Sprintf("Devolução do destaque %s a %s (pago sem vaga)", b.StartDay, b.EndDay)
		_, err := models.AdjustWalletBalance(tx, int64(b.EstablishmentID), sponsorWalletType,
			"credit", "sponsor", b.Total, sponsorRefundRef(b.ID), desc, "")
		if errors.Is(err, models.ErrDuplicateCredit) {
			return nil
		}
		return err
	})
}

// activateSponsorBooking liga uma reserva pendente como paga. Idempotente.
// Reserva expirada só liga se as vagas ainda estiverem livres; senão devolve
// errSponsorSlotsGone (o chamador decide o que fazer com o dinheiro).
func activateSponsorBooking(tx *gorm.DB, b *authModels.SponsorBooking, now time.Time) error {
	if b.Status == authModels.SponsorBookingActive {
		return nil
	}
	if b.Status != authModels.SponsorBookingPending {
		return errors.New("reserva cancelada")
	}
	if err := lockSponsorZone(tx, b.ZoneID); err != nil {
		return err
	}
	if b.ExpiresAt != nil && !b.ExpiresAt.After(now) {
		days, _ := authModels.SponsorDayRange(b.StartDay, b.Days)
		if _, err := authModels.SponsorCheckAvailability(tx, b.EstablishmentID, b.ZoneID, days, now); err != nil {
			return errSponsorSlotsGone
		}
	}
	paid := now
	b.Status = authModels.SponsorBookingActive
	b.PaidAt = &paid
	b.ExpiresAt = nil
	return tx.Model(b).Updates(map[string]interface{}{
		"status": b.Status, "paid_at": paid, "expires_at": nil,
	}).Error
}

// settleSponsorCharge é chamado pelo webhook (e pela varredura) com uma
// cobrança JÁ verificada como paga no gateway. Devolve handled=false se a
// cobrança não é de destaque — aí segue o fluxo de pedidos.
func settleSponsorCharge(chargeID string, paidCents int64, now time.Time) (handled bool) {
	if chargeID == "" || models.DB == nil {
		return false
	}
	var b authModels.SponsorBooking
	if err := models.DB.Where("gateway_charge_id = ?", chargeID).First(&b).Error; err != nil {
		return false
	}
	if paidCents >= 0 && paidCents != toCents(b.Total) {
		log.Printf("[SPONSOR] cobrança %s paga com %d centavos, reserva %d custa %d — não liga",
			chargeID, paidCents, b.ID, toCents(b.Total))
		return true
	}
	err := models.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&b, b.ID).Error; err != nil {
			return err
		}
		return activateSponsorBooking(tx, &b, now)
	})
	if err == nil {
		log.Printf("[SPONSOR] reserva %d paga por PIX — destaque ativo", b.ID)
		return true
	}
	// Pagou mas não dá para ligar: a vaga expirou e foi vendida, ou a loja
	// cancelou e pagou mesmo assim. O dinheiro entrou: volta como crédito na
	// carteira da loja, na hora (serve para outra reserva ou saque), e a
	// reserva fica cancelada. Ref sponsor-refund:<id> é a mesma do
	// cancelamento: nunca devolve duas vezes.
	if rErr := refundSponsorToWallet(b.ID, now); rErr != nil {
		log.Printf("[SPONSOR] reserva %d PAGA e sem vaga (%v); crédito falhou: %v", b.ID, err, rErr)
	} else {
		log.Printf("[SPONSOR] reserva %d paga sem vaga (%v) — valor devolvido à carteira", b.ID, err)
	}
	return true
}

// ReconcileSponsorPixOnce confere no gateway as reservas pendentes com PIX
// emitido (webhook perdido). Janela: de 2 min a 48 h depois da reserva.
func ReconcileSponsorPixOnce(now time.Time) {
	if models.DB == nil {
		return
	}
	var pending []authModels.SponsorBooking
	// Cancelada também entra: PIX pago depois do cancelamento com webhook
	// perdido ainda precisa voltar à carteira.
	if err := models.DB.Where("status IN ? AND gateway_charge_id <> '' AND paid_at IS NULL AND created_at BETWEEN ? AND ?",
		[]string{authModels.SponsorBookingPending, authModels.SponsorBookingCancelled},
		now.Add(-48*time.Hour), now.Add(-reconcileGracePeriod)).
		Limit(200).Find(&pending).Error; err != nil {
		return
	}
	for _, b := range pending {
		paid, cents, err := checkSponsorPix(b.GatewayChargeID)
		if err != nil || !paid {
			continue
		}
		settleSponsorCharge(b.GatewayChargeID, cents, now)
	}
}
