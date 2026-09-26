package handlers

// referral.go — indicação de amigos.
//
// Como funciona:
//   - Cada cliente tem um código pessoal ("AMIGO7K3QX", GET /referral/me)
//     para compartilhar.
//   - O amigo digita o código no campo de cupom do carrinho. Se ele nunca
//     pediu antes, o servidor cria um cupom de boas-vindas SÓ DELE
//     (R$ 10 em pedido a partir de R$ 20, bancado pela plataforma) e o
//     pedido usa esse cupom pela maquinaria normal (consumo, devolução, split).
//   - Quem indicou ganha 100 pontos (= R$ 10 no resgate) quando esse primeiro
//     pedido é ENTREGUE — não antes. O modelo antigo (/coupons/referral)
//     dava o prêmio na hora a quem digitasse qualquer telefone.
//
// Travas: o código do próprio cliente não vale para ele; cada telefone só é
// indicado uma vez (índice único); só vale para quem não tem pedido anterior;
// o prêmio é pago uma vez (UPDATE condicional pending→rewarded).

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/big"
	"strings"
	"time"

	"github.com/carloshomar/fuudelivery/auth_api/app/middlewares"
	"github.com/carloshomar/fuudelivery/orders_api/app/dto"
	"github.com/carloshomar/fuudelivery/orders_api/app/models"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

const (
	referralWelcomeValue = 10.0 // R$ de desconto para o amigo
	referralMinOrder     = 20.0 // pedido mínimo (produtos) para o desconto
	referralRewardPoints = 100  // pontos para quem indicou (100 = R$ 10)
	referralCouponDays   = 30
)

const referralAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789" // sem 0/O, 1/I/L

func randomCode(prefix string, n int) (string, error) {
	var b strings.Builder
	b.WriteString(prefix)
	for i := 0; i < n; i++ {
		k, err := rand.Int(rand.Reader, big.NewInt(int64(len(referralAlphabet))))
		if err != nil {
			return "", err
		}
		b.WriteByte(referralAlphabet[k.Int64()])
	}
	return b.String(), nil
}

// referralCodeFor devolve (criando na primeira vez) o código do cliente.
func referralCodeFor(phone string) (models.ReferralCode, error) {
	var rc models.ReferralCode
	if err := models.DB.Where("phone = ?", phone).First(&rc).Error; err == nil {
		return rc, nil
	}
	for i := 0; i < 5; i++ {
		code, err := randomCode("AMIGO", 5)
		if err != nil {
			return rc, err
		}
		rc = models.ReferralCode{Phone: phone, Code: code}
		if err := models.DB.Create(&rc).Error; err == nil {
			return rc, nil
		}
		// Colisão de código, ou outro request criou o do mesmo telefone.
		if models.DB.Where("phone = ?", phone).First(&rc).Error == nil {
			return rc, nil
		}
	}
	return rc, errors.New("não foi possível gerar o código")
}

// GetMyReferral — GET /referral/me: código do cliente e o placar.
func GetMyReferral(c *fiber.Ctx) error {
	phone, err := middlewares.GetUserPhoneFromToken(c)
	if err != nil || phone == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Entre na sua conta"})
	}
	rc, err := referralCodeFor(phone)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Falha ao gerar o código"})
	}
	var invited, rewarded int64
	models.DB.Model(&models.Referral{}).Where("referrer_phone = ?", phone).Count(&invited)
	models.DB.Model(&models.Referral{}).Where("referrer_phone = ? AND status = ?", phone, models.ReferralRewarded).Count(&rewarded)
	return c.JSON(fiber.Map{
		"code":          rc.Code,
		"welcome_value": referralWelcomeValue,
		"min_order":     referralMinOrder,
		"reward_points": referralRewardPoints,
		"invited":       invited,
		"rewarded":      rewarded,
		"share_text": fmt.Sprintf(
			"Use meu código %s no FuuDelivery e ganhe R$ %.0f no seu primeiro pedido (a partir de R$ %.0f)!",
			rc.Code, referralWelcomeValue, referralMinOrder),
	})
}

// resolveReferralCode: se code é um código de indicação, devolve o cupom de
// boas-vindas do amigo (criando na primeira vez). matched=false quando o
// código não é de indicação — aí segue como cupom normal.
func resolveReferralCode(code, tokenPhone string) (couponCode string, matched bool, err error) {
	var rc models.ReferralCode
	if models.DB.Where("code = ?", code).First(&rc).Error != nil {
		return "", false, nil
	}
	if tokenPhone == "" {
		return "", true, errors.New("entre na sua conta para usar o código de indicação")
	}
	if samePhone(rc.Phone, tokenPhone) {
		return "", true, errors.New("o seu próprio código é para compartilhar com amigos")
	}

	// Já foi indicado: reaproveita o cupom se ainda não foi usado (pedido que
	// falhou/cancelou devolve o uso).
	var existing models.Referral
	if models.DB.Where("referee_phone = ?", tokenPhone).First(&existing).Error == nil {
		var cp models.Coupon
		if models.DB.Where("code = ?", existing.CouponCode).First(&cp).Error == nil && cp.UsedCount == 0 &&
			existing.Status == models.ReferralPending {
			return cp.Code, true, nil
		}
		return "", true, errors.New("o código de indicação vale só no primeiro pedido")
	}

	// Só para quem nunca pediu (pedido recusado/cancelado não conta).
	var previous int64
	models.DB.Model(&models.OrderDocument{}).
		Where("user_phone = ? AND status NOT IN ?", tokenPhone, []string{"DENIED", "CANCELLED"}).
		Count(&previous)
	if previous > 0 {
		return "", true, errors.New("o código de indicação vale só no primeiro pedido")
	}

	welcome, err := randomCode("BEMVINDO", 6)
	if err != nil {
		return "", true, err
	}
	now := time.Now()
	txErr := models.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&models.Coupon{
			Code:           welcome,
			Description:    "Boas-vindas por indicação",
			DiscountType:   "FIXED",
			DiscountValue:  referralWelcomeValue,
			MinOrderValue:  referralMinOrder,
			MaxUses:        1,
			MaxUsesPerUser: 1,
			StartDate:      now,
			ExpiryDate:     now.AddDate(0, 0, referralCouponDays),
			IsActive:       true,
			OwnerPhone:     tokenPhone,
			FundedBy:       models.CouponFundedByPlatform,
		}).Error; err != nil {
			return err
		}
		return tx.Create(&models.Referral{
			ReferrerPhone: rc.Phone,
			RefereePhone:  tokenPhone,
			CouponCode:    welcome,
			Status:        models.ReferralPending,
		}).Error
	})
	if txErr != nil {
		// Corrida: outro pedido simultâneo do mesmo amigo criou a indicação.
		if models.DB.Where("referee_phone = ?", tokenPhone).First(&existing).Error == nil {
			return existing.CouponCode, true, nil
		}
		return "", true, errors.New("não foi possível aplicar o código de indicação")
	}
	return welcome, true, nil
}

func orderCouponCode(doc *models.OrderDocument) string {
	var p dto.RequestPayload
	if json.Unmarshal(doc.Payload, &p) != nil {
		return ""
	}
	return strings.ToUpper(strings.TrimSpace(p.CouponCode))
}

// handleReferralOnStatus roda a cada mudança de status do pedido:
//   - FINISHED: 1º pedido do amigo entregue → pontos para quem indicou (uma vez);
//   - DENIED/CANCELLED: devolve o cupom de boas-vindas (o amigo pode tentar de novo).
func handleReferralOnStatus(doc *models.OrderDocument) {
	if doc == nil || models.DB == nil {
		return
	}
	switch doc.Status {
	case "FINISHED", "DENIED", "CANCELLED":
	default:
		return
	}
	code := orderCouponCode(doc)
	if code == "" {
		return
	}
	var ref models.Referral
	if models.DB.Where("coupon_code = ? AND status = ?", code, models.ReferralPending).First(&ref).Error != nil {
		return
	}
	if doc.Status != "FINISHED" {
		releaseCoupon(code, doc.LegacyID)
		return
	}
	if !samePhone(ref.RefereePhone, doc.UserPhone) {
		return
	}
	err := models.DB.Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		upd := tx.Model(&models.Referral{}).
			Where("id = ? AND status = ?", ref.ID, models.ReferralPending).
			Updates(map[string]interface{}{"status": models.ReferralRewarded, "order_id": doc.LegacyID, "rewarded_at": now})
		if upd.Error != nil || upd.RowsAffected == 0 {
			return upd.Error // já premiado: no-op
		}
		acc, err := lockLoyaltyAccount(tx, ref.ReferrerPhone, true)
		if err != nil {
			return err
		}
		acc.Points += referralRewardPoints
		acc.Tier = getTier(acc.Points)
		acc.UpdatedAt = now
		if err := tx.Save(&acc).Error; err != nil {
			return err
		}
		return tx.Create(&models.LoyaltyTransaction{
			UserPhone:   ref.ReferrerPhone,
			Points:      referralRewardPoints,
			Type:        "earn",
			Description: "Indicação: seu amigo fez o primeiro pedido",
			OrderID:     "referral:" + doc.LegacyID,
			CreatedAt:   now,
		}).Error
	})
	if err != nil {
		log.Printf("[REFERRAL] prêmio da indicação %d (pedido %s) falhou: %v", ref.ID, doc.LegacyID, err)
	}
}
