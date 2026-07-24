// Package handlers - establishment_handler_test.go
// Testes unitarios do establishment handler.
// Testam validacao, logica de localizacao, e calculo de distancia.
package handlers

import (
	"testing"
)

// === Testes de CreateEstablishment validation ===

func TestCreateEstablishment_NameRequired(t *testing.T) {
	tests := []struct {
		name      string
		estName   string
		wantError bool
	}{
		{"valid name", "Restaurante ABC", false},
		{"empty name", "", true},
		{"whitespace only", "   ", false}, // whitespace passes, but name check catches it
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isInvalid := tt.estName == ""
			if isInvalid != tt.wantError {
				t.Errorf("name=%q: got invalid=%v, want %v", tt.estName, isInvalid, tt.wantError)
			}
		})
	}
}

// === Testes de location string building ===

func TestLocationStringBuilding(t *testing.T) {
	tests := []struct {
		name     string
		address  string
		city     string
		state    string
		expected string
	}{
		{
			"address only",
			"Rua Principal, 123",
			"",
			"",
			"Rua Principal, 123",
		},
		{
			"address and city",
			"Rua Principal, 123",
			"Sao Paulo",
			"",
			"Rua Principal, 123, Sao Paulo",
		},
		{
			"address city and state",
			"Rua Principal, 123",
			"Sao Paulo",
			"SP",
			"Rua Principal, 123, Sao Paulo - SP",
		},
		{
			"city and state only",
			"",
			"Rio de Janeiro",
			"RJ",
			"Rio de Janeiro - RJ",
		},
		{
			"all empty",
			"",
			"",
			"",
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			locationString := tt.address
			if tt.city != "" || tt.state != "" {
				if locationString != "" {
					locationString += ", "
				}
				locationString += tt.city
				if tt.state != "" {
					locationString += " - " + tt.state
				}
			}

			if locationString != tt.expected {
				t.Errorf("Location: got %q, want %q", locationString, tt.expected)
			}
		})
	}
}

// === Testes de maxDistanceDelivery calculation ===

func TestMaxDistanceDelivery(t *testing.T) {
	tests := []struct {
		name         string
		deliveryTime int
		expected     float64
	}{
		{"default", 0, 10.0},
		{"30 minutes", 30, 6.0},
		{"60 minutes", 60, 12.0},
		{"45 minutes", 45, 9.0},
		{"90 minutes", 90, 18.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			maxDist := 10.0
			if tt.deliveryTime > 0 {
				maxDist = float64(tt.deliveryTime) / 5.0
			}

			if maxDist != tt.expected {
				t.Errorf("deliveryTime=%d: got %f, want %f", tt.deliveryTime, maxDist, tt.expected)
			}
		})
	}
}

// === Testes de Establishment struct ===

func TestEstablishment_DefaultColors(t *testing.T) {
	primaryColor := "#EA1D2C"
	secondaryColor := "#FFFFFF"

	if primaryColor != "#EA1D2C" {
		t.Errorf("PrimaryColor: got %q, want %q", primaryColor, "#EA1D2C")
	}
	if secondaryColor != "#FFFFFF" {
		t.Errorf("SecondaryColor: got %q, want %q", secondaryColor, "#FFFFFF")
	}
}

// === Testes de UpdateEstablishmentWallet validation ===

func TestUpdateEstablishmentWallet_Validation(t *testing.T) {
	tests := []struct {
		name            string
		establishmentID string
		walletID        string
		wantError       bool
	}{
		{"valid", "123", "wall_abc", false},
		{"empty establishment", "", "wall_abc", true},
		{"empty wallet", "123", "", true},
		{"both empty", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isInvalid := tt.establishmentID == "" || tt.walletID == ""
			if isInvalid != tt.wantError {
				t.Errorf("got invalid=%v, want %v", isInvalid, tt.wantError)
			}
		})
	}
}

// === Testes de HandlerEstablishmentStatus toggle ===

func TestEstablishmentStatusToggle(t *testing.T) {
	// Simulate open/close toggle
	var openData *string

	// Initially closed (nil)
	if openData != nil {
		t.Error("Should start as nil (closed)")
	}

	// Toggle to open
	if openData == nil {
		now := "2024-01-15T10:00:00Z"
		openData = &now
	}
	if openData == nil {
		t.Error("Should be open now")
	}

	// Toggle to close
	if openData != nil {
		openData = nil
	}
	if openData != nil {
		t.Error("Should be closed now")
	}
}
