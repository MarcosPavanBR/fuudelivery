package models

import (
	"math"
	"time"
)

// Planos de patrocínio disponíveis
const (
	SponsorPlanBasic   = "basic"   // R$ 199/mês: destaque no topo da busca
	SponsorPlanPremium = "premium" // R$ 499/mês: destaque + banner + push notification
)

// Status do patrocínio
const (
	SponsorStatusActive    = "active"
	SponsorStatusExpired   = "expired"
	SponsorStatusCancelled = "cancelled"
)

// Slots máximos de patrocínio por zona, por plano
// Evita que a busca vire só anúncio
const (
	MaxSponsoredSlotsBasicPerZone   = 5 // até 5 estabelecimentos basic por zona
	MaxSponsoredSlotsPremiumPerZone = 3 // até 3 estabelecimentos premium por zona
	MaxSponsoredSlotsTotalPerZone   = 8 // total combinado: no máximo 8
)

// SponsoredListing representa um patrocínio de cardápio de um estabelecimento.
// O restaurante paga para aparecer com destaque no topo da listagem de busca.
// É a receita de maior margem do modelo, pois não depende de custo logístico.
type SponsoredListing struct {
	ID              uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	EstablishmentID uint   `gorm:"not null;uniqueIndex:idx_sponsored_est_zone" json:"establishment_id"`
	ZoneID          uint   `gorm:"not null;uniqueIndex:idx_sponsored_est_zone" json:"zone_id"`
	Plan            string `gorm:"size:20;not null;default:'basic'" json:"plan"`

	// Status: active, expired, cancelled
	Status string `gorm:"size:20;not null;default:'active'" json:"status"`

	// Valor mensal do patrocínio (R$)
	Amount float64 `gorm:"not null" json:"amount"`

	// Período vigente
	StartDate   time.Time  `json:"start_date"`
	EndDate     time.Time  `json:"end_date"`
	CancelledAt *time.Time `json:"cancelled_at,omitempty"`

	// Benefícios
	// Prioridade na ordenação (maior = aparece primeiro)
	Priority int `gorm:"not null;default:0" json:"priority"`
	// Se tem direito a banner extra
	HasBanner bool `gorm:"not null;default:false" json:"has_banner"`
	// Se tem direito a push notification
	HasPushNotification bool `gorm:"not null;default:false" json:"has_push_notification"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (SponsoredListing) TableName() string {
	return "sponsored_listings"
}

// IsActive retorna true se o patrocínio está ativo e dentro do período vigente.
func (s *SponsoredListing) IsActive() bool {
	if s.Status != SponsorStatusActive {
		return false
	}
	now := time.Now()
	return now.After(s.StartDate) && now.Before(s.EndDate)
}

// GetSponsorPlanAmount retorna o valor do plano de patrocínio.
func GetSponsorPlanAmount(plan string) float64 {
	switch plan {
	case SponsorPlanBasic:
		return 199.00
	case SponsorPlanPremium:
		return 499.00
	default:
		return 199.00
	}
}

// GetSponsorPlanBenefits retorna os benefícios de cada plano.
func GetSponsorPlanBenefits(plan string) (hasBanner, hasPushNotification bool) {
	switch plan {
	case SponsorPlanBasic:
		return false, false
	case SponsorPlanPremium:
		return true, true
	default:
		return false, false
	}
}

// CheckSponsoredSlotsAvailable verifica se ainda há vagas de patrocínio disponíveis
// para uma determinada zona e plano.
// Retorna true se disponível, false se o limite de slots foi atingido.
func CheckSponsoredSlotsAvailable(zoneID uint, plan string) (bool, error) {
	if DB == nil {
		return true, nil
	}

	// Conta slots ativos totais na zona
	var totalActive int64
	if err := DB.Model(&SponsoredListing{}).
		Where("zone_id = ? AND status = ? AND end_date > ?", zoneID, SponsorStatusActive, time.Now()).
		Count(&totalActive).Error; err != nil {
		return false, err
	}

	// Se já atingiu o total máximo, não aceita mais
	if totalActive >= MaxSponsoredSlotsTotalPerZone {
		return false, nil
	}

	// Conta slots ativos por plano
	var planActive int64
	planLimit := MaxSponsoredSlotsBasicPerZone
	if plan == SponsorPlanPremium {
		planLimit = MaxSponsoredSlotsPremiumPerZone
	}

	if err := DB.Model(&SponsoredListing{}).
		Where("zone_id = ? AND plan = ? AND status = ? AND end_date > ?",
			zoneID, plan, SponsorStatusActive, time.Now()).
		Count(&planActive).Error; err != nil {
		return false, err
	}

	return planActive < int64(planLimit), nil
}

// RankListings ordena uma lista de estabelecimentos colocando os patrocinados
// no topo e mantendo a ordem natural dos não-patrocinados depois.
//
// Lógica:
// 1. Patrocinados ativos vêm no topo, ordenados por priority (maior primeiro)
// 2. Dentro da mesma priority, ordena por nome
// 3. Não-patrocinados vêm depois, na ordem original
//
// zoneID: zona onde a busca está acontecendo
// establishments: slice dos estabelecimentos a ordenar (já pode vir filtrado por zona)
// establishmentNames: mapa de ID -> nome para ordenação secundária
func RankListings(zoneID uint, establishments []Establishment) []Establishment {
	if DB == nil || len(establishments) == 0 {
		return establishments
	}

	// Busca patrocínios ativos para esta zona
	var activeSponsors []SponsoredListing
	if err := DB.Where("zone_id = ? AND status = ? AND end_date > ?",
		zoneID, SponsorStatusActive, time.Now()).
		Order("priority desc, created_at asc").
		Find(&activeSponsors).Error; err != nil {
		return establishments
	}

	if len(activeSponsors) == 0 {
		return establishments
	}

	// Mapa de establishment_id -> priority para quick lookup
	sponsorPriority := make(map[uint]int)
	for _, s := range activeSponsors {
		sponsorPriority[s.EstablishmentID] = s.Priority
	}

	// Divide: patrocinados vs não-patrocinados
	sponsored := make([]Establishment, 0, len(activeSponsors))
	nonSponsored := make([]Establishment, 0, len(establishments))

	for _, est := range establishments {
		if prio, ok := sponsorPriority[est.ID]; ok && prio > 0 {
			sponsored = append(sponsored, est)
		} else {
			nonSponsored = append(nonSponsored, est)
		}
	}

	// Ordena patrocinados por priority desc, depois por nome
	for i := 0; i < len(sponsored); i++ {
		for j := i + 1; j < len(sponsored); j++ {
			pi := sponsorPriority[sponsored[i].ID]
			pj := sponsorPriority[sponsored[j].ID]
			if pi < pj || (pi == pj && sponsored[i].Name > sponsored[j].Name) {
				sponsored[i], sponsored[j] = sponsored[j], sponsored[i]
			}
		}
	}

	// Concatena: patrocinados primeiro, depois não-patrocinados
	result := make([]Establishment, 0, len(establishments))
	result = append(result, sponsored...)
	result = append(result, nonSponsored...)

	return result
}

// GetSponsoredByEstablishment retorna o patrocínio ativo de um estabelecimento em uma zona.
func GetSponsoredByEstablishment(establishmentID, zoneID uint) (*SponsoredListing, error) {
	if DB == nil {
		return nil, nil
	}

	var sponsor SponsoredListing
	if err := DB.Where("establishment_id = ? AND zone_id = ? AND status = ?",
		establishmentID, zoneID, SponsorStatusActive).
		First(&sponsor).Error; err != nil {
		return nil, err
	}

	return &sponsor, nil
}

// GetSponsoredListingsByZone retorna todos os patrocínios ativos de uma zona.
func GetSponsoredListingsByZone(zoneID uint) ([]SponsoredListing, error) {
	if DB == nil {
		return nil, nil
	}

	var sponsors []SponsoredListing
	if err := DB.Where("zone_id = ? AND status = ?", zoneID, SponsorStatusActive).
		Order("priority desc, created_at asc").
		Find(&sponsors).Error; err != nil {
		return nil, err
	}

	return sponsors, nil
}

// GetFeaturedEstablishments retorna os estabelecimentos em destaque (patrocinados)
// em uma zona, com informações de plano e benefícios.
// Este é o endpoint público que o frontend consome para exibir os cards de destaque.
func GetFeaturedEstablishments(zoneID uint, limit int) ([]map[string]interface{}, error) {
	if DB == nil {
		return nil, nil
	}

	if limit <= 0 {
		limit = MaxSponsoredSlotsTotalPerZone
	}

	var sponsors []SponsoredListing
	if err := DB.Where("zone_id = ? AND status = ? AND end_date > ?",
		zoneID, SponsorStatusActive, time.Now()).
		Order("priority desc, created_at asc").
		Limit(limit).
		Find(&sponsors).Error; err != nil {
		return nil, err
	}

	if len(sponsors) == 0 {
		return []map[string]interface{}{}, nil
	}

	// Busca dados dos estabelecimentos patrocinados
	result := make([]map[string]interface{}, 0, len(sponsors))
	for _, s := range sponsors {
		var est Establishment
		if err := DB.First(&est, s.EstablishmentID).Error; err != nil {
			continue
		}

		item := map[string]interface{}{
			"sponsor_id":            s.ID,
			"establishment_id":      est.ID,
			"establishment_name":    est.Name,
			"establishment_image":   est.Image,
			"description":           est.Description,
			"primary_color":         est.PrimaryColor,
			"secondary_color":       est.SecondaryColor,
			"plan":                  s.Plan,
			"priority":              s.Priority,
			"has_banner":            s.HasBanner,
			"has_push_notification": s.HasPushNotification,
			"amount":                s.Amount,
		}

		// Inclui avaliação média se existir (join opcional com reviews)
		type ReviewStats struct {
			AverageRating float64
			TotalReviews  int
		}
		var stats ReviewStats
		DB.Table("reviews").
			Select("COALESCE(AVG(rating), 0) as average_rating, COUNT(*) as total_reviews").
			Where("establishment_id = ?", est.ID).
			Scan(&stats)

		item["average_rating"] = math.Round(stats.AverageRating*10) / 10
		item["total_reviews"] = stats.TotalReviews

		result = append(result, item)
	}

	return result, nil
}

// PriorityScheduler calcula a prioridade automaticamente para um novo patrocínio
// baseado no plano e na quantidade de slots já ocupados.
// Quanto mais cheia a zona, menor a prioridade do próximo entrante.
func PriorityScheduler(zoneID uint, plan string) int {
	basePriority := 1000
	if plan == SponsorPlanPremium {
		basePriority = 2000
	}

	if DB == nil {
		return basePriority
	}

	// Quantos já estão ativos?
	var count int64
	DB.Model(&SponsoredListing{}).
		Where("zone_id = ? AND status = ?", zoneID, SponsorStatusActive).
		Count(&count)

	// Reduz prioridade conforme lotação: cada slot ativo reduz 50 pontos
	adjusted := basePriority - int(count)*50
	if adjusted < 100 {
		adjusted = 100
	}

	return adjusted
}
