package models

import (
	"fmt"
	"log"
	"math"
	"time"
)

// Zone representa uma praça/regiao geografica que define as regras
// de split de pagamento, raio de entrega, taxa mínima e limites de
// entregadores. Cada estabelecimento pertence a uma zona.
// Se um estabelecimento nao tiver zona atribuida, usa os defaults.
type Zone struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	Name      string    `gorm:"size:100;not null;uniqueIndex" json:"name"`

	// Percentual da plataforma sobre o total do pedido (ex: 5.0 = 5%)
	PlatformFeePercentage float64 `gorm:"not null;default:5.0" json:"platform_fee_percentage"`
	// Percentual do estabelecimento sobre o total (ex: 85.0 = 85%)
	EstablishmentPercentage float64 `gorm:"not null;default:85.0" json:"establishment_percentage"`

	// === Campos geograficos (motor de despacho) ===
	City        string  `gorm:"size:100;index" json:"city"`
	State       string  `gorm:"size:50;index" json:"state"`
	GeohashPrefix string `gorm:"size:20;index" json:"geohash_prefix"` // prefixo para resolucao rapida

	// === Configuracao de raio de entrega ===
	// Raio minimo de entrega em km (nunca busca abaixo disso)
	MinRadiusKm float64 `gorm:"not null;default:2.0" json:"min_radius_km"`
	// Raio base de entrega em km (usado como primeiro estagio da busca progressiva)
	RadiusKm float64 `gorm:"not null;default:5.0" json:"radius_km"`
	// Raio maximo de entrega em km (limite absoluto, mesmo em busca progressiva)
	MaxRadiusKm float64 `gorm:"not null;default:15.0" json:"max_radius_km"`

	// === Configuracao de horario de pico ===
	// Inicio do horario de pico (formato HH:MM, ex: "11:00")
	PeakHourStart string `gorm:"size:5;default:'11:00'" json:"peak_hour_start"`
	// Fim do horario de pico (formato HH:MM, ex: "14:00")
	PeakHourEnd string `gorm:"size:5;default:'14:00'" json:"peak_hour_end"`
	// Multiplicador de raio em horario de pico (ex: 0.7 = raio 30% menor porque tem mais entregadores)
	PeakRadiusMultiplier float64 `gorm:"not null;default:0.7" json:"peak_radius_multiplier"`

	// === Porte da cidade (para calibracao inicial sem dados historicos) ===
	// Valores: "small" (<20k), "medium" (20-100k), "large" (100k-1M), "metro" (>1M)
	CitySize string `gorm:"size:20;default:'medium'" json:"city_size"`

	// === Densidade estimada de entregadores ===
	// Entregadores ativos por km² (calculado pelo job de calibracao automatica)
	DensityCouriersPerKm2 float64 `gorm:"default:0" json:"density_couriers_per_km2"`
	// Data da ultima calibracao
	LastCalibratedAt *time.Time `json:"last_calibrated_at"`

	// Taxa de entrega minima (R$)
	MinDeliveryFee float64 `gorm:"not null;default:5.0" json:"min_delivery_fee"`
	// Multiplicador de surge pricing (1.0 = normal, 1.5 = 50% mais caro)
	SurgeMultiplier float64 `gorm:"not null;default:1.0" json:"surge_multiplier"`
	// Abaixo deste numero de entregadores ativos, ativa fallback comunitario
	MinCouriersThreshold int `gorm:"not null;default:3" json:"min_couriers_threshold"`
	// Algoritmo de match: proximity, round_robin, least_loaded
	MatchAlgorithm string `gorm:"size:30;not null;default:proximity" json:"match_algorithm"`
	// Se permite batching (entregador pegar mais de 1 pedido na mesma rota)
	AllowBatching bool `gorm:"not null;default:true" json:"allow_batching"`

// === Decaimento automatico do split (maturidade da praca) ===
	// Split inicial (praca nova, mercado nao maduro)
	SplitInitialPlatformPct float64 `gorm:"default:3.0" json:"split_initial_platform_pct"`
	SplitInitialEstablishmentPct float64 `gorm:"default:87.0" json:"split_initial_establishment_pct"`

	// Split alvo (praca madura, padrao de mercado)
	SplitTargetPlatformPct float64 `gorm:"default:12.0" json:"split_target_platform_pct"`
	SplitTargetEstablishmentPct float64 `gorm:"default:78.0" json:"split_target_establishment_pct"`

	// Decaimento: a cada N meses sobe X%
	SplitStepMonths int `gorm:"default:3" json:"split_step_months"`         // a cada 3 meses
	SplitStepPlatformPct float64 `gorm:"default:1.5" json:"split_step_platform_pct"` // sobe 1.5%
	SplitStepEstablishmentPct float64 `gorm:"default:-1.5" json:"split_step_establishment_pct"` // desce 1.5%

	// Gatilhos: nao sobe se nao atingiu metricas
	SplitMinMonthlyOrders int `gorm:"default:50" json:"split_min_monthly_orders"`  // minimo 50 pedidos/mes
	SplitMinActiveCouriers int `gorm:"default:3" json:"split_min_active_couriers"` // minimo 3 entregadores

	// Split atual efetivo (cache, atualizado pelo job de decaimento)
	SplitCurrentPlatformPct float64 `gorm:"default:3.0" json:"split_current_platform_pct"`
	SplitCurrentEstablishmentPct float64 `gorm:"default:87.0" json:"split_current_establishment_pct"`

	// Ultima vez que o split foi ajustado
	SplitLastAdjustedAt *time.Time `json:"split_last_adjusted_at"`

	IsActive  bool      `gorm:"default:true" json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (Zone) TableName() string {
	return "zones"
}

// IsPeakHour retorna true se o horario atual esta dentro da janela de pico da zona.
func (z *Zone) IsPeakHour() bool {
	if z.PeakHourStart == "" || z.PeakHourEnd == "" {
		return false
	}
	now := time.Now()
	current := now.Format("15:04")
	return current >= z.PeakHourStart && current <= z.PeakHourEnd
}

// GetEffectiveRadius retorna o raio efetivo considerando horario de pico.
// Em horario de pico (mais entregadores), o raio diminui.
func (z *Zone) GetEffectiveRadius() float64 {
	if z.IsPeakHour() {
		adjusted := z.RadiusKm * z.PeakRadiusMultiplier
		if adjusted < z.MinRadiusKm {
			return z.MinRadiusKm
		}
		return adjusted
	}
	return z.RadiusKm
}

// GetSuggestedRadiusByDensity calcula o raio ideal baseado na densidade
// de entregadores, usando a formula de raiz quadrada do processo de Poisson:
// raio = sqrt( N_desejado / (pi * densidade) )
func (z *Zone) GetSuggestedRadiusByDensity(targetCouriers int) float64 {
	if z.DensityCouriersPerKm2 <= 0 {
		return z.GetEffectiveRadius()
	}
	suggested := math.Sqrt(float64(targetCouriers) / (math.Pi * z.DensityCouriersPerKm2))
	if suggested < z.MinRadiusKm {
		return z.MinRadiusKm
	}
	if suggested > z.MaxRadiusKm {
		return z.MaxRadiusKm
	}
	return suggested
}

// GetRadiusStages retorna os 3 estagios de raio para busca progressiva.
// Estagio 1: raio efetivo (base ou pico)
// Estagio 2: raio efetivo * 1.7
// Estagio 3: max_radius_km
func (z *Zone) GetRadiusStages() [3]float64 {
	base := z.GetEffectiveRadius()
	return [3]float64{
		base,
		math.Min(base*1.7, z.MaxRadiusKm),
		z.MaxRadiusKm,
	}
}

// GetInitialRadiusByCitySize retorna raios iniciais baseados no porte da cidade.
// Usado quando nao ha dado historico de densidade ainda.
func (z *Zone) GetInitialRadiusByCitySize() (base, min, max float64) {
	switch z.CitySize {
	case "small":
		return 6.0, 4.0, 15.0
	case "medium":
		return 4.0, 2.5, 10.0
	case "large":
		return 2.5, 1.5, 6.0
	case "metro":
		return 2.0, 1.0, 5.0
	default:
		return 5.0, 2.0, 10.0
	}
}

// GetSplitConfig busca a configuracao de split da zona
// associada a um estabelecimento. Se o estabelecimento nao tiver
// zona, ou a zona estiver inativa, retorna os percentuais padrao (5/85).
func GetZoneSplitConfig(establishmentID uint) (platformPct, establishmentPct float64) {
	defaultPlatform := 5.0
	defaultEstablishment := 85.0

	if DB == nil {
		return defaultPlatform, defaultEstablishment
	}

	var est Establishment
	if err := DB.First(&est, establishmentID).Error; err != nil {
		log.Printf("[ZONE] Establishment %d not found, using defaults: %v", establishmentID, err)
		return defaultPlatform, defaultEstablishment
	}

	if est.ZoneID == nil || *est.ZoneID == 0 {
		return defaultPlatform, defaultEstablishment
	}

	var zone Zone
	if err := DB.First(&zone, *est.ZoneID).Error; err != nil {
		log.Printf("[ZONE] Zone %d not found for establishment %d, using defaults: %v", *est.ZoneID, establishmentID, err)
		return defaultPlatform, defaultEstablishment
	}

	if !zone.IsActive {
		log.Printf("[ZONE] Zone %d is inactive for establishment %d, using defaults", *est.ZoneID, establishmentID)
		return defaultPlatform, defaultEstablishment
	}

	// Aplica decaimento automatico do split
	// Le metricas da zona para calcular o split efetivo
	// Nota: monthlyOrders e activeCouriers seriam preenchidos por consulta real
	// Aqui usamos o split cacheado (SplitCurrent*) para performance
	platformPct, establishmentPct = zone.SplitCurrentPlatformPct, zone.SplitCurrentEstablishmentPct

	log.Printf("[ZONE] Establishment %d uses zone %q: platform=%.1f%% establishment=%.1f%% (decay=%s)",
		establishmentID, zone.Name, platformPct, establishmentPct, formatDecayStatus(zone))

	return platformPct, establishmentPct
}

// formatDecayStatus retorna o status do decaimento para logging.
func formatDecayStatus(z Zone) string {
	if z.SplitLastAdjustedAt == nil {
		return "initial"
	}
	if z.SplitCurrentPlatformPct >= z.SplitTargetPlatformPct {
		return "target"
	}
	return fmt.Sprintf("step: %.1f%% -> %.1f%%", z.SplitInitialPlatformPct, z.SplitTargetPlatformPct)
}

// GetZoneByEstablishment retorna a zona completa de um estabelecimento.
// Se nao tiver zona, retorna nil sem erro.
func GetZoneByEstablishment(establishmentID uint) (*Zone, error) {
	if DB == nil {
		return nil, nil
	}

	var est Establishment
	if err := DB.First(&est, establishmentID).Error; err != nil {
		return nil, err
	}

	if est.ZoneID == nil || *est.ZoneID == 0 {
		return nil, nil
	}

	var zone Zone
	if err := DB.First(&zone, *est.ZoneID).Error; err != nil {
		return nil, err
	}

	if !zone.IsActive {
		return nil, nil
	}

	return &zone, nil
}

// ResolveZoneByCity retorna a primeira zona ativa que corresponde
// a uma cidade/estado (case-insensitive, partial match).
func ResolveZoneByCity(city, state string) *Zone {
	if DB == nil {
		return nil
	}

	var zone Zone
	query := DB.Where("is_active = ?", true)

	if city != "" {
		query = query.Where("LOWER(city) = LOWER(?)", city)
	}
	if state != "" {
		query = query.Where("LOWER(state) = LOWER(?)", state)
	}

	if err := query.Order("radius_km desc").First(&zone).Error; err != nil {
		return nil
	}

	return &zone
}

// CalculateEffectiveSplit calcula o split efetivo da zona considerando o decaimento
// automatico por maturidade da praça.
//
// Logica:
// 1. Calcula quantos steps ja deveriam ter ocorrido desde a ativacao
// 2. Se o split atual (cache) ja esta atualizado, retorna ele
// 3. Se passou tempo suficiente E as metricas minimas foram atingidas, aplica o step
// 4. Nunca ultrapassa o split alvo
func (z *Zone) CalculateEffectiveSplit(monthlyOrders, activeCouriers int) (platformPct, establishmentPct float64) {
	// Se nunca foi ativada, usa o inicial
	if z.CreatedAt.IsZero() {
		return z.SplitInitialPlatformPct, z.SplitInitialEstablishmentPct
	}

	activatedAt := z.CreatedAt
	now := time.Now()

	// Calcula steps desde a ativacao
	mesesDesdeAtivacao := monthsBetween(activatedAt, now)
	stepsPossiveis := mesesDesdeAtivacao / z.SplitStepMonths

	// Calcula steps ja aplicados
	stepsAplicados := 0
	if z.SplitLastAdjustedAt != nil {
		mesesDesdeAjuste := monthsBetween(activatedAt, *z.SplitLastAdjustedAt)
		stepsAplicados = mesesDesdeAjuste / z.SplitStepMonths
	}

	// Se ja aplicou todos os steps possiveis, mantem atual
	if stepsAplicados >= stepsPossiveis {
		return z.SplitCurrentPlatformPct, z.SplitCurrentEstablishmentPct
	}

	// Verifica gatilhos de maturidade
	if monthlyOrders < z.SplitMinMonthlyOrders {
		return z.SplitCurrentPlatformPct, z.SplitCurrentEstablishmentPct
	}
	if activeCouriers < z.SplitMinActiveCouriers {
		return z.SplitCurrentPlatformPct, z.SplitCurrentEstablishmentPct
	}

	// Aplica steps pendentes
	novaPlatform := z.SplitCurrentPlatformPct
	novaEstablishment := z.SplitCurrentEstablishmentPct

	stepsParaAplicar := stepsPossiveis - stepsAplicados
	for i := 0; i < stepsParaAplicar; i++ {
		novaPlatform += z.SplitStepPlatformPct
		novaEstablishment += z.SplitStepEstablishmentPct

		// Nao ultrapassa o alvo
		if (z.SplitStepPlatformPct > 0 && novaPlatform > z.SplitTargetPlatformPct) ||
			(z.SplitStepPlatformPct < 0 && novaPlatform < z.SplitTargetPlatformPct) {
			novaPlatform = z.SplitTargetPlatformPct
			novaEstablishment = z.SplitTargetEstablishmentPct
			break
		}
	}

	return novaPlatform, novaEstablishment
}

// monthsBetween retorna o numero inteiro de meses entre duas datas.
func monthsBetween(a, b time.Time) int {
	years := b.Year() - a.Year()
	months := int(b.Month()) - int(a.Month())
	total := years*12 + months
	if total < 0 {
		return 0
	}
	return total
}

// HaversineDistance calcula a distancia em km entre dois pontos geograficos.
func HaversineDistance(lat1, lng1, lat2, lng2 float64) float64 {
	const R = 6371.0 // raio da Terra em km

	dLat := (lat2 - lat1) * math.Pi / 180
	dLng := (lng2 - lng1) * math.Pi / 180

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*
			math.Sin(dLng/2)*math.Sin(dLng/2)

	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return R * c
}
