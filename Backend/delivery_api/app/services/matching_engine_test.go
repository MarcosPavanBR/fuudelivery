package services

import (
	"fmt"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/carloshomar/vercardapio/delivery_api/app/dto"
)

// ============================================================================
// Mock ZoneResolver para testes do MatchingEngine
// ============================================================================

// mockZoneResolver implementa ZoneResolver com configuracao controlavel.
type mockZoneResolver struct {
	zoneMetadata map[uint]*ZoneMetadata
	defaultZone  *ZoneMetadata
	resolveFunc  func(lat, lng float64) (uint, string, float64, error)
}

func newMockZoneResolver() *mockZoneResolver {
	return &mockZoneResolver{
		zoneMetadata: make(map[uint]*ZoneMetadata),
		defaultZone: &ZoneMetadata{
			RadiusKm:              10.0,
			MaxRadiusKm:           15.0,
			MinRadiusKm:           2.0,
			PeakRadiusMultiplier:  0.7,
			PeakHourStart:         "",
			PeakHourEnd:           "",
			MinDeliveryFee:        5.0,
			SurgeMultiplier:       1.0,
			MinCouriersThreshold:  3,
			AllowBatching:         true,
			DensityCouriersPerKm2: 0,
		},
	}
}

func (m *mockZoneResolver) ResolveByLatLng(lat, lng float64) (uint, string, float64, error) {
	if m.resolveFunc != nil {
		return m.resolveFunc(lat, lng)
	}
	return 1, "Centro", 10.0, nil
}

func (m *mockZoneResolver) GetDeliveryFee(zoneID uint, distanceKm float64) float64 {
	return 5.0 + math.Max(0, distanceKm-3.0)*1.5
}

func (m *mockZoneResolver) GetSurgeMultiplier(zoneID uint) float64 {
	if z, ok := m.zoneMetadata[zoneID]; ok {
		return z.SurgeMultiplier
	}
	return 1.0
}

func (m *mockZoneResolver) GetMinCouriersThreshold(zoneID uint) int {
	if z, ok := m.zoneMetadata[zoneID]; ok {
		return z.MinCouriersThreshold
	}
	return m.defaultZone.MinCouriersThreshold
}

func (m *mockZoneResolver) AllowsBatching(zoneID uint) bool {
	if z, ok := m.zoneMetadata[zoneID]; ok {
		return z.AllowBatching
	}
	return m.defaultZone.AllowBatching
}

func (m *mockZoneResolver) GetZoneMetadata(zoneID uint) *ZoneMetadata {
	if z, ok := m.zoneMetadata[zoneID]; ok {
		return z
	}
	return m.defaultZone
}

// ============================================================================
// Testes de Haversine (haversineKm)
// ============================================================================

func TestHaversineKm_SamePoint(t *testing.T) {
	lat, lng := -23.5505, -46.6333
	dist := haversineKm(lat, lng, lat, lng)
	if dist > 0.001 {
		t.Errorf("Same point distance: got %f, want ~0", dist)
	}
}

func TestHaversineKm_KnownDistance(t *testing.T) {
	tests := []struct {
		name     string
		lat1     float64
		lng1     float64
		lat2     float64
		lng2     float64
		minKm    float64
		maxKm    float64
	}{
		{
			name:  "Sao Paulo to Rio de Janeiro",
			lat1:  -23.5505, lng1: -46.6333,
			lat2:  -22.9068, lng2: -43.1729,
			minKm: 340, maxKm: 400,
		},
		{
			name:  "1 degree latitude apart",
			lat1:  -23.0, lng1: -46.0,
			lat2:  -24.0, lng2: -46.0,
			minKm: 100, maxKm: 125,
		},
		{
			name:  "1 degree longitude at equator",
			lat1:  0.0, lng1: 0.0,
			lat2:  0.0, lng2: 1.0,
			minKm: 100, maxKm: 125,
		},
		{
			name:  "short distance ~1km",
			lat1:  -23.5505, lng1: -46.6333,
			lat2:  -23.5505 + 0.01, lng2: -46.6333,
			minKm: 0.5, maxKm: 2.0,
		},
		{
			name:  "antipodal points",
			lat1:  0.0, lng1: 0.0,
			lat2:  0.0, lng2: 180.0,
			minKm: 20000, maxKm: 20100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dist := haversineKm(tt.lat1, tt.lng1, tt.lat2, tt.lng2)
			if dist < tt.minKm || dist > tt.maxKm {
				t.Errorf("haversineKm: got %f km, want between %f and %f",
					dist, tt.minKm, tt.maxKm)
			}
		})
	}
}

func TestHaversineKm_Symmetry(t *testing.T) {
	a := haversineKm(-23.5505, -46.6333, -22.9068, -43.1729)
	b := haversineKm(-22.9068, -43.1729, -23.5505, -46.6333)
	if math.Abs(a-b) > 0.001 {
		t.Errorf("Haversine not symmetric: a=%f, b=%f", a, b)
	}
}

func TestHaversineKm_NegativeCoordinates(t *testing.T) {
	// Buenos Aires (-34.6037, -58.3816) to Montevideo (-34.9011, -56.1645)
	// Known distance ~210 km (straight line)
	dist := haversineKm(-34.6037, -58.3816, -34.9011, -56.1645)
	if dist < 180 || dist > 240 {
		t.Errorf("BA to Montevideo: got %f km, want ~210 km", dist)
	}
}

// ============================================================================
// Testes de Radius Stages (3-stage progressive search)
// ============================================================================

func TestZoneMetadata_GetRadiusStages(t *testing.T) {
	tests := []struct {
		name            string
		minRadius       float64
		radius          float64
		maxRadius       float64
		expectedStages  [3]float64
	}{
		{
			name:      "standard zone",
			minRadius: 2.0,
			radius:    5.0,
			maxRadius: 10.0,
			// stage1=5.0, stage2=min(8.5,10)=8.5, stage3=10
			expectedStages: [3]float64{5.0, 8.5, 10.0},
		},
		{
			name:      "stage2 clamped to maxRadius",
			minRadius: 2.0,
			radius:    8.0,
			maxRadius: 10.0,
			// stage1=8.0, stage2=min(13.6,10)=10, stage3=10
			expectedStages: [3]float64{8.0, 10.0, 10.0},
		},
		{
			name:      "tiny zone",
			minRadius: 0.5,
			radius:    1.0,
			maxRadius: 2.0,
			// stage1=1.0, stage2=min(1.7,2)=1.7, stage3=2
			expectedStages: [3]float64{1.0, 1.7, 2.0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			z := &ZoneMetadata{
				MinRadiusKm: tt.minRadius,
				RadiusKm:    tt.radius,
				MaxRadiusKm: tt.maxRadius,
			}
			stages := z.GetRadiusStages()
			for i, expected := range tt.expectedStages {
				if math.Abs(stages[i]-expected) > 0.01 {
					t.Errorf("stage[%d]: got %f, want %f", i, stages[i], expected)
				}
			}
		})
	}
}

func TestZoneMetadata_GetRadiusStages_NonPeak(t *testing.T) {
	// Outside peak hours, base radius should be RadiusKm
	z := &ZoneMetadata{
		MinRadiusKm:          2.0,
		RadiusKm:             5.0,
		MaxRadiusKm:          10.0,
		PeakRadiusMultiplier: 0.5,
		PeakHourStart:        "",
		PeakHourEnd:          "",
	}

	stages := z.GetRadiusStages()
	// Without peak hours, effective radius = 5.0
	if stages[0] != 5.0 {
		t.Errorf("stage[0] outside peak: got %f, want 5.0", stages[0])
	}
}

func TestZoneMetadata_GetEffectiveRadius_PeakHour(t *testing.T) {
	// We need to test with peak hours that match current time
	now := time.Now()
	currentHour := now.Format("15:04")

	z := &ZoneMetadata{
		MinRadiusKm:          2.0,
		RadiusKm:             10.0,
		MaxRadiusKm:          15.0,
		PeakRadiusMultiplier: 0.5,
		PeakHourStart:        currentHour,
		PeakHourEnd:          currentHour,
	}

	radius := z.GetEffectiveRadius()
	expected := 10.0 * 0.5 // 5.0
	if radius != expected {
		t.Errorf("GetEffectiveRadius during peak: got %f, want %f", radius, expected)
	}
}

func TestZoneMetadata_GetEffectiveRadius_PeakClampedToMin(t *testing.T) {
	now := time.Now()
	currentHour := now.Format("15:04")

	z := &ZoneMetadata{
		MinRadiusKm:          3.0,
		RadiusKm:             5.0,
		MaxRadiusKm:          10.0,
		PeakRadiusMultiplier: 0.3,
		PeakHourStart:        currentHour,
		PeakHourEnd:          currentHour,
	}

	radius := z.GetEffectiveRadius()
	// 5.0 * 0.3 = 1.5, but min is 3.0
	if radius != 3.0 {
		t.Errorf("GetEffectiveRadius peak clamped: got %f, want 3.0", radius)
	}
}

// ============================================================================
// Testes de Poisson Density Calculation
// ============================================================================

func TestZoneMetadata_GetSuggestedRadiusByDensity(t *testing.T) {
	tests := []struct {
		name            string
		density         float64
		minRadius       float64
		maxRadius       float64
		targetCouriers  int
		expectedMin     float64
		expectedMax     float64
	}{
		{
			name:           "zero density returns effective radius",
			density:        0,
			minRadius:      2.0,
			maxRadius:      15.0,
			targetCouriers: 3,
			expectedMin:    9.0,
			expectedMax:    11.0,
		},
		{
			name:           "high density suggests smaller radius",
			density:        5.0,
			minRadius:      1.0,
			maxRadius:      15.0,
			targetCouriers: 3,
			// sqrt(3 / (pi * 5)) = sqrt(0.191) = 0.437 -> clamped to min=1.0
			expectedMin: 0.9,
			expectedMax: 1.1,
		},
		{
			name:           "low density suggests larger radius",
			density:        0.01,
			minRadius:      1.0,
			maxRadius:      15.0,
			targetCouriers: 3,
			// sqrt(3 / (pi * 0.01)) = sqrt(95.49) = 9.77
			expectedMin: 9.0,
			expectedMax: 10.5,
		},
		{
			name:           "clamped to maxRadius",
			density:        0.001,
			minRadius:      1.0,
			maxRadius:      5.0,
			targetCouriers: 10,
			// sqrt(10 / (pi * 0.001)) = sqrt(3183) = 56.4 -> clamped to 5.0
			expectedMin: 4.9,
			expectedMax: 5.1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			z := &ZoneMetadata{
				MinRadiusKm:          tt.minRadius,
				RadiusKm:             10.0,
				MaxRadiusKm:          tt.maxRadius,
				DensityCouriersPerKm2: tt.density,
			}
			result := z.GetSuggestedRadiusByDensity(tt.targetCouriers)
			if result < tt.expectedMin || result > tt.expectedMax {
				t.Errorf("GetSuggestedRadiusByDensity(density=%.2f, target=%d): got %f, want [%f, %f]",
					tt.density, tt.targetCouriers, result, tt.expectedMin, tt.expectedMax)
			}
		})
	}
}

// ============================================================================
// Testes de DLQStore
// ============================================================================

func TestDLQStore_PushAndLen(t *testing.T) {
	dlq := NewDLQStore(5)

	dlq.Push(&UnmatchedOrder{OrderID: "order-1", RetryCount: 0, LastAttemptAt: 0})
	dlq.Push(&UnmatchedOrder{OrderID: "order-2", RetryCount: 0, LastAttemptAt: 0})

	if dlq.Len() != 2 {
		t.Errorf("DLQ Len: got %d, want 2", dlq.Len())
	}
}

func TestDLQStore_MaxSizeEviction(t *testing.T) {
	dlq := NewDLQStore(3)

	for i := 0; i < 5; i++ {
		dlq.Push(&UnmatchedOrder{
			OrderID:    "order-" + string(rune('0'+i)),
			RetryCount: 0,
		})
	}

	// Max 3, so first 2 should be evicted
	if dlq.Len() != 3 {
		t.Errorf("DLQ Len after overflow: got %d, want 3", dlq.Len())
	}
}

func TestDLQStore_PopNext_Empty(t *testing.T) {
	dlq := NewDLQStore(10)
	result := dlq.PopNext()
	if result != nil {
		t.Errorf("PopNext on empty DLQ: got %v, want nil", result)
	}
}

func TestDLQStore_PopNext_MaxRetriesExceeded(t *testing.T) {
	dlq := NewDLQStore(10)
	dlq.Push(&UnmatchedOrder{
		OrderID:    "order-exhausted",
		RetryCount: 3, // max retries
		LastAttemptAt: 0,
	})

	result := dlq.PopNext()
	if result != nil {
		t.Errorf("PopNext should skip exhausted orders, got %v", result)
	}
	if dlq.Len() != 1 {
		t.Errorf("DLQ should still have exhausted order: Len=%d, want 1", dlq.Len())
	}
}

func TestDLQStore_PopNext_RecentAttempt(t *testing.T) {
	dlq := NewDLQStore(10)
	now := time.Now().UnixMilli()
	dlq.Push(&UnmatchedOrder{
		OrderID:       "order-recent",
		RetryCount:    0,
		LastAttemptAt: now, // just attempted
	})

	result := dlq.PopNext()
	// Should skip because last attempt was < 30s ago
	if result != nil {
		t.Errorf("PopNext should skip recently attempted orders, got %v", result)
	}
}

func TestDLQStore_PopNext_ReadyForRetry(t *testing.T) {
	dlq := NewDLQStore(10)
	oldTime := time.Now().UnixMilli() - 60000 // 60s ago
	dlq.Push(&UnmatchedOrder{
		OrderID:       "order-old",
		RetryCount:    1,
		LastAttemptAt: oldTime,
	})

	result := dlq.PopNext()
	if result == nil {
		t.Fatal("PopNext should return old order")
	}
	if result.OrderID != "order-old" {
		t.Errorf("PopNext wrong order: got %s, want order-old", result.OrderID)
	}
	if dlq.Len() != 0 {
		t.Errorf("DLQ should be empty after PopNext: Len=%d", dlq.Len())
	}
}

func TestDLQStore_List(t *testing.T) {
	dlq := NewDLQStore(10)
	dlq.Push(&UnmatchedOrder{OrderID: "o1"})
	dlq.Push(&UnmatchedOrder{OrderID: "o2"})

	list := dlq.List()
	if len(list) != 2 {
		t.Errorf("List len: got %d, want 2", len(list))
	}
	// Verify list is a copy, not a reference
	dlq.Push(&UnmatchedOrder{OrderID: "o3"})
	if len(list) != 2 {
		t.Errorf("List should be independent copy: len changed to %d after push", len(list))
	}
}

// ============================================================================
// Testes do MatchingEngine com MockZoneResolver
// ============================================================================

func TestMatchingEngine_AttemptMatch_NoCouriers(t *testing.T) {
	store := NewCourierStore()
	resolver := newMockZoneResolver()
	engine := NewMatchingEngine(store, resolver)

	order := &dto.OrderDTO{
		OrderId: "order-no-couriers",
		Establishment: dto.EstablishmentDTO{
			Lat:  -23.5505,
			Long: -46.6333,
		},
	}

	result := engine.AttemptMatch(order)

	if result.Matched {
		t.Error("Expected no match with empty store")
	}
	if result.StageReached != 0 {
		t.Errorf("StageReached: got %d, want 0", result.StageReached)
	}
	if result.CourierID != 0 {
		t.Errorf("CourierID should be 0, got %d", result.CourierID)
	}
}

func TestMatchingEngine_AttemptMatch_WithAvailableCourier(t *testing.T) {
	store := NewCourierStore()
	resolver := newMockZoneResolver()
	engine := NewMatchingEngine(store, resolver)

	// Add a courier very close to the order
	store.UpdateLocation(1, "Alice", -23.5505, -46.6333, "available")

	order := &dto.OrderDTO{
		OrderId: "order-match",
		Establishment: dto.EstablishmentDTO{
			Lat:  -23.5505,
			Long: -46.6333,
		},
	}

	result := engine.AttemptMatch(order)

	if !result.Matched {
		t.Fatal("Expected match with available courier")
	}
	if result.CourierID != 1 {
		t.Errorf("CourierID: got %d, want 1", result.CourierID)
	}
	if result.CourierName != "Alice" {
		t.Errorf("CourierName: got %s, want Alice", result.CourierName)
	}
	if result.DistanceKm > 0.1 {
		t.Errorf("DistanceKm too large: got %f, want ~0", result.DistanceKm)
	}
}

func TestMatchingEngine_AttemptMatch_StageProgression(t *testing.T) {
	store := NewCourierStore()
	resolver := newMockZoneResolver()

	// Configure zone with small initial radius but large max
	zoneMeta := &ZoneMetadata{
		MinRadiusKm:          1.0,
		RadiusKm:             2.0,     // stage1 = 2km
		MaxRadiusKm:          10.0,    // stage3 = 10km
		PeakRadiusMultiplier: 1.0,
		MinCouriersThreshold: 3,
		DensityCouriersPerKm2: 0,
	}
	resolver.zoneMetadata[1] = zoneMeta

	engine := NewMatchingEngine(store, resolver)

	// Place courier at ~4km away (only reachable at stage2 with 1.7x = 3.4km, or stage3)
	// 0.04 degrees lat ~= 4.4km
	store.UpdateLocation(1, "FarCourier", -23.5105, -46.6333, "available")

	order := &dto.OrderDTO{
		OrderId: "order-stage",
		Establishment: dto.EstablishmentDTO{
			Lat:  -23.5505,
			Long: -46.6333,
		},
	}

	result := engine.AttemptMatch(order)

	if !result.Matched {
		t.Fatal("Expected match at stage 3")
	}
	if result.StageReached != 3 {
		t.Errorf("StageReached: got %d, want 3", result.StageReached)
	}
}

func TestMatchingEngine_AttemptMatch_BusyCourierSkipped(t *testing.T) {
	store := NewCourierStore()
	resolver := newMockZoneResolver()
	engine := NewMatchingEngine(store, resolver)

	// Courier is busy - should not be matched
	store.UpdateLocation(1, "BusyCourier", -23.5505, -46.6333, "busy")

	order := &dto.OrderDTO{
		OrderId: "order-busy",
		Establishment: dto.EstablishmentDTO{
			Lat:  -23.5505,
			Long: -46.6333,
		},
	}

	result := engine.AttemptMatch(order)

	if result.Matched {
		t.Error("Should not match a busy courier")
	}
}

func TestMatchingEngine_AttemptMatch_FullCapacitySkipped(t *testing.T) {
	store := NewCourierStore()
	resolver := newMockZoneResolver()
	engine := NewMatchingEngine(store, resolver)

	// Courier at max capacity
	store.UpdateLocation(1, "FullCourier", -23.5505, -46.6333, "available")
	store.SetOrdersCount(1, 3) // MaxOrders = 3

	order := &dto.OrderDTO{
		OrderId: "order-full",
		Establishment: dto.EstablishmentDTO{
			Lat:  -23.5505,
			Long: -46.6333,
		},
	}

	result := engine.AttemptMatch(order)

	if result.Matched {
		t.Error("Should not match a courier at full capacity")
	}
}

func TestMatchingEngine_AttemptMatch_OrdersCountIncremented(t *testing.T) {
	store := NewCourierStore()
	resolver := newMockZoneResolver()
	engine := NewMatchingEngine(store, resolver)

	store.UpdateLocation(1, "Alice", -23.5505, -46.6333, "available")

	order := &dto.OrderDTO{
		OrderId: "order-increment",
		Establishment: dto.EstablishmentDTO{
			Lat:  -23.5505,
			Long: -46.6333,
		},
	}

	engine.AttemptMatch(order)

	c := store.GetCourier(1)
	if c == nil {
		t.Fatal("Courier should still exist")
	}
	if c.CurrentOrders != 1 {
		t.Errorf("CurrentOrders: got %d, want 1 after match", c.CurrentOrders)
	}
}

func TestMatchingEngine_AttemptMatch_Callbacks(t *testing.T) {
	store := NewCourierStore()
	resolver := newMockZoneResolver()
	engine := NewMatchingEngine(store, resolver)

	store.UpdateLocation(1, "Alice", -23.5505, -46.6333, "available")

	var matchedOrderID string
	var matchedCourierID int64
	engine.OnMatch = func(orderID string, courierID int64) {
		matchedOrderID = orderID
		matchedCourierID = courierID
	}

	order := &dto.OrderDTO{
		OrderId: "order-callback",
		Establishment: dto.EstablishmentDTO{
			Lat:  -23.5505,
			Long: -46.6333,
		},
	}

	engine.AttemptMatch(order)

	if matchedOrderID != "order-callback" {
		t.Errorf("OnMatch orderID: got %s, want order-callback", matchedOrderID)
	}
	if matchedCourierID != 1 {
		t.Errorf("OnMatch courierID: got %d, want 1", matchedCourierID)
	}
}

func TestMatchingEngine_AttemptMatch_FallbackCallback(t *testing.T) {
	store := NewCourierStore()
	resolver := newMockZoneResolver()

	// Set high threshold so fallback triggers
	zoneMeta := &ZoneMetadata{
		MinRadiusKm:          1.0,
		RadiusKm:             2.0,
		MaxRadiusKm:          5.0,
		MinCouriersThreshold: 10, // very high threshold
		DensityCouriersPerKm2: 0,
	}
	resolver.zoneMetadata[1] = zoneMeta

	engine := NewMatchingEngine(store, resolver)
	// No couriers at all, threshold is 10

	var fallbackOrderID string
	var fallbackZoneName string
	engine.OnFallback = func(orderID, zoneName string) {
		fallbackOrderID = orderID
		fallbackZoneName = zoneName
	}

	order := &dto.OrderDTO{
		OrderId: "order-fallback",
		Establishment: dto.EstablishmentDTO{
			Lat:  -23.5505,
			Long: -46.6333,
		},
	}

	result := engine.AttemptMatch(order)

	if result.Matched {
		t.Error("Should not match with no couriers")
	}
	if !result.Fallback {
		t.Error("Expected fallback=true when no couriers available")
	}
	if fallbackOrderID != "order-fallback" {
		t.Errorf("Fallback callback orderID: got %s, want order-fallback", fallbackOrderID)
	}
	if fallbackZoneName != "Centro" {
		t.Errorf("Fallback callback zoneName: got %s, want Centro", fallbackZoneName)
	}
}

func TestMatchingEngine_AttemptMatch_NoFallbackWhenEnoughCouriers(t *testing.T) {
	store := NewCourierStore()
	resolver := newMockZoneResolver()
	engine := NewMatchingEngine(store, resolver)

	// Default threshold is 3. Add 5 far-away couriers (within max radius but
	// none close enough to match). CountAvailable counts them.
	for i := int64(1); i <= 5; i++ {
		store.UpdateLocation(i, "Far", -23.4505, -46.6333, "available")
	}

	fallbackCalled := false
	engine.OnFallback = func(orderID, zoneName string) {
		fallbackCalled = true
	}

	order := &dto.OrderDTO{
		OrderId: "order-no-fallback",
		Establishment: dto.EstablishmentDTO{
			Lat:  -23.5505,
			Long: -46.6333,
		},
	}

	result := engine.AttemptMatch(order)
	if result.Fallback {
		t.Error("Should not trigger fallback when enough couriers available")
	}
	if fallbackCalled {
		t.Error("Fallback callback should not be called")
	}
}

func TestMatchingEngine_AttemptMatch_DLQFilled(t *testing.T) {
	store := NewCourierStore()
	resolver := newMockZoneResolver()
	engine := NewMatchingEngine(store, resolver)

	order := &dto.OrderDTO{
		OrderId: "order-dlq",
		Establishment: dto.EstablishmentDTO{
			Lat:  -23.5505,
			Long: -46.6333,
		},
	}

	engine.AttemptMatch(order)

	if engine.DLQ.Len() != 1 {
		t.Errorf("DLQ Len: got %d, want 1", engine.DLQ.Len())
	}

	orders := engine.DLQ.List()
	if orders[0].OrderID != "order-dlq" {
		t.Errorf("DLQ order: got %s, want order-dlq", orders[0].OrderID)
	}
}

// ============================================================================
// Testes de CheckBatching
// ============================================================================

func TestMatchingEngine_CheckBatching(t *testing.T) {
	tests := []struct {
		name          string
		existLat      float64
		existLng      float64
		newLat        float64
		newLng        float64
		restLat       float64
		restLng       float64
		expectedMatch bool
	}{
		{
			name:          "close new order, acceptable detour",
			existLat:      -23.5505, existLng: -46.6333,
			newLat:        -23.5515, newLng: -46.6343, // ~0.15km from restaurant
			restLat:       -23.5510, restLng: -46.6338,
			expectedMatch: true,
		},
		{
			name:          "new order too far from existing",
			existLat:      -23.5505, existLng: -46.6333,
			newLat:        -23.6000, newLng: -46.7000, // ~8km away
			restLat:       -23.5510, restLng: -46.6338,
			expectedMatch: false,
		},
		{
			name:          "same location as existing",
			existLat:      -23.5505, existLng: -46.6333,
			newLat:        -23.5505, newLng: -46.6333,
			restLat:       -23.5510, restLng: -46.6338,
			expectedMatch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := &MatchingEngine{}
			result := engine.CheckBatching(
				tt.existLat, tt.existLng,
				tt.newLat, tt.newLng,
				tt.restLat, tt.restLng,
			)
			if result != tt.expectedMatch {
				t.Errorf("CheckBatching: got %v, want %v", result, tt.expectedMatch)
			}
		})
	}
}

// ============================================================================
// Testes de Metricas
// ============================================================================

func TestMatchingEngine_GetUnmatchedRate_NoOrders(t *testing.T) {
	engine := NewMatchingEngine(NewCourierStore(), newMockZoneResolver())
	rate := engine.GetUnmatchedRate()
	if rate != 0 {
		t.Errorf("GetUnmatchedRate with no orders: got %f, want 0", rate)
	}
}

func TestMatchingEngine_GetUnmatchedRate_AllMatched(t *testing.T) {
	store := NewCourierStore()
	resolver := newMockZoneResolver()
	engine := NewMatchingEngine(store, resolver)

	// Add enough couriers to match multiple orders
	for i := int64(1); i <= 10; i++ {
		store.UpdateLocation(i, "Courier", -23.5505, -46.6333, "available")
	}

	for i := 0; i < 5; i++ {
		order := &dto.OrderDTO{
			OrderId: "order-rate-" + string(rune('A'+i)),
			Establishment: dto.EstablishmentDTO{
				Lat:  -23.5505,
				Long: -46.6333,
			},
		}
		engine.AttemptMatch(order)
	}

	rate := engine.GetUnmatchedRate()
	if rate != 0 {
		t.Errorf("GetUnmatchedRate all matched: got %f, want 0", rate)
	}
}

func TestMatchingEngine_GetUnmatchedRate_AllUnmatched(t *testing.T) {
	store := NewCourierStore()
	resolver := newMockZoneResolver()
	engine := NewMatchingEngine(store, resolver)
	// Empty store -> all unmatched

	for i := 0; i < 3; i++ {
		order := &dto.OrderDTO{
			OrderId: "order-unmatched-" + string(rune('A'+i)),
			Establishment: dto.EstablishmentDTO{
				Lat:  -23.5505,
				Long: -46.6333,
			},
		}
		engine.AttemptMatch(order)
	}

	rate := engine.GetUnmatchedRate()
	if rate != 1.0 {
		t.Errorf("GetUnmatchedRate all unmatched: got %f, want 1.0", rate)
	}
}

func TestMatchingEngine_GetMatchTimeP90_Empty(t *testing.T) {
	engine := NewMatchingEngine(NewCourierStore(), newMockZoneResolver())
	p90 := engine.GetMatchTimeP90()
	if p90 != 0 {
		t.Errorf("GetMatchTimeP90 empty: got %f, want 0", p90)
	}
}

func TestMatchingEngine_GetTotalOrders(t *testing.T) {
	store := NewCourierStore()
	resolver := newMockZoneResolver()
	engine := NewMatchingEngine(store, resolver)

	for i := int64(1); i <= 5; i++ {
		store.UpdateLocation(i, "C", -23.5505, -46.6333, "available")
	}

	for i := 0; i < 3; i++ {
		order := &dto.OrderDTO{
			OrderId: "o" + string(rune('A'+i)),
			Establishment: dto.EstablishmentDTO{
				Lat:  -23.5505,
				Long: -46.6333,
			},
		}
		engine.AttemptMatch(order)
	}

	total := engine.GetTotalOrders()
	if total != 3 {
		t.Errorf("GetTotalOrders: got %d, want 3", total)
	}
}

// ============================================================================
// Testes de RetryUnmatched
// ============================================================================

func TestMatchingEngine_RetryUnmatched_EmptyDLQ(t *testing.T) {
	store := NewCourierStore()
	resolver := newMockZoneResolver()
	engine := NewMatchingEngine(store, resolver)

	retried := engine.RetryUnmatched()
	if retried != 0 {
		t.Errorf("RetryUnmatched empty DLQ: got %d, want 0", retried)
	}
}

func TestMatchingEngine_RetryUnmatched_MatchFound(t *testing.T) {
	store := NewCourierStore()
	resolver := newMockZoneResolver()
	engine := NewMatchingEngine(store, resolver)

	// Push an order directly into DLQ with an old LastAttemptAt so PopNext accepts it
	// (PopNext requires >= 30s since last attempt)
	oldTime := time.Now().UnixMilli() - 60000
	engine.DLQ.Push(&UnmatchedOrder{
		OrderID:          "order-retry",
		EstablishmentLat: -23.5505,
		EstablishmentLng: -46.6333,
		RetryCount:       0,
		LastAttemptAt:    oldTime,
	})
	if engine.DLQ.Len() != 1 {
		t.Fatal("Expected 1 order in DLQ")
	}

	// Now add a courier
	store.UpdateLocation(1, "NewCourier", -23.5505, -46.6333, "available")

	// Retry should match
	retried := engine.RetryUnmatched()
	if retried != 1 {
		t.Errorf("RetryUnmatched: got %d, want 1", retried)
	}
	if engine.DLQ.Len() != 0 {
		t.Errorf("DLQ should be empty after retry match: Len=%d", engine.DLQ.Len())
	}
}

func TestMatchingEngine_RetryUnmatched_StillNoCourier(t *testing.T) {
	store := NewCourierStore()
	resolver := newMockZoneResolver()
	engine := NewMatchingEngine(store, resolver)

	// Add order to DLQ with old LastAttemptAt
	engine.DLQ.Push(&UnmatchedOrder{
		OrderID:       "order-no-retry",
		RetryCount:    2,
		LastAttemptAt: time.Now().UnixMilli() - 60000,
	})

	// Retry again -- still no couriers, retryCount becomes 3, stays in DLQ
	// Actually PopNext skips retryCount >= 3. This one has 2, so it should pop.
	retried := engine.RetryUnmatched()
	if retried != 0 {
		t.Errorf("RetryUnmatched no couriers: got %d, want 0", retried)
	}
}

// ============================================================================
// Testes de DefaultZoneResolver
// ============================================================================

func TestDefaultZoneResolver_AllMethods(t *testing.T) {
	dzr := &DefaultZoneResolver{}

	zoneID, zoneName, radius, err := dzr.ResolveByLatLng(-23.5505, -46.6333)
	if err != nil {
		t.Errorf("ResolveByLatLng error: %v", err)
	}
	if zoneID != 0 {
		t.Errorf("ResolveByLatLng zoneID: got %d, want 0", zoneID)
	}
	if zoneName != "Default" {
		t.Errorf("ResolveByLatLng zoneName: got %s, want Default", zoneName)
	}
	if radius != 10.0 {
		t.Errorf("ResolveByLatLng radius: got %f, want 10.0", radius)
	}

	fee := dzr.GetDeliveryFee(0, 5.0)
	if fee < 5.0 {
		t.Errorf("GetDeliveryFee: got %f, want >= 5.0", fee)
	}

	surge := dzr.GetSurgeMultiplier(0)
	if surge != 1.0 {
		t.Errorf("GetSurgeMultiplier: got %f, want 1.0", surge)
	}

	minC := dzr.GetMinCouriersThreshold(0)
	if minC != 3 {
		t.Errorf("GetMinCouriersThreshold: got %d, want 3", minC)
	}

	if !dzr.AllowsBatching(0) {
		t.Error("AllowsBatching should be true")
	}

	meta := dzr.GetZoneMetadata(0)
	if meta == nil {
		t.Fatal("GetZoneMetadata should not return nil")
	}
	if meta.RadiusKm != 10.0 {
		t.Errorf("GetZoneMetadata RadiusKm: got %f, want 10.0", meta.RadiusKm)
	}
}

// ============================================================================
// Testes de MatchResult.ToJSON
// ============================================================================

func TestMatchResult_ToJSON(t *testing.T) {
	mr := &MatchResult{
		Matched:      true,
		CourierID:    42,
		CourierName:  "Alice",
		DistanceKm:   2.5,
		StageReached: 2,
	}

	data := mr.ToJSON()
	if len(data) == 0 {
		t.Error("ToJSON should return non-empty data")
	}

	// Verify it contains expected fields
	json := string(data)
	for _, expected := range []string{"\"matched\":true", "\"courier_id\":42", "\"courier_name\":\"Alice\""} {
		if !contains(json, expected) {
			t.Errorf("ToJSON missing field: %s in %s", expected, json)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// ============================================================================
// Testes de Concurrent Access
// ============================================================================

func TestMatchingEngine_ConcurrentAttempts(t *testing.T) {
	store := NewCourierStore()
	resolver := newMockZoneResolver()
	engine := NewMatchingEngine(store, resolver)

	// Add couriers
	for i := int64(1); i <= 5; i++ {
		store.UpdateLocation(i, "Courier", -23.5505, -46.6333, "available")
	}

	var wg sync.WaitGroup
	results := make([]*MatchResult, 20)

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			order := &dto.OrderDTO{
				OrderId: "concurrent-order-" + string(rune('A'+idx)),
				Establishment: dto.EstablishmentDTO{
					Lat:  -23.5505,
					Long: -46.6333,
				},
			}
			results[idx] = engine.AttemptMatch(order)
		}(i)
	}

	wg.Wait()

	// No panics, no data races (run with -race), all results should be valid
	for i, r := range results {
		if r == nil {
			t.Errorf("Result %d is nil", i)
		}
	}
}

// ============================================================================
// Edge Cases
// ============================================================================

func TestMatchingEngine_AttemptMatch_ZeroRadiusZone(t *testing.T) {
	store := NewCourierStore()
	resolver := newMockZoneResolver()
	zoneMeta := &ZoneMetadata{
		MinRadiusKm:   0,
		RadiusKm:      0,
		MaxRadiusKm:   0,
		MinCouriersThreshold: 3,
		DensityCouriersPerKm2: 0,
	}
	resolver.zoneMetadata[1] = zoneMeta

	engine := NewMatchingEngine(store, resolver)
	store.UpdateLocation(1, "Alice", -23.5505, -46.6333, "available")

	order := &dto.OrderDTO{
		OrderId: "order-zero-radius",
		Establishment: dto.EstablishmentDTO{
			Lat:  -23.5505,
			Long: -46.6333,
		},
	}

	result := engine.AttemptMatch(order)
	// With zero radius stages, no candidates should be found
	if result.Matched {
		t.Error("Should not match with zero radius zone")
	}
}

func TestMatchingEngine_AttemptMatch_ExtremeDistance(t *testing.T) {
	store := NewCourierStore()
	resolver := newMockZoneResolver()
	engine := NewMatchingEngine(store, resolver)

	// Courier in a completely different continent
	store.UpdateLocation(1, "FarAway", 51.5074, -0.1278, "available") // London

	order := &dto.OrderDTO{
		OrderId: "order-faraway",
		Establishment: dto.EstablishmentDTO{
			Lat:  -23.5505,
			Long: -46.6333,
		},
	}

	result := engine.AttemptMatch(order)
	if result.Matched {
		t.Error("Should not match courier in another continent")
	}
}

func TestMatchingEngine_AttemptMatch_ZoneResolverError(t *testing.T) {
	store := NewCourierStore()
	resolver := newMockZoneResolver()
	resolver.resolveFunc = func(lat, lng float64) (uint, string, float64, error) {
		return 0, "", 0, fmt.Errorf("zone resolution failed")
	}

	engine := NewMatchingEngine(store, resolver)
	store.UpdateLocation(1, "Alice", -23.5505, -46.6333, "available")

	order := &dto.OrderDTO{
		OrderId: "order-resolve-error",
		Establishment: dto.EstablishmentDTO{
			Lat:  -23.5505,
			Long: -46.6333,
		},
	}

	// Should still work with default zone metadata
	result := engine.AttemptMatch(order)
	if !result.Matched {
		t.Error("Should still match with default zone on resolver error")
	}
}

func TestMatchingEngine_AttemptMatch_MultipleCandidatesPicksBest(t *testing.T) {
	store := NewCourierStore()
	resolver := newMockZoneResolver()
	engine := NewMatchingEngine(store, resolver)

	// Alice is very close, Bob is farther
	store.UpdateLocation(1, "Alice", -23.5505, -46.6333, "available")  // same spot
	store.UpdateLocation(2, "Bob", -23.5520, -46.6350, "available")    // ~0.2km away
	store.UpdateLocation(3, "Charlie", -23.5600, -46.6400, "available") // ~1km away

	order := &dto.OrderDTO{
		OrderId: "order-best",
		Establishment: dto.EstablishmentDTO{
			Lat:  -23.5505,
			Long: -46.6333,
		},
	}

	result := engine.AttemptMatch(order)

	if !result.Matched {
		t.Fatal("Expected a match")
	}
	if result.CourierID != 1 {
		t.Errorf("Expected closest courier (Alice, ID=1), got ID=%d (%s)",
			result.CourierID, result.CourierName)
	}
}

func TestMatchingEngine_DensityOverridesStage1(t *testing.T) {
	store := NewCourierStore()
	resolver := newMockZoneResolver()

	// Zone with small base radius but high density should expand stage1
	zoneMeta := &ZoneMetadata{
		MinRadiusKm:           1.0,
		RadiusKm:              2.0,     // base radius = 2km
		MaxRadiusKm:           20.0,
		PeakRadiusMultiplier:  1.0,
		MinCouriersThreshold:  3,
		DensityCouriersPerKm2: 0.01,    // low density -> big suggested radius
	}
	resolver.zoneMetadata[1] = zoneMeta

	// Put a courier at ~8km away (within Poisson-suggested radius)
	store.UpdateLocation(1, "MediumDistance", -23.5505+0.07, -46.6333, "available")

	engine := NewMatchingEngine(store, resolver)

	order := &dto.OrderDTO{
		OrderId: "order-density",
		Establishment: dto.EstablishmentDTO{
			Lat:  -23.5505,
			Long: -46.6333,
		},
	}

	result := engine.AttemptMatch(order)
	// The Poisson density radius should be used as stage1 if larger than base
	// sqrt(3 / (pi * 0.01)) = ~9.77km, so the courier at ~7.8km should be found at stage1
	if !result.Matched {
		t.Error("Expected match using density-expanded radius")
	}
}
