package models

import (
	"math"
	"time"

	"gorm.io/gorm"
)

// EstablishmentListing é a loja como a vitrine do app do cliente mostra.
type EstablishmentListing struct {
	Establishment
	OpeningStatus
	IsSponsored  bool    `json:"is_sponsored"`
	Rating       float64 `json:"rating"`        // média 1–5, uma casa; 0 = sem avaliação
	ReviewsCount int     `json:"reviews_count"` // quantas avaliações
}

// OrderListing define a ordem da vitrine:
//
//  1. Patrocinadas ABERTAS, na ordem do rodízio (sponsorOrder).
//  2. Demais abertas, na ordem recebida.
//  3. Fechadas pela grade no fim (dá para ver o cardápio, não pedir).
//
// Patrocinada fechada não ganha o topo nem o selo: pagar não faz a loja
// fechada atrapalhar quem quer pedir agora.
func OrderListing(items []EstablishmentListing, sponsorOrder []uint) []EstablishmentListing {
	rank := make(map[uint]int, len(sponsorOrder))
	for i, id := range sponsorOrder {
		rank[id] = i
	}
	sponsored := make([]EstablishmentListing, len(sponsorOrder))
	has := make([]bool, len(sponsorOrder))
	open := make([]EstablishmentListing, 0, len(items))
	closed := make([]EstablishmentListing, 0)
	for _, it := range items {
		it.IsSponsored = false
		if !it.IsOpen {
			closed = append(closed, it)
			continue
		}
		if r, ok := rank[it.ID]; ok {
			it.IsSponsored = true
			sponsored[r] = it
			has[r] = true
			continue
		}
		open = append(open, it)
	}
	out := make([]EstablishmentListing, 0, len(items))
	for i, it := range sponsored {
		if has[i] {
			out = append(out, it)
		}
	}
	out = append(out, open...)
	return append(out, closed...)
}

// BuildListing monta a vitrine com três consultas no total (horários,
// avaliações e destaques), não uma por loja.
func BuildListing(db *gorm.DB, establishments []Establishment, now time.Time) []EstablishmentListing {
	ids := make([]uint, 0, len(establishments))
	for _, e := range establishments {
		ids = append(ids, e.ID)
	}

	hoursBy := map[uint][]BusinessHours{}
	if len(ids) > 0 {
		var hours []BusinessHours
		if err := db.Where("establishment_id IN ?", ids).Find(&hours).Error; err == nil {
			for _, h := range hours {
				hoursBy[h.EstablishmentID] = append(hoursBy[h.EstablishmentID], h)
			}
		}
	}

	type ratingRow struct {
		EstablishmentID uint
		Avg             float64
		Count           int
	}
	ratings := map[uint]ratingRow{}
	if len(ids) > 0 {
		var rows []ratingRow
		// reviews é do orders_api (mesmo banco). Falha aqui (tabela ausente)
		// só deixa a vitrine sem nota.
		if err := db.Table("reviews").
			Select("establishment_id, AVG(rating) AS avg, COUNT(*) AS count").
			Where("establishment_id IN ?", ids).
			Group("establishment_id").Scan(&rows).Error; err == nil {
			for _, r := range rows {
				ratings[r.EstablishmentID] = r
			}
		}
	}

	sponsors, err := SponsorsNow(db, now)
	if err != nil {
		sponsors = nil
	}

	items := make([]EstablishmentListing, 0, len(establishments))
	for _, e := range establishments {
		r := ratings[e.ID]
		items = append(items, EstablishmentListing{
			Establishment: e,
			OpeningStatus: OpeningAt(e.OpenData != nil, hoursBy[e.ID], now),
			Rating:        math.Round(r.Avg*10) / 10,
			ReviewsCount:  r.Count,
		})
	}
	return OrderListing(items, sponsors)
}
