package main

// Motor de despacho: instâncias globais, inicialização, métricas de split
// e o resolvedor de zonas no Postgres.

import (
	"encoding/json"
	"log"
	"strings"
	"time"

	"gorm.io/gorm"

	// Models (database initialization)
	"github.com/carloshomar/fuudelivery/auth_api/app/models"

	// Handlers

	deliveryHandlers "github.com/carloshomar/fuudelivery/delivery_api/app/handlers"

	// Middleware

	// Dispatch engine
	dispatchServices "github.com/carloshomar/fuudelivery/delivery_api/app/services"
	// Batch expiry

	// Queue + Health + Upload + Metrics + Search

	"github.com/carloshomar/fuudelivery/pkg/metrics"
	"github.com/carloshomar/fuudelivery/pkg/queue"
)

// === DISPATCH ENGINE (global instances) ===
var (
	courierStore    *dispatchServices.CourierStore
	matchingEngine  *dispatchServices.MatchingEngine
	dispatchHandler *deliveryHandlers.DispatchHandler
	calibrationJob  *dispatchServices.AutoCalibrationJob
	splitDecayJob   *dispatchServices.SplitDecayJob
)

func initDispatchEngine(db *gorm.DB) {
	courierStore = dispatchServices.NewCourierStore()

	// Zone resolver: consulta PostgreSQL via GORM
	zoneResolver := &zoneDBResolver{DB: db}

	// DLQ PERSISTENTE, não em memória. A NewDLQStore in-memory descarta o pedido
	// mais antigo em SILÊNCIO quando enche (matching_engine.go:52,
	// `d.orders = d.orders[1:]`) e perde tudo num restart — e restart aqui é
	// rotina, não exceção: o free tier do Render derruba o processo por
	// inatividade. Pedido não casado que some é pedido pago que nunca recebe
	// entregador, sem nada no backend percebendo.
	//
	// A PostgresDLQStore e o construtor WithDLQ já existiam, testados, com a
	// tabela criada em sql/17_unmatched_orders.sql — só nunca tinham sido
	// ligados aqui.
	matchingEngine = dispatchServices.NewMatchingEngineWithDLQ(
		courierStore, zoneResolver, dispatchServices.NewPostgresDLQStore(db))

	// Expõe a profundidade da DLQ no /metrics. É um hook porque o pacote de
	// métricas não pode importar main; sem ele, a fila de pedidos sem
	// entregador continuaria invisível — que é metade do problema que a DLQ
	// persistente resolve (a outra metade é não perdê-los no restart).
	metrics.DLQDepthFunc = func() int {
		if matchingEngine == nil || matchingEngine.DLQ == nil {
			return 0
		}
		return matchingEngine.DLQ.Len()
	}

	// Callback: quando um pedido e matchado, publica no canal de delivery_updates
	matchingEngine.OnMatch = func(orderID string, courierID int64) {
		data, _ := json.Marshal(map[string]interface{}{
			"type":       "order_matched",
			"order_id":   orderID,
			"courier_id": courierID,
			"matched_at": time.Now().UTC(),
		})
		if err := queue.Publish("delivery_updates", data); err != nil {
			log.Printf("[QUEUE] ERRO ao publicar order_matched (%s): %v", orderID, err)
		}
	}

	// Callback: fallback comunitario ativado
	matchingEngine.OnFallback = func(orderID string, zoneName string) {
		log.Printf("[FALLBACK] Order %s needs community fallback in zone %q", orderID, zoneName)
		data, _ := json.Marshal(map[string]interface{}{
			"type":      "community_fallback",
			"order_id":  orderID,
			"zone_name": zoneName,
			"time":      time.Now().UTC(),
		})
		if err := queue.Publish("delivery_updates", data); err != nil {
			log.Printf("[QUEUE] ERRO ao publicar community_fallback (%s): %v", orderID, err)
		}
	}

	// Inicia retry loop a cada 30s
	matchingEngine.StartRetryLoop(30 * time.Second)

	// Inicia cleanup de couriers stale a cada 5min
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			courierStore.CleanupStale(300) // 5 minutos
		}
	}()

	// Cria handler HTTP (corte 3: dispatch não usa mais Mongo)
	dispatchHandler = deliveryHandlers.NewDispatchHandler(courierStore, matchingEngine)

	// === Job de calibracao automatica ===
	calConfig := dispatchServices.DefaultCalibrationConfig()
	calConfig.Interval = 24 * time.Hour
	calibrationJob = dispatchServices.NewAutoCalibrationJob(calConfig, matchingEngine, zoneResolver)

	// Callback: quando uma zona e calibrada, persiste o novo raio no banco
	calibrationJob.SetOnCalibrate(func(result dispatchServices.CalibrationResult) {
		if result.OldRadiusKm == result.NewRadiusKm {
			return
		}
		if db == nil {
			return
		}
		if err := db.Model(&models.Zone{}).Where("id = ?", result.ZoneID).Update("radius_km", result.NewRadiusKm).Error; err != nil {
			log.Printf("[CALIBRATION] Failed to update radius for zone %d: %v", result.ZoneID, err)
		}
	})

	// Funcao de busca de zonas para o job
	fetchZones := func() []dispatchServices.ZoneMetadata {
		if db == nil {
			return nil
		}
		var zones []models.Zone
		if err := db.Where("is_active = ?", true).Find(&zones).Error; err != nil {
			log.Printf("[CALIBRATION] Failed to fetch zones: %v", err)
			return nil
		}
		result := make([]dispatchServices.ZoneMetadata, 0, len(zones))
		for _, z := range zones {
			result = append(result, dispatchServices.ZoneMetadata{
				ID:                    z.ID,
				Name:                  z.Name,
				MinRadiusKm:           z.MinRadiusKm,
				RadiusKm:              z.RadiusKm,
				MaxRadiusKm:           z.MaxRadiusKm,
				PeakRadiusMultiplier:  z.PeakRadiusMultiplier,
				PeakHourStart:         z.PeakHourStart,
				PeakHourEnd:           z.PeakHourEnd,
				CitySize:              z.CitySize,
				DensityCouriersPerKm2: z.DensityCouriersPerKm2,
				MinDeliveryFee:        z.MinDeliveryFee,
				SurgeMultiplier:       z.SurgeMultiplier,
				MinCouriersThreshold:  z.MinCouriersThreshold,
				AllowBatching:         z.AllowBatching,
			})
		}
		return result
	}

	// Inicia o job de calibracao
	calibrationJob.Start(fetchZones)

	// Inicia recalculo periodico de densidade (a cada 15min)
	go func() {
		ticker := time.NewTicker(15 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			zones := fetchZones()
			if len(zones) == 0 {
				continue
			}
			// Converte ZoneMetadata para ZoneInfo
			zoneInfos := make([]dispatchServices.ZoneInfo, len(zones))
			for i, z := range zones {
				// Calcula centroide real a partir dos estabelecimentos da zona
				var centerLat, centerLng float64
				if db != nil {
					var ests []models.Establishment
					db.Select("lat, long").Where("zone_id = ? AND lat != 0", z.ID).Find(&ests)
					if len(ests) > 0 {
						sumLat, sumLng := 0.0, 0.0
						for _, e := range ests {
							sumLat += e.Lat
							sumLng += e.Long
						}
						centerLat = sumLat / float64(len(ests))
						centerLng = sumLng / float64(len(ests))
					}
				}
				zoneInfos[i] = dispatchServices.ZoneInfo{
					ID:        z.ID,
					CenterLat: centerLat,
					CenterLng: centerLng,
					RadiusKm:  z.RadiusKm,
				}
			}
			courierStore.RecalculateAllDensities(zoneInfos)
		}
	}()

	// === Job de decaimento de split ===
	splitDecayConfig := dispatchServices.DefaultSplitDecayConfig()
	splitDecayJob = dispatchServices.NewSplitDecayJob(splitDecayConfig, &splitMetricsProvider{DB: db})

	// Callback: persiste o novo split no banco
	splitDecayJob.SetOnDecay(func(result dispatchServices.SplitDecayResult) {
		if !result.Applied {
			return
		}
		if db == nil {
			return
		}
		now := time.Now()
		updates := map[string]interface{}{
			"split_current_platform_pct":      result.NewPlatformPct,
			"split_current_establishment_pct": result.NewEstablishmentPct,
			"split_last_adjusted_at":          now,
		}
		if err := db.Model(&models.Zone{}).Where("id = ?", result.ZoneID).Updates(updates).Error; err != nil {
			log.Printf("[SPLIT_DECAY] Failed to update split for zone %d: %v", result.ZoneID, err)
		} else {
			log.Printf("[SPLIT_DECAY] Zone %d split updated: %.1f%% -> %.1f%%",
				result.ZoneID, result.OldPlatformPct, result.NewPlatformPct)
		}
	})

	// Funcao de busca de dados de split das zonas
	fetchSplitZones := func() []dispatchServices.ZoneSplitData {
		if db == nil {
			return nil
		}
		var zones []models.Zone
		if err := db.Where("is_active = ?", true).Find(&zones).Error; err != nil {
			log.Printf("[SPLIT_DECAY] Failed to fetch zones: %v", err)
			return nil
		}
		result := make([]dispatchServices.ZoneSplitData, 0, len(zones))
		for _, z := range zones {
			result = append(result, dispatchServices.ZoneSplitData{
				ID:                           z.ID,
				Name:                         z.Name,
				SplitCurrentPlatformPct:      z.SplitCurrentPlatformPct,
				SplitCurrentEstablishmentPct: z.SplitCurrentEstablishmentPct,
				SplitInitialPlatformPct:      z.SplitInitialPlatformPct,
				SplitInitialEstablishmentPct: z.SplitInitialEstablishmentPct,
				SplitTargetPlatformPct:       z.SplitTargetPlatformPct,
				SplitTargetEstablishmentPct:  z.SplitTargetEstablishmentPct,
				SplitStepMonths:              z.SplitStepMonths,
				SplitStepPlatformPct:         z.SplitStepPlatformPct,
				SplitStepEstablishmentPct:    z.SplitStepEstablishmentPct,
				SplitMinMonthlyOrders:        z.SplitMinMonthlyOrders,
				SplitMinActiveCouriers:       z.SplitMinActiveCouriers,
				SplitLastAdjustedAt:          z.SplitLastAdjustedAt,
				CreatedAt:                    z.CreatedAt,
			})
		}
		return result
	}

	// Inicia o job de decaimento
	splitDecayJob.Start(fetchSplitZones)

	log.Println("[DISPATCH] Engine initialized: courier store + matching engine + calibration job + split decay + retry loop")
}

// splitMetricsProvider implementa dispatchServices.ZoneMetricsProvider usando GORM.
type splitMetricsProvider struct {
	DB *gorm.DB
}

func (s *splitMetricsProvider) GetMonthlyOrders(zoneID uint) int {
	// Conta pedidos dos ÚLTIMOS 30 DIAS para estabelecimentos vinculados
	// a esta zona. Usa order_documents (tabela ativa desde o corte 5) que
	// possui created_at; a tabela legada "orders" não tem coluna temporal.
	if s.DB == nil {
		return 0
	}
	var count int64
	err := s.DB.Raw(
		"SELECT COUNT(*) FROM order_documents "+
			"JOIN establishments ON establishments.id = order_documents.establishment_id "+
			"WHERE establishments.zone_id = ? "+
			"AND order_documents.created_at >= NOW() - INTERVAL '30 days'", zoneID,
	).Scan(&count).Error
	if err != nil {
		// Coluna establishment_id pode nao existir ainda (migration pendente)
		log.Printf("[SPLIT_DECAY] GetMonthlyOrders fallback: %v", err)
		return 0
	}
	return int(count)
}

func (s *splitMetricsProvider) GetActiveCouriers(zoneID uint) int {
	// Conta entregadores com status 'available' ou 'busy' vinculados a esta zona
	if s.DB == nil {
		return 0
	}
	var count int64
	err := s.DB.Model(&models.DeliveryMan{}).
		Where("zone_id = ? AND status IN ('available', 'busy')", zoneID).
		Count(&count).Error
	if err != nil {
		log.Printf("[SPLIT_DECAY] GetActiveCouriers fallback: %v", err)
		return 0
	}
	return int(count)
}

// zoneDBResolver implementa dispatchServices.ZoneResolver usando GORM.
type zoneDBResolver struct {
	DB *gorm.DB
}

func (z *zoneDBResolver) ResolveByLatLng(lat, lng float64) (uint, string, float64, error) {
	if z.DB == nil {
		return 0, "Default", 10.0, nil
	}

	var zones []models.Zone
	if err := z.DB.Where("is_active = ?", true).Find(&zones).Error; err != nil || len(zones) == 0 {
		return 0, "Default", 10.0, nil
	}

	// Zona única ativa = deployment simples; mantém comportamento anterior.
	if len(zones) == 1 {
		z0 := zones[0]
		return z0.ID, z0.Name, z0.RadiusKm, nil
	}

	// Multi-zona: casa pelo prefixo geohash (match mais longo vence).
	point := encodeGeohash(lat, lng, 8)
	bestIdx, bestLen := -1, 0
	for i, zn := range zones {
		p := strings.ToLower(strings.TrimSpace(zn.GeohashPrefix))
		if p == "" || len(p) > len(point) {
			continue
		}
		if strings.HasPrefix(point, p) && len(p) > bestLen {
			bestIdx, bestLen = i, len(p)
		}
	}
	if bestIdx >= 0 {
		zb := zones[bestIdx]
		return zb.ID, zb.Name, zb.RadiusKm, nil
	}

	// Nenhuma zona casou geometricamente — cai na primeira ativa (comportamento
	// antigo), mas agora VISÍVEL: sem este log o desvio passaria despercebido.
	log.Printf("[ZONE] WARN: lat=%.5f,lng=%.5f não casou com geohash de zona alguma (%d ativas); usando %q",
		lat, lng, len(zones), zones[0].Name)
	zf := zones[0]
	return zf.ID, zf.Name, zf.RadiusKm, nil
}

// encodeGeohash codifica (lat,lng) na string geohash padrão (base32) com a
// precisão pedida. Implementação mínima sem dependência externa — suficiente
// para casar prefixes de zona (precisão 5 ≈ 4.9km, 6 ≈ 1.2km, 8 ≈ 38m).
func encodeGeohash(lat, lng float64, precision int) string {
	const base32 = "0123456789bcdefghjkmnpqrstuvwxyz"
	latMin, latMax := -90.0, 90.0
	lngMin, lngMax := -180.0, 180.0

	var hash []byte
	even := true
	bit := 0
	chIdx := 0

	for len(hash) < precision {
		if even {
			mid := (lngMin + lngMax) / 2
			if lng >= mid {
				chIdx = chIdx<<1 | 1
				lngMin = mid
			} else {
				chIdx <<= 1
				lngMax = mid
			}
		} else {
			mid := (latMin + latMax) / 2
			if lat >= mid {
				chIdx = chIdx<<1 | 1
				latMin = mid
			} else {
				chIdx <<= 1
				latMax = mid
			}
		}
		even = !even
		bit++
		if bit == 5 {
			hash = append(hash, base32[chIdx])
			bit = 0
			chIdx = 0
		}
	}
	return string(hash)
}

func (z *zoneDBResolver) GetDeliveryFee(zoneID uint, distanceKm float64) float64 {
	if z.DB == nil {
		return 5.0
	}
	var zone models.Zone
	if err := z.DB.First(&zone, zoneID).Error; err != nil {
		return 5.0
	}
	fee := zone.MinDeliveryFee
	if distanceKm > 3.0 {
		fee += (distanceKm - 3.0) * 1.5
	}
	return fee
}

func (z *zoneDBResolver) GetSurgeMultiplier(zoneID uint) float64 {
	if z.DB == nil {
		return 1.0
	}
	var zone models.Zone
	if err := z.DB.First(&zone, zoneID).Error; err != nil {
		return 1.0
	}
	return zone.SurgeMultiplier
}

func (z *zoneDBResolver) GetMinCouriersThreshold(zoneID uint) int {
	if z.DB == nil {
		return 3
	}
	var zone models.Zone
	if err := z.DB.First(&zone, zoneID).Error; err != nil {
		return 3
	}
	return zone.MinCouriersThreshold
}

func (z *zoneDBResolver) AllowsBatching(zoneID uint) bool {
	if z.DB == nil {
		return true
	}
	var zone models.Zone
	if err := z.DB.First(&zone, zoneID).Error; err != nil {
		return true
	}
	return zone.AllowBatching
}

func (z *zoneDBResolver) GetZoneMetadata(zoneID uint) *dispatchServices.ZoneMetadata {
	meta := &dispatchServices.ZoneMetadata{
		MinRadiusKm:          2.0,
		RadiusKm:             5.0,
		MaxRadiusKm:          15.0,
		PeakRadiusMultiplier: 0.7,
		PeakHourStart:        "11:00",
		PeakHourEnd:          "14:00",
		MinDeliveryFee:       5.0,
		SurgeMultiplier:      1.0,
		MinCouriersThreshold: 3,
		AllowBatching:        true,
	}

	if z.DB == nil || zoneID == 0 {
		return meta
	}

	var zone models.Zone
	if err := z.DB.First(&zone, zoneID).Error; err != nil {
		return meta
	}

	meta.ID = zone.ID
	meta.Name = zone.Name
	meta.MinRadiusKm = zone.MinRadiusKm
	meta.RadiusKm = zone.RadiusKm
	meta.MaxRadiusKm = zone.MaxRadiusKm
	meta.PeakRadiusMultiplier = zone.PeakRadiusMultiplier
	meta.PeakHourStart = zone.PeakHourStart
	meta.PeakHourEnd = zone.PeakHourEnd
	meta.CitySize = zone.CitySize
	meta.DensityCouriersPerKm2 = zone.DensityCouriersPerKm2
	meta.MinDeliveryFee = zone.MinDeliveryFee
	meta.SurgeMultiplier = zone.SurgeMultiplier
	meta.MinCouriersThreshold = zone.MinCouriersThreshold
	meta.AllowBatching = zone.AllowBatching

	return meta
}
