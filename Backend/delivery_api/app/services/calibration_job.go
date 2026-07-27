package services

import (
	"log"
	"math"
	"time"
)

// CalibrationConfig define parametros do job de calibracao.
type CalibrationConfig struct {
	// Intervalo entre calibracoes (ex: 24h para noturna, 1h para continua)
	Interval time.Duration
	// Alvo de taxa de pedidos nao matchados (ex: 0.05 = 5%)
	TargetUnmatchedRate float64
	// Limite superior de unmatched rate para considerar "alto" (ex: 0.15)
	HighUnmatchedRate float64
	// Alvo de tempo de matching P90 em ms (ex: 3000 = 3s)
	TargetMatchTimeP90 float64
	// Limite superior de match time para considerar "alto"
	HighMatchTimeP90 float64
	// Numero desejado de candidatos por busca
	TargetCouriers int
	// Fator de ajuste do raio (ex: 0.1 = ajusta 10% por vez)
	AdjustmentFactor float64
	// Raio maximo absoluto (nunca ultrapassa)
	GlobalMaxRadius float64
	// Raio minimo absoluto (nunca abaixo)
	GlobalMinRadius float64
}

// DefaultCalibrationConfig retorna a configuracao padrao de calibracao.
func DefaultCalibrationConfig() CalibrationConfig {
	return CalibrationConfig{
		Interval:            24 * time.Hour,
		TargetUnmatchedRate: 0.05,
		HighUnmatchedRate:   0.15,
		TargetMatchTimeP90:  3000,
		HighMatchTimeP90:    8000,
		TargetCouriers:      3,
		AdjustmentFactor:    0.1,
		GlobalMaxRadius:     20.0,
		GlobalMinRadius:     1.0,
	}
}

// CalibrationMetrics armazena as metricas coletadas para uma zona.
type CalibrationMetrics struct {
	ZoneID          uint
	ZoneName        string
	UnmatchedRate   float64
	MatchTimeP90Ms  float64
	CurrentRadiusKm float64
	MinRadiusKm     float64
	MaxRadiusKm     float64
	DensityPerKm2   float64
}

// CalibrationResult armazena o resultado da calibracao de uma zona.
type CalibrationResult struct {
	ZoneID          uint
	ZoneName        string
	OldRadiusKm     float64
	NewRadiusKm     float64
	Reason          string // "density", "high_unmatched", "high_match_time", "no_change"
	UnmatchedRate   float64
	MatchTimeP90Ms  float64
	DensityPerKm2   float64
}

// AutoCalibrationJob recalibra automaticamente os raios das zonas
// baseado nas metricas de matching coletadas.
//
// Logica:
//  1. Se unmatched_rate > HIGH → aumenta raio (poucos entregadores)
//  2. Se match_time P90 > HIGH → diminui raio (entregadores longe demais)
//  3. Se density > 0 → recalcula raio ideal por Poisson
//  4. Se unmatched_rate < TARGET e match_time < TARGET → mantem (ok)
//  5. Limita ajuste a AdjustmentFactor (ex: 10%) por ciclo
//  6. Respeita min/max radius da zona e GlobalMin/MaxRadius
type AutoCalibrationJob struct {
	config     CalibrationConfig
	engine     *MatchingEngine
	resolver   ZoneResolver
	onCalibrate func(result CalibrationResult)
}

// NewAutoCalibrationJob cria um novo job de calibracao.
func NewAutoCalibrationJob(config CalibrationConfig, engine *MatchingEngine, resolver ZoneResolver) *AutoCalibrationJob {
	return &AutoCalibrationJob{
		config:   config,
		engine:   engine,
		resolver: resolver,
	}
}

// SetOnCalibrate define callback chamado apos cada calibracao.
func (j *AutoCalibrationJob) SetOnCalibrate(fn func(result CalibrationResult)) {
	j.onCalibrate = fn
}

// RunOnce executa um ciclo de calibracao para todas as zonas conhecidas.
// Retorna os resultados de cada zona calibrada.
func (j *AutoCalibrationJob) RunOnce(zoneMetadatas []ZoneMetadata) []CalibrationResult {
	log.Printf("[CALIBRATION] Starting calibration cycle for %d zones", len(zoneMetadatas))

	var results []CalibrationResult

	for _, zone := range zoneMetadatas {
		if zone.ID == 0 {
			continue // pula zona default
		}

		result := j.calibrateZone(zone)
		results = append(results, result)

		if result.Reason != "no_change" {
			log.Printf("[CALIBRATION] Zone %q: %.1fkm -> %.1fkm (reason=%s, unmatched=%.1f%%, p90=%.0fms, density=%.3f/km²)",
				zone.Name, result.OldRadiusKm, result.NewRadiusKm,
				result.Reason, result.UnmatchedRate*100,
				result.MatchTimeP90Ms, result.DensityPerKm2)
		}

		if j.onCalibrate != nil {
			j.onCalibrate(result)
		}
	}

	return results
}

// calibrateZone executa a calibracao para uma zona especifica.
func (j *AutoCalibrationJob) calibrateZone(zone ZoneMetadata) CalibrationResult {
	result := CalibrationResult{
		ZoneID:      zone.ID,
		ZoneName:    zone.Name,
		OldRadiusKm: zone.RadiusKm,
		Reason:      "no_change",
	}

	// Coleta metricas POR ZONA (nao globais)
	result.UnmatchedRate = j.engine.GetUnmatchedRateForZone(zone.ID)
	result.MatchTimeP90Ms = j.engine.GetMatchTimeP90ForZone(zone.ID)

	// Fallback para metricas globais se nao houver dados por zona
	if result.UnmatchedRate == 0 && result.MatchTimeP90Ms == 0 {
		result.UnmatchedRate = j.engine.GetUnmatchedRate()
		result.MatchTimeP90Ms = j.engine.GetMatchTimeP90()
	}
	result.DensityPerKm2 = j.engine.CourierStore.GetZoneDensity(zone.ID)

	newRadius := zone.RadiusKm

	// 1. Se ha densidade, calcula raio ideal por Poisson
	if zone.DensityCouriersPerKm2 > 0 {
		poissonRadius := zone.GetSuggestedRadiusByDensity(j.config.TargetCouriers)
		if poissonRadius > 0 {
			// Suaviza: mistura raio atual com raio de Poisson (70% poisson, 30% atual)
			newRadius = poissonRadius*0.7 + newRadius*0.3
			result.Reason = "density"
		}
	}

	// 2. Se unmatched_rate > HIGH, aumenta raio
	if result.UnmatchedRate > j.config.HighUnmatchedRate {
		increase := newRadius * j.config.AdjustmentFactor
		newRadius = newRadius + increase
		result.Reason = "high_unmatched"
		log.Printf("[CALIBRATION] Zone %q: unmatched_rate=%.1f%% > %.1f%%, increasing radius by %.1fkm",
			zone.Name, result.UnmatchedRate*100, j.config.HighUnmatchedRate*100, increase)
	}

	// 3. Se match_time P90 > HIGH (entregadores longe demais), diminui raio
	if result.MatchTimeP90Ms > j.config.HighMatchTimeP90 {
		decrease := newRadius * j.config.AdjustmentFactor
		newRadius = newRadius - decrease
		result.Reason = "high_match_time"
		log.Printf("[CALIBRATION] Zone %q: match_p90=%.0fms > %.0fms, decreasing radius by %.1fkm",
			zone.Name, result.MatchTimeP90Ms, j.config.HighMatchTimeP90, decrease)
	}

	// 4. Respeita limites globais e da zona
	globalMin := math.Max(j.config.GlobalMinRadius, zone.MinRadiusKm)
	globalMax := math.Min(j.config.GlobalMaxRadius, zone.MaxRadiusKm)

	if newRadius < globalMin {
		newRadius = globalMin
	}
	if newRadius > globalMax {
		newRadius = globalMax
	}

	// Arredonda para 1 decimal
	newRadius = math.Round(newRadius*10) / 10

	result.NewRadiusKm = newRadius

	return result
}

// Start inicia o job de calibracao em background.
// Roda no intervalo configurado e tambem imediatamente na inicializacao.
// Aceita uma funcao para buscar as zonas (evita dependencia circular com GORM).
func (j *AutoCalibrationJob) Start(fetchZones func() []ZoneMetadata) {
	log.Printf("[CALIBRATION] Job started with interval %v", j.config.Interval)

	// Roda imediatamente na inicializacao
	go func() {
		time.Sleep(10 * time.Second) // espera o sistema estabilizar
		zones := fetchZones()
		if len(zones) > 0 {
			j.RunOnce(zones)
		}
	}()

	// Roda no intervalo configurado
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
