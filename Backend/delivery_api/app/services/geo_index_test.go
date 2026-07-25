package services

import (
	"testing"
)

// mockRedisClient simula um cliente Redis para testes sem dependencia externa.
// Como go-redis usa interfaces, podemos testar a logica de parse e formatacao
// das funcoes auxiliares, e usar um mini-stub para metodos principais.
//
// Testes de integracao com Redis real devem ser feitos com testcontainers
// ou contra uma instancia local.

func TestFormatCourierKey(t *testing.T) {
	tests := []struct {
		input    int64
		expected string
	}{
		{1, "courier:1"},
		{123, "courier:123"},
		{0, "courier:0"},
		{-1, "courier:-1"},
	}

	for _, tt := range tests {
		result := formatCourierKey(tt.input)
		if result != tt.expected {
			t.Errorf("formatCourierKey(%d) = %s; want %s", tt.input, result, tt.expected)
		}
	}
}

func TestParseCourierKey(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"courier:1", 1},
		{"courier:123", 123},
		{"courier:0", 0},
		{"courier:", 0},
		{"invalid", 0},
		{"", 0},
	}

	for _, tt := range tests {
		result := parseCourierKey(tt.input)
		if result != tt.expected {
			t.Errorf("parseCourierKey(%q) = %d; want %d", tt.input, result, tt.expected)
		}
	}
}

func TestFormatUintKey(t *testing.T) {
	tests := []struct {
		input    uint
		expected string
	}{
		{1, "1"},
		{123, "123"},
		{0, "0"},
		{999999, "999999"},
	}

	for _, tt := range tests {
		result := formatUintKey(tt.input)
		if result != tt.expected {
			t.Errorf("formatUintKey(%d) = %s; want %s", tt.input, result, tt.expected)
		}
	}
}

func TestParseInt64(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"123", 123},
		{"0", 0},
		{"", 0},
		{"abc", 0},
		{"999999999999", 999999999999},
	}

	for _, tt := range tests {
		result := parseInt64(tt.input)
		if result != tt.expected {
			t.Errorf("parseInt64(%q) = %d; want %d", tt.input, result, tt.expected)
		}
	}
}

func TestParseFloat64(t *testing.T) {
	tests := []struct {
		input    string
		expected float64
	}{
		{"123.45", 123.45},
		{"0", 0},
		{"", 0},
		{"-5.5", -5.5},
		{"abc", 0},
		{"3.14159", 3.14159},
	}

	for _, tt := range tests {
		result := parseFloat64(tt.input)
		if diff := result - tt.expected; diff > 0.001 || diff < -0.001 {
			t.Errorf("parseFloat64(%q) = %f; want %f", tt.input, result, tt.expected)
		}
	}
}

func TestHashToCourier(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]string
		expected *CourierLocation
	}{
		{
			name: "complete data",
			input: map[string]string{
				"deliveryman_id": "42",
				"name":           "João",
				"lat":            "-23.5505",
				"lng":            "-46.6333",
				"status":         "available",
				"last_update":    "1700000000000",
				"current_orders": "1",
				"max_orders":     "3",
			},
			expected: &CourierLocation{
				DeliverymanID: 42,
				Name:          "João",
				Lat:           -23.5505,
				Lng:           -46.6333,
				Status:        "available",
				LastUpdate:    1700000000000,
				CurrentOrders: 1,
				MaxOrders:     3,
			},
		},
		{
			name:     "empty data returns nil",
			input:    map[string]string{},
			expected: nil,
		},
		{
			name: "missing deliveryman_id returns nil",
			input: map[string]string{
				"name": "Maria",
			},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hashToCourier(tt.input)
			if tt.expected == nil {
				if result != nil {
					t.Errorf("hashToCourier() = %+v; want nil", result)
				}
				return
			}
			if result == nil {
				t.Fatal("hashToCourier() = nil; want non-nil")
			}
			if result.DeliverymanID != tt.expected.DeliverymanID {
				t.Errorf("DeliverymanID = %d; want %d", result.DeliverymanID, tt.expected.DeliverymanID)
			}
			if result.Name != tt.expected.Name {
				t.Errorf("Name = %s; want %s", result.Name, tt.expected.Name)
			}
			if result.Status != tt.expected.Status {
				t.Errorf("Status = %s; want %s", result.Status, tt.expected.Status)
			}
		})
	}
}

// TestFindNearbyScoring verifica a logica de scoring do FindNearby
// usando o CourierStore in-memory (nao ha dependencia externa).
func TestFindNearbyScoring(t *testing.T) {
	store := NewCourierStore()

	// Adiciona 3 entregadores em posicoes diferentes
	store.UpdateLocation(1, "Alice", -23.5500, -46.6330, "available")
	store.UpdateLocation(2, "Bob", -23.5600, -46.6430, "available")
	store.UpdateLocation(3, "Charlie", -23.8000, -46.9000, "available")

	// Alice deve ser a mais proxima do ponto de busca
	t.Run("nearest first", func(t *testing.T) {
		couriers := store.FindNearby(-23.5505, -46.6333, 10.0, 10)
		if len(couriers) == 0 {
			t.Fatal("Expected at least 1 courier, got 0")
		}
		if couriers[0].DeliverymanID != 1 {
			t.Errorf("Expected Alice (ID=1) first, got ID=%d (%s)", couriers[0].DeliverymanID, couriers[0].Name)
		}
	})

	// Entregador com max orders atingido deve ser ignorado
	t.Run("max orders respected", func(t *testing.T) {
		store.SetOrdersCount(1, 3) // Alice atingiu max de 3
		couriers := store.FindNearby(-23.5505, -46.6333, 10.0, 10)
		for _, c := range couriers {
			if c.DeliverymanID == 1 {
				t.Error("Alice should not appear (max orders reached)")
			}
		}
		store.SetOrdersCount(1, 0) // reset
	})

	// Busy couriers devem ser ignorados
	t.Run("status filtered", func(t *testing.T) {
		store.SetCourierStatus(1, "busy")
		couriers := store.FindNearby(-23.5505, -46.6333, 10.0, 10)
		for _, c := range couriers {
			if c.DeliverymanID == 1 {
				t.Error("Alice should not appear (busy)")
			}
		}
		store.SetCourierStatus(1, "available")
	})

	// Courier fora do raio deve ser ignorado
	t.Run("radius respected", func(t *testing.T) {
		couriers := store.FindNearby(-23.5505, -46.6333, 1.0, 10) // raio de 1km
		for _, c := range couriers {
			if c.DeliverymanID == 3 {
				t.Error("Charlie should not appear (>1km away)")
			}
		}
	})
}

func TestCountTotalByZone(t *testing.T) {
	store := NewCourierStore()

	store.UpdateLocation(1, "Alice", -23.5500, -46.6330, "available")
	store.UpdateLocation(2, "Bob", -23.5600, -46.6430, "busy")
	store.UpdateLocation(3, "Charlie", -23.8000, -46.9000, "offline")

	// Zona centrada em Sao Paulo, raio 5km: so Alice e Bob
	count := store.CountTotalByZone(1, -23.5505, -46.6333, 5.0)
	if count != 2 {
		t.Errorf("Expected 2 couriers in zone, got %d", count)
	}

	// Raio maior: todos os 3
	count = store.CountTotalByZone(1, -23.5505, -46.6333, 50.0)
	if count != 3 {
		t.Errorf("Expected 3 couriers in large zone, got %d", count)
	}
}

func TestEstimateDensity(t *testing.T) {
	store := NewCourierStore()

	// 2 couriers numa zona com area de ~78.5 km² (raio 5km)
	store.UpdateLocation(1, "Alice", -23.5500, -46.6330, "available")
	store.UpdateLocation(2, "Bob", -23.5600, -46.6430, "available")

	density := store.EstimateDensity(1, -23.5505, -46.6333, 5.0)
	// area = pi * 25 = 78.54, density = 2/78.54 = 0.0254
	if density < 0.02 || density > 0.03 {
		t.Errorf("Expected density ~0.025, got %f", density)
	}
}

func TestCleanupStale(t *testing.T) {
	store := NewCourierStore()

	store.UpdateLocation(1, "Alice", -23.5500, -46.6330, "available")
	store.UpdateLocation(2, "Bob", -23.5600, -46.6430, "offline")

	// Cleanup com 0 segundos: remove todos
	store.CleanupStale(0)

	couriers := store.FindNearby(-23.5505, -46.6333, 50.0, 10)
	if len(couriers) != 0 {
		t.Errorf("Expected 0 couriers after cleanup, got %d", len(couriers))
	}
}

func TestSetOrdersCount(t *testing.T) {
	store := NewCourierStore()
	store.UpdateLocation(1, "Alice", -23.5500, -46.6330, "available")

	// Verifica valor inicial
	c := store.GetCourier(1)
	if c == nil || c.CurrentOrders != 0 {
		t.Fatal("Expected CurrentOrders=0 initially")
	}

	store.SetOrdersCount(1, 2)
	c = store.GetCourier(1)
	if c == nil || c.CurrentOrders != 2 {
		t.Errorf("Expected CurrentOrders=2, got %d", c.CurrentOrders)
	}
}

func TestSetCourierStatus(t *testing.T) {
	store := NewCourierStore()
	store.UpdateLocation(1, "Alice", -23.5500, -46.6330, "available")

	c := store.GetCourier(1)
	if c == nil || c.Status != "available" {
		t.Fatal("Expected status=available")
	}

	store.SetCourierStatus(1, "busy")
	c = store.GetCourier(1)
	if c == nil || c.Status != "busy" {
		t.Errorf("Expected status=busy, got %s", c.Status)
	}
}
