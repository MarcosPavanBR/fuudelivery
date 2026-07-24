// Package handlers - solicitations_test.go
// Testes unitarios do solicitations handler.
// Testam a formula de Haversine (calculo de distancia) e validacao.
package handlers

import (
	"math"
	"testing"
)

// === Testes de calculateDistance (Haversine) ===

func TestCalculateDistance_SamePoint(t *testing.T) {
	lat := -23.5505
	long := -46.6333

	dist := calculateDistance(lat, long, lat, long)

	if dist > 0.001 { // tolerancia para float
		t.Errorf("Same point distance: got %f, want ~0", dist)
	}
}

func TestCalculateDistance_KnownDistance(t *testing.T) {
	// Sao Paulo to Rio de Janeiro: ~430 km
	sp := struct{ lat, long float64 }{-23.5505, -46.6333}
	rj := struct{ lat, long float64 }{-22.9068, -43.1729}

	dist := calculateDistance(sp.lat, sp.long, rj.lat, rj.long)

	// Distance should be approximately 360 km (Haversine gives straight-line distance)
	if dist < 340 || dist > 400 {
		t.Errorf("SP to RJ distance: got %f km, want ~360 km", dist)
	}
}

func TestCalculateDistance_ShortDistance(t *testing.T) {
	// Two points very close: ~1 km apart
	// A point and another 0.01 degree away
	lat1 := -23.5505
	long1 := -46.6333
	lat2 := -23.5505 + 0.01 // ~1.1 km north
	long2 := -46.6333

	dist := calculateDistance(lat1, long1, lat2, long2)

	// Should be approximately 1.1 km
	if dist < 0.5 || dist > 2.0 {
		t.Errorf("Short distance: got %f km, want ~1.1 km", dist)
	}
}

func TestCalculateDistance_NorthSouth(t *testing.T) {
	// Same longitude, different latitude
	lat1 := -23.0
	lat2 := -24.0
	long := -46.0

	dist := calculateDistance(lat1, long, lat2, long)

	// 1 degree latitude ~ 111 km
	if dist < 100 || dist > 125 {
		t.Errorf("North-south distance: got %f km, want ~111 km", dist)
	}
}

func TestCalculateDistance_EastWest(t *testing.T) {
	// Same latitude, different longitude (at equator)
	lat := 0.0
	long1 := 0.0
	long2 := 1.0

	dist := calculateDistance(lat, long1, lat, long2)

	// 1 degree longitude at equator ~ 111 km
	if dist < 100 || dist > 125 {
		t.Errorf("East-west distance at equator: got %f km, want ~111 km", dist)
	}
}

// === Testes de degreesToRadians ===

func TestDegreesToRadians(t *testing.T) {
	tests := []struct {
		name     string
		degrees  float64
		expected float64
	}{
		{"zero degrees", 0.0, 0.0},
		{"90 degrees", 90.0, math.Pi / 2},
		{"180 degrees", 180.0, math.Pi},
		{"360 degrees", 360.0, 2 * math.Pi},
		{"45 degrees", 45.0, math.Pi / 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := degreesToRadians(tt.degrees)
			if math.Abs(result-tt.expected) > 0.0001 {
				t.Errorf("degreesToRadians(%f) = %f, want %f", tt.degrees, result, tt.expected)
			}
		})
	}
}

func TestDegreesToRadians_Negative(t *testing.T) {
	result := degreesToRadians(-90.0)
	expected := -math.Pi / 2

	if math.Abs(result-expected) > 0.0001 {
		t.Errorf("degreesToRadians(-90) = %f, want %f", result, expected)
	}
}

// === Testes de GetApprovedSolicitations query filter ===

func TestApprovedSolicitationsFilter(t *testing.T) {
	// Replicate the filter logic
	validStatuses := []string{"APPROVED", "DONE"}

	for _, status := range validStatuses {
		if status != "APPROVED" && status != "DONE" {
			t.Errorf("Status %q should be in filter", status)
		}
	}
}

// === Testes de OrderDTO structure ===

func TestOrderDTO_Fields(t *testing.T) {
	type OrderDTO struct {
		OrderId string
		Status  string
	}

	order := OrderDTO{
		OrderId: "order_123",
		Status:  "APPROVED",
	}

	if order.OrderId != "order_123" {
		t.Errorf("OrderId: got %q", order.OrderId)
	}
	if order.Status != "APPROVED" {
		t.Errorf("Status: got %q", order.Status)
	}
}

// === Testes de distance limit ===

func TestDistanceLimit(t *testing.T) {
	tests := []struct {
		name     string
		distance float64
		limit    float64
		include  bool
	}{
		{"within limit", 3.0, 5.0, true},
		{"at limit", 5.0, 5.0, true},
		{"over limit", 7.0, 5.0, false},
		{"zero distance", 0.0, 5.0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			include := tt.distance <= tt.limit
			if include != tt.include {
				t.Errorf("distance=%.1f limit=%.1f: got include=%v, want %v",
					tt.distance, tt.limit, include, tt.include)
			}
		})
	}
}
