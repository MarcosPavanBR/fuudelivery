package models

import (
	"errors"
	"os"
	"sort"
	"strconv"
	"time"

	"gorm.io/gorm"
)

// ============================================================================
// Destaque patrocinado por DIA.
//
// A loja reserva dias em que aparece no topo da lista do app do cliente, com o
// selo "Patrocinado". Regras (o "e se várias lojas quiserem?"):
//
//   - Vagas limitadas por dia e por praça (SPONSOR_SLOTS_PER_DAY, padrão 3):
//     o topo não vira só anúncio e cada vaga continua valendo.
//   - Quem reserva primeiro garante o dia. Dia lotado não é vendido; a loja
//     vê a próxima data com vaga (NextAvailableStart).
//   - Entre as lojas do dia, rodízio justo: a ordem gira a cada hora, então
//     cada uma passa o mesmo tempo em 1º lugar (RotateSponsors).
//   - Preço fixo por dia (SPONSOR_DAILY_PRICE, padrão R$ 15,00).
//   - Loja fechada ou fora do raio não ganha nada com o topo: a vitrine só
//     destaca quem está aberto (o app do cliente ordena).
//
// Pagamento (payment_api): débito na carteira da loja, na hora, ou PIX com a
// reserva "aguardando pagamento" por 24 h — o webhook do gateway liga o
// destaque quando o PIX cai (o admin confirma à mão só se o gateway falhar).
// A reserva pendente segura a vaga só enquanto não expira.
//
// Substitui o patrocínio mensal (SponsoredListing), que só o admin criava e
// ninguém pagava.
// ============================================================================

const (
	SponsorBookingPending   = "pending_payment"
	SponsorBookingActive    = "active"
	SponsorBookingCancelled = "cancelled"

	SponsorPayWallet = "wallet"
	SponsorPayPix    = "pix"

	// Janela para pagar uma reserva por PIX antes de a vaga voltar ao mercado.
	SponsorPendingHold = 24 * time.Hour
	// Limites de uma reserva.
	SponsorMaxDays      = 30
	SponsorMaxAheadDays = 60

	sponsorDayLayout = "2006-01-02"
)

var (
	ErrSponsorDayFull     = errors.New("dia sem vaga de destaque")
	ErrSponsorOverlap     = errors.New("a loja já tem destaque reservado nesses dias")
	ErrSponsorInvalidDays = errors.New("período inválido")
)

// SponsorBooking — uma reserva de N dias seguidos de destaque.
// Dias guardados como "AAAA-MM-DD" no fuso da loja (StoreLocation): o dia
// comercial é o de Brasília, e a comparação de texto já ordena as datas.
type SponsorBooking struct {
	ID              uint       `gorm:"primaryKey;autoIncrement" json:"id"`
	EstablishmentID uint       `gorm:"not null;index" json:"establishment_id"`
	ZoneID          uint       `gorm:"not null;default:0;index" json:"zone_id"` // 0 = sem praça
	StartDay        string     `gorm:"size:10;not null;index" json:"start_day"`
	EndDay          string     `gorm:"size:10;not null;index" json:"end_day"` // inclusivo
	Days            int        `gorm:"not null" json:"days"`
	PricePerDay     float64    `gorm:"not null" json:"price_per_day"`
	Total           float64    `gorm:"not null" json:"total"`
	Status          string     `gorm:"size:20;not null;index" json:"status"`
	PayWith         string     `gorm:"size:10;not null" json:"pay_with"`
	PaidAt          *time.Time `json:"paid_at,omitempty"`
	// PIX automático (payment_api/app/handlers/sponsor_pix.go): a cobrança
	// fica ligada à reserva e o webhook liga o destaque quando cai. O id da
	// cobrança não sai no JSON.
	GatewayChargeID string     `gorm:"size:100;index" json:"-"`
	PixCopyPaste    string     `gorm:"type:text" json:"pix_copy_paste,omitempty"`
	PixQRBase64     string     `gorm:"type:text" json:"pix_qr_base64,omitempty"`
	ExpiresAt       *time.Time `json:"expires_at,omitempty"` // só pendente
	CancelledAt     *time.Time `json:"cancelled_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

func (SponsorBooking) TableName() string { return "sponsor_bookings" }

// SponsorPricePerDay e SponsorSlotsPerDay vêm do ambiente para o admin
// ajustar sem deploy de código.
func SponsorPricePerDay() float64 {
	if v, err := strconv.ParseFloat(os.Getenv("SPONSOR_DAILY_PRICE"), 64); err == nil && v > 0 {
		return v
	}
	return 15.00
}

func SponsorSlotsPerDay() int {
	if v, err := strconv.Atoi(os.Getenv("SPONSOR_SLOTS_PER_DAY")); err == nil && v > 0 {
		return v
	}
	return 3
}

// SponsorToday é a data comercial (fuso da loja) do instante t.
func SponsorToday(t time.Time) string {
	return t.In(StoreLocation()).Format(sponsorDayLayout)
}

// SponsorDayRange devolve os N dias a partir de start ("AAAA-MM-DD").
func SponsorDayRange(start string, days int) ([]string, error) {
	d, err := time.Parse(sponsorDayLayout, start)
	if err != nil || days < 1 {
		return nil, ErrSponsorInvalidDays
	}
	out := make([]string, days)
	for i := range out {
		out[i] = d.AddDate(0, 0, i).Format(sponsorDayLayout)
	}
	return out, nil
}

// holdsSlot: reserva que ocupa vaga no instante now — ativa, ou pendente
// dentro das 24 h para pagar.
func holdsSlot(db *gorm.DB, now time.Time) *gorm.DB {
	return db.Where("status = ? OR (status = ? AND expires_at > ?)",
		SponsorBookingActive, SponsorBookingPending, now)
}

// SponsorOccupancy conta as vagas ocupadas em cada dia do período, na praça.
func SponsorOccupancy(db *gorm.DB, zoneID uint, days []string, now time.Time) (map[string]int, error) {
	occ := make(map[string]int, len(days))
	if len(days) == 0 {
		return occ, nil
	}
	var bookings []SponsorBooking
	q := db.Model(&SponsorBooking{}).
		Where("zone_id = ? AND start_day <= ? AND end_day >= ?", zoneID, days[len(days)-1], days[0])
	if err := holdsSlot(q, now).Find(&bookings).Error; err != nil {
		return nil, err
	}
	for _, day := range days {
		for _, b := range bookings {
			if b.StartDay <= day && b.EndDay >= day {
				occ[day]++
			}
		}
	}
	return occ, nil
}

// SponsorCheckAvailability confere vaga em todos os dias e se a loja já não
// tem destaque em algum deles. Devolve o primeiro dia lotado, se houver.
func SponsorCheckAvailability(db *gorm.DB, establishmentID, zoneID uint, days []string, now time.Time) (fullDay string, err error) {
	var own int64
	q := db.Model(&SponsorBooking{}).
		Where("establishment_id = ? AND start_day <= ? AND end_day >= ?", establishmentID, days[len(days)-1], days[0])
	if err := holdsSlot(q, now).Count(&own).Error; err != nil {
		return "", err
	}
	if own > 0 {
		return "", ErrSponsorOverlap
	}
	occ, err := SponsorOccupancy(db, zoneID, days, now)
	if err != nil {
		return "", err
	}
	slots := SponsorSlotsPerDay()
	for _, d := range days {
		if occ[d] >= slots {
			return d, ErrSponsorDayFull
		}
	}
	return "", nil
}

// NextAvailableStart procura, a partir de from, o primeiro início em que
// cabem `days` dias seguidos com vaga (até SponsorMaxAheadDays). "" se não há.
func NextAvailableStart(db *gorm.DB, zoneID uint, from string, days int, now time.Time) (string, error) {
	window, err := SponsorDayRange(from, SponsorMaxAheadDays+days)
	if err != nil {
		return "", err
	}
	occ, err := SponsorOccupancy(db, zoneID, window, now)
	if err != nil {
		return "", err
	}
	slots := SponsorSlotsPerDay()
	for i := 0; i+days <= len(window) && i <= SponsorMaxAheadDays; i++ {
		ok := true
		for _, d := range window[i : i+days] {
			if occ[d] >= slots {
				ok = false
				break
			}
		}
		if ok {
			return window[i], nil
		}
	}
	return "", nil
}

// RotateSponsors ordena as lojas destacadas no instante t: ordem estável
// (reserva mais antiga primeiro) girada a cada hora. Com 3 lojas, cada uma
// fica 1/3 do dia em 1º lugar — ninguém paga o mesmo e fica sempre atrás.
func RotateSponsors(establishmentIDs []uint, t time.Time) []uint {
	n := len(establishmentIDs)
	if n < 2 {
		return establishmentIDs
	}
	local := t.In(StoreLocation())
	offset := (local.YearDay()*24 + local.Hour()) % n
	out := make([]uint, 0, n)
	out = append(out, establishmentIDs[offset:]...)
	out = append(out, establishmentIDs[:offset]...)
	return out
}

// SponsorsNow devolve as lojas em destaque agora (reservas ATIVAS que cobrem
// hoje), já na ordem do rodízio. Pendente não aparece: só quem pagou.
func SponsorsNow(db *gorm.DB, t time.Time) ([]uint, error) {
	today := SponsorToday(t)
	var bookings []SponsorBooking
	if err := db.Where("status = ? AND start_day <= ? AND end_day >= ?", SponsorBookingActive, today, today).
		Find(&bookings).Error; err != nil {
		return nil, err
	}
	sort.Slice(bookings, func(i, j int) bool { return bookings[i].ID < bookings[j].ID })
	seen := map[uint]bool{}
	ids := make([]uint, 0, len(bookings))
	for _, b := range bookings {
		if !seen[b.EstablishmentID] {
			seen[b.EstablishmentID] = true
			ids = append(ids, b.EstablishmentID)
		}
	}
	return RotateSponsors(ids, t), nil
}
