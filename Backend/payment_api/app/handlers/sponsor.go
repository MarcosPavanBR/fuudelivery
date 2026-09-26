package handlers

import (
	"errors"
	"fmt"
	"log"
	"math"
	"strconv"
	"time"

	"github.com/carloshomar/fuudelivery/auth_api/app/middlewares"
	authModels "github.com/carloshomar/fuudelivery/auth_api/app/models"
	"github.com/carloshomar/fuudelivery/payment_api/app/models"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// ============================================================================
// Destaque patrocinado por dia — reserva e cobrança. As regras de vaga e
// rodízio estão em auth_api/app/models/sponsor_booking.go.
//
// Dinheiro: o débito na carteira da loja acontece na MESMA transação que cria
// a reserva. Sem saldo, nada é criado (ou a loja escolhe PIX e a reserva fica
// pendente por 24 h até o admin confirmar). A referência do ledger é
// "sponsor:<id>", única por carteira (sql/20): o mesmo destaque não é
// cobrado duas vezes, e o estorno usa "sponsor-refund:<id>".
// ============================================================================

const sponsorWalletType = "establishment"

func sponsorDebitRef(id uint) string  { return fmt.Sprintf("sponsor:%d", id) }
func sponsorRefundRef(id uint) string { return fmt.Sprintf("sponsor-refund:%d", id) }

func roundCents(v float64) float64 { return math.Round(v*100) / 100 }

// lockSponsorZone serializa reservas da mesma praça: sem a trava, duas lojas
// pegando a última vaga ao mesmo tempo passariam as duas na contagem.
func lockSponsorZone(tx *gorm.DB, zoneID uint) error {
	if tx.Dialector.Name() != "postgres" {
		return nil // sqlite dos testes unitários já serializa escritas
	}
	return tx.Exec("SELECT pg_advisory_xact_lock(?)", int64(77_000_000)+int64(zoneID)).Error
}

func establishmentZone(tx *gorm.DB, estID int64) (uint, error) {
	var est authModels.Establishment
	if err := tx.Select("id", "zone_id").First(&est, estID).Error; err != nil {
		return 0, err
	}
	if est.ZoneID != nil {
		return *est.ZoneID, nil
	}
	return 0, nil
}

func sponsorStoreID(c *fiber.Ctx) (int64, bool) {
	estID, err := middlewares.GetEstablishmentIDFromToken(c)
	if err != nil || estID <= 0 {
		return 0, false
	}
	return estID, true
}

func isAdmin(c *fiber.Ctx) bool {
	role, err := middlewares.GetUserRoleFromToken(c)
	return err == nil && role == "admin"
}

// GetSponsorOffer — o que a loja vê antes de reservar: preço, vagas por dia,
// ocupação dos próximos dias, saldo da carteira e as reservas dela.
// GET /sponsored/offer?days=3
func GetSponsorOffer(c *fiber.Ctx) error {
	estID, ok := sponsorStoreID(c)
	if !ok {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Só a loja pode ver o destaque"})
	}
	days, _ := strconv.Atoi(c.Query("days", "1"))
	if days < 1 || days > authModels.SponsorMaxDays {
		days = 1
	}
	now := time.Now()
	zoneID, err := establishmentZone(models.DB, estID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Loja não encontrada"})
	}
	today := authModels.SponsorToday(now)
	calendarDays, _ := authModels.SponsorDayRange(today, 30)
	occ, err := authModels.SponsorOccupancy(models.DB, zoneID, calendarDays, now)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Falha ao consultar vagas"})
	}
	slots := authModels.SponsorSlotsPerDay()
	calendar := make([]fiber.Map, 0, len(calendarDays))
	for _, d := range calendarDays {
		calendar = append(calendar, fiber.Map{"day": d, "taken": occ[d], "free": max(0, slots-occ[d])})
	}
	next, _ := authModels.NextAvailableStart(models.DB, zoneID, today, days, now)

	var mine []authModels.SponsorBooking
	models.DB.Where("establishment_id = ? AND end_day >= ? AND status <> ?", estID, today, authModels.SponsorBookingCancelled).
		Order("start_day").Find(&mine)

	balance := 0.0
	if w, wErr := ensureWalletSeeded(models.DB, estID, sponsorWalletType); wErr == nil {
		balance = w.Balance
	}

	return c.JSON(fiber.Map{
		"price_per_day":  authModels.SponsorPricePerDay(),
		"slots_per_day":  slots,
		"max_days":       authModels.SponsorMaxDays,
		"max_ahead_days": authModels.SponsorMaxAheadDays,
		"today":          today,
		"calendar":       calendar,
		"next_available": next,
		"wallet_balance": balance,
		"bookings":       mine,
	})
}

// CreateSponsorBooking — a loja reserva N dias a partir de start_day.
// POST /sponsored/bookings {"start_day":"2026-09-26","days":3,"pay_with":"wallet"|"pix"}
func CreateSponsorBooking(c *fiber.Ctx) error {
	estID, ok := sponsorStoreID(c)
	if !ok {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Só a loja pode reservar destaque"})
	}
	var req struct {
		StartDay string `json:"start_day"`
		Days     int    `json:"days"`
		PayWith  string `json:"pay_with"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Corpo inválido"})
	}
	if req.PayWith != authModels.SponsorPayWallet && req.PayWith != authModels.SponsorPayPix {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "pay_with deve ser wallet ou pix"})
	}
	now := time.Now()
	today := authModels.SponsorToday(now)
	lastStart, _ := authModels.SponsorDayRange(today, authModels.SponsorMaxAheadDays+1)
	if req.Days < 1 || req.Days > authModels.SponsorMaxDays {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": fmt.Sprintf("Escolha de 1 a %d dias", authModels.SponsorMaxDays)})
	}
	days, err := authModels.SponsorDayRange(req.StartDay, req.Days)
	if err != nil || req.StartDay < today || req.StartDay > lastStart[len(lastStart)-1] {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("Início deve ser entre hoje e daqui a %d dias", authModels.SponsorMaxAheadDays),
		})
	}

	price := authModels.SponsorPricePerDay()
	total := roundCents(price * float64(req.Days))
	var booking authModels.SponsorBooking
	var fullDay string

	txErr := models.DB.Transaction(func(tx *gorm.DB) error {
		zoneID, zErr := establishmentZone(tx, estID)
		if zErr != nil {
			return zErr
		}
		if err := lockSponsorZone(tx, zoneID); err != nil {
			return err
		}
		fd, aErr := authModels.SponsorCheckAvailability(tx, uint(estID), zoneID, days, now)
		if aErr != nil {
			fullDay = fd
			return aErr
		}
		booking = authModels.SponsorBooking{
			EstablishmentID: uint(estID),
			ZoneID:          zoneID,
			StartDay:        days[0],
			EndDay:          days[len(days)-1],
			Days:            req.Days,
			PricePerDay:     price,
			Total:           total,
			PayWith:         req.PayWith,
			Status:          authModels.SponsorBookingPending,
		}
		if req.PayWith == authModels.SponsorPayPix {
			exp := now.Add(authModels.SponsorPendingHold)
			booking.ExpiresAt = &exp
			return tx.Create(&booking).Error
		}

		// Carteira: cria a reserva e debita na mesma transação.
		paid := now
		booking.Status = authModels.SponsorBookingActive
		booking.PaidAt = &paid
		if err := tx.Create(&booking).Error; err != nil {
			return err
		}
		if _, err := ensureWalletSeeded(tx, estID, sponsorWalletType); err != nil {
			return err
		}
		desc := fmt.Sprintf("Destaque patrocinado: %d dia(s), %s a %s", req.Days, booking.StartDay, booking.EndDay)
		_, err := models.AdjustWalletBalance(tx, estID, sponsorWalletType, "debit", "sponsor", total,
			sponsorDebitRef(booking.ID), desc, "")
		return err
	})

	switch {
	case txErr == nil:
	case errors.Is(txErr, authModels.ErrSponsorOverlap):
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "Sua loja já tem destaque reservado nesses dias"})
	case errors.Is(txErr, authModels.ErrSponsorDayFull):
		zoneID, _ := establishmentZone(models.DB, estID)
		next, _ := authModels.NextAvailableStart(models.DB, zoneID, today, req.Days, now)
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error":          fmt.Sprintf("O dia %s já está com todas as vagas de destaque ocupadas", fullDay),
			"full_day":       fullDay,
			"next_available": next,
		})
	case errors.Is(txErr, models.ErrInsufficientBalance):
		return c.Status(fiber.StatusPaymentRequired).JSON(fiber.Map{
			"error": fmt.Sprintf("Saldo insuficiente na carteira para R$ %.2f. Escolha pagar por PIX.", total),
			"total": total,
		})
	default:
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Falha ao reservar destaque"})
	}

	resp := fiber.Map{"booking": booking}
	if booking.Status == authModels.SponsorBookingPending {
		if err := issueSponsorPix(c.Context(), &booking); err != nil {
			log.Printf("[SPONSOR] reserva %d: PIX automático indisponível (%v) — confirmação manual", booking.ID, err)
			resp["message"] = "Reserva guardada por 24 h. O PIX automático está fora do ar: pague por PIX e envie o comprovante ao suporte; o destaque liga quando o pagamento for confirmado."
		} else {
			resp["message"] = "Reserva guardada por 24 h. Pague o PIX abaixo: o destaque liga sozinho quando o pagamento cair."
		}
		resp["booking"] = booking
	} else {
		resp["message"] = "Destaque reservado e pago com o saldo da carteira."
	}
	return c.Status(fiber.StatusCreated).JSON(resp)
}

// CancelSponsorBooking — cancela uma reserva que ainda não começou. Se foi
// paga pela carteira, o valor volta para ela (uma vez só: ref única).
// POST /sponsored/bookings/:id/cancel (dono da loja ou admin)
func CancelSponsorBooking(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil || id == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "ID inválido"})
	}
	admin := isAdmin(c)
	estID, isStore := sponsorStoreID(c)
	now := time.Now()
	today := authModels.SponsorToday(now)

	var booking authModels.SponsorBooking
	txErr := models.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&booking, id).Error; err != nil {
			return fiber.ErrNotFound
		}
		if !admin && (!isStore || int64(booking.EstablishmentID) != estID) {
			return fiber.ErrForbidden
		}
		if booking.Status == authModels.SponsorBookingCancelled {
			return nil // idempotente
		}
		// A loja só cancela antes de começar; o admin pode a qualquer hora
		// (ex.: loja suspensa), e o estorno é decisão dele fora daqui.
		if !admin && booking.StartDay <= today {
			return fiber.NewError(fiber.StatusConflict, "O destaque já começou e não pode ser cancelado")
		}
		wasPaidByWallet := booking.Status == authModels.SponsorBookingActive && booking.PayWith == authModels.SponsorPayWallet
		cancelled := now
		if err := tx.Model(&booking).Updates(map[string]interface{}{
			"status": authModels.SponsorBookingCancelled, "cancelled_at": cancelled,
		}).Error; err != nil {
			return err
		}
		booking.Status = authModels.SponsorBookingCancelled
		booking.CancelledAt = &cancelled
		if wasPaidByWallet && booking.StartDay > today {
			desc := fmt.Sprintf("Estorno do destaque %s a %s", booking.StartDay, booking.EndDay)
			if _, err := models.AdjustWalletBalance(tx, int64(booking.EstablishmentID), sponsorWalletType,
				"credit", "sponsor", booking.Total, sponsorRefundRef(booking.ID), desc, ""); err != nil &&
				!errors.Is(err, models.ErrDuplicateCredit) {
				return err
			}
		}
		return nil
	})
	var fe *fiber.Error
	if errors.As(txErr, &fe) {
		return c.Status(fe.Code).JSON(fiber.Map{"error": fe.Message})
	}
	if txErr != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Falha ao cancelar"})
	}
	return c.JSON(fiber.Map{"booking": booking})
}

// ListSponsorBookings — admin: reservas (filtro ?status=pending_payment).
// GET /sponsored/bookings
func ListSponsorBookings(c *fiber.Ctx) error {
	q := models.DB.Model(&authModels.SponsorBooking{}).Order("created_at DESC").Limit(300)
	if s := c.Query("status"); s != "" {
		q = q.Where("status = ?", s)
	}
	var out []authModels.SponsorBooking
	if err := q.Find(&out).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Falha ao listar"})
	}
	names := map[uint]string{}
	var ests []authModels.Establishment
	ids := make([]uint, 0, len(out))
	for _, b := range out {
		ids = append(ids, b.EstablishmentID)
	}
	if len(ids) > 0 {
		models.DB.Select("id", "name").Where("id IN ?", ids).Find(&ests)
	}
	for _, e := range ests {
		names[e.ID] = e.Name
	}
	rows := make([]fiber.Map, 0, len(out))
	for _, b := range out {
		b.PixQRBase64 = "" // imagem pesada; o admin não precisa
		rows = append(rows, fiber.Map{"booking": b, "establishment_name": names[b.EstablishmentID]})
	}
	return c.JSON(rows)
}

// ConfirmSponsorPayment — admin confirma o PIX de uma reserva pendente.
// Se a reserva já expirou, só confirma se as vagas ainda estiverem livres.
// POST /sponsored/bookings/:id/confirm
func ConfirmSponsorPayment(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil || id == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "ID inválido"})
	}
	now := time.Now()
	var booking authModels.SponsorBooking
	txErr := models.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&booking, id).Error; err != nil {
			return fiber.ErrNotFound
		}
		if booking.Status == authModels.SponsorBookingCancelled {
			return fiber.NewError(fiber.StatusConflict, "Reserva cancelada não pode ser confirmada")
		}
		if err := activateSponsorBooking(tx, &booking, now); err != nil {
			if errors.Is(err, errSponsorSlotsGone) {
				return fiber.NewError(fiber.StatusConflict, "A reserva expirou e as vagas desses dias já foram ocupadas")
			}
			return err
		}
		return nil
	})
	var fe *fiber.Error
	if errors.As(txErr, &fe) {
		return c.Status(fe.Code).JSON(fiber.Map{"error": fe.Message})
	}
	if txErr != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Falha ao confirmar"})
	}
	return c.JSON(fiber.Map{"booking": booking})
}
