package services

import (
	"log"
	"time"
)

// SplitDecayConfig define os parametros do job de decaimento.
type SplitDecayConfig struct {
	Interval time.Duration // intervalo entre execucoes (ex: 24h para diario, 30d para mensal)
}

// DefaultSplitDecayConfig retorna config padrao (diario para testar, idealmente mensal).
func DefaultSplitDecayConfig() SplitDecayConfig {
	return SplitDecayConfig{
		Interval: 24 * time.Hour,
	}
}

// SplitDecayResult armazena o resultado do decaimento para uma zona.
type SplitDecayResult struct {
	ZoneID               uint
	ZoneName             string
	OldPlatformPct       float64
	NewPlatformPct       float64
	OldEstablishmentPct  float64
	NewEstablishmentPct  float64
	MonthlyOrders        int
	ActiveCouriers       int
	Applied              bool // true se o split foi ajustado
}

// ZoneSplitData representa os dados de zona necessarios para o decaimento.
type ZoneSplitData struct {
	ID                   uint
	Name                 string
	SplitCurrentPlatformPct       float64
	SplitCurrentEstablishmentPct  float64
	SplitInitialPlatformPct       float64
	SplitInitialEstablishmentPct  float64
	SplitTargetPlatformPct        float64
	SplitTargetEstablishmentPct   float64
	SplitStepMonths               int
	SplitStepPlatformPct          float64
	SplitStepEstablishmentPct     float64
	SplitMinMonthlyOrders         int
	SplitMinActiveCouriers        int
	SplitLastAdjustedAt           *time.Time
	CreatedAt                     time.Time
}

// ZoneMetricsProvider fornece as metricas de uma zona para o job de decaimento.
type ZoneMetricsProvider interface {
	GetMonthlyOrders(zoneID uint) int
	GetActiveCouriers(zoneID uint) int
}

// SplitDecayJob recalibra automaticamente o split das zonas
// baseado no tempo de maturidade da praça.
type SplitDecayJob struct {
	config          SplitDecayConfig
	metricsProvider ZoneMetricsProvider
	onDecay         func(result SplitDecayResult)
}

// NewSplitDecayJob cria um novo job de decaimento de split.
func NewSplitDecayJob(config SplitDecayConfig, metricsProvider ZoneMetricsProvider) *SplitDecayJob {
	return &SplitDecayJob{
		config:          config,
		metricsProvider: metricsProvider,
	}
}

// SetOnDecay define callback chamado apos cada decaimento de zona.
func (j *SplitDecayJob) SetOnDecay(fn func(result SplitDecayResult)) {
	j.onDecay = fn
}

// RunOnce executa um ciclo de decaimento para todas as zonas.
func (j *SplitDecayJob) RunOnce(zones []ZoneSplitData) []SplitDecayResult {
	log.Printf("[SPLIT_DECAY] Running decay cycle for %d zones", len(zones))

	var results []SplitDecayResult

	for _, zone := range zones {
		result := j.decayZone(zone)
		results = append(results, result)

		if result.Applied {
			log.Printf("[SPLIT_DECAY] Zone %q: %.1f%% -> %.1f%% (orders=%d, couriers=%d)",
				zone.Name, result.OldPlatformPct, result.NewPlatformPct,
				result.MonthlyOrders, result.ActiveCouriers)
		}

		if j.onDecay != nil {
			j.onDecay(result)
		}
	}

	return results
}

// decayZone calcula o novo split para uma zona.
func (j *SplitDecayJob) decayZone(zone ZoneSplitData) SplitDecayResult {
	result := SplitDecayResult{
		ZoneID:              zone.ID,
		ZoneName:            zone.Name,
		OldPlatformPct:      zone.SplitCurrentPlatformPct,
		OldEstablishmentPct: zone.SplitCurrentEstablishmentPct,
		NewPlatformPct:      zone.SplitCurrentPlatformPct,
		NewEstablishmentPct: zone.SplitCurrentEstablishmentPct,
		Applied:             false,
	}

	// Coleta metricas
	if j.metricsProvider != nil {
		result.MonthlyOrders = j.metricsProvider.GetMonthlyOrders(zone.ID)
		result.ActiveCouriers = j.metricsProvider.GetActiveCouriers(zone.ID)
	}

	// Calcula meses desde a ativacao
	if zone.CreatedAt.IsZero() {
		return result
	}

	now := time.Now()
	mesesDesdeAtivacao := monthsBetweenInt(zone.CreatedAt, now)
	stepsPossiveis := mesesDesdeAtivacao / zone.SplitStepMonths

	// Calcula steps ja aplicados
	stepsAplicados := 0
	if zone.SplitLastAdjustedAt != nil {
		mesesDesdeAjuste := monthsBetweenInt(zone.CreatedAt, *zone.SplitLastAdjustedAt)
		stepsAplicados = mesesDesdeAjuste / zone.SplitStepMonths
	}

	// Se ja aplicou todos, nada a fazer
	if stepsAplicados >= stepsPossiveis {
		return result
	}

	// Verifica gatilhos
	if result.MonthlyOrders < zone.SplitMinMonthlyOrders {
		log.Printf("[SPLIT_DECAY] Zone %q: skipping decay — only %d orders/mo (min %d)",
			zone.Name, result.MonthlyOrders, zone.SplitMinMonthlyOrders)
		return result
	}
	if result.ActiveCouriers < zone.SplitMinActiveCouriers {
		log.Printf("[SPLIT_DECAY] Zone %q: skipping decay — only %d couriers (min %d)",
			zone.Name, result.ActiveCouriers, zone.SplitMinActiveCouriers)
		return result
	}

	// Aplica steps pendentes
	novaPlatform := zone.SplitCurrentPlatformPct
	novaEstablishment := zone.SplitCurrentEstablishmentPct
	stepsParaAplicar := stepsPossiveis - stepsAplicados

	for i := 0; i < stepsParaAplicar; i++ {
		novaPlatform += zone.SplitStepPlatformPct
		novaEstablishment += zone.SplitStepEstablishmentPct

		// Nao ultrapassa o alvo
		if (zone.SplitStepPlatformPct > 0 && novaPlatform > zone.SplitTargetPlatformPct) ||
			(zone.SplitStepPlatformPct < 0 && novaPlatform < zone.SplitTargetPlatformPct) {
			novaPlatform = zone.SplitTargetPlatformPct
			novaEstablishment = zone.SplitTargetEstablishmentPct
			break
		}
	}

	result.NewPlatformPct = novaPlatform
	result.NewEstablishmentPct = novaEstablishment
	result.Applied = true

	return result
}

// Start inicia o job de decaimento em background.
func (j *SplitDecayJob) Start(fetchZones func() []ZoneSplitData) {
	log.Printf("[SPLIT_DECAY] Job started with interval %v", j.config.Interval)

	// Roda na inicializacao (apos 15s para o sistema estabilizar)
	go func() {
		time.Sleep(15 * time.Second)
		zones := fetchZones()
		if len(zones) > 0 {
			j.RunOnce(zones)
		}
	}()

	if j.config.Interval <= 0 {
		return
	}

	go func() {
		ticker := time.NewTicker(j.config.Interval)
		defer ticker.Stop()
		for range ticker.C {
			zones := fetchZones()
			if len(zones) > 0 {
				j.RunOnce(zones)
			}
		}
	}()
}

// monthsBetweenInt retorna o numero inteiro de meses entre duas datas.
func monthsBetweenInt(a, b time.Time) int {
	years := b.Year() - a.Year()
	months := int(b.Month()) - int(a.Month())
	total := years*12 + months
	if total < 0 {
		return 0
	}
	return total
}
