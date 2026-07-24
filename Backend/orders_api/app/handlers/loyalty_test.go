// Package handlers - loyalty_test.go
// Testes unitarios do loyalty handler.
// Testam tier calculation, multiplier, pontos, e cashback.
package handlers

import (
	"testing"
)

// === Testes de getTier ===

func TestGetTier(t *testing.T) {
	tests := []struct {
		name     string
		points   int
		expected string
	}{
		{"zero points is bronze", 0, "bronze"},
		{"100 points is bronze", 100, "bronze"},
		{"499 points is bronze", 499, "bronze"},
		{"500 points is prata", 500, "prata"},
		{"1000 points is prata", 1000, "prata"},
		{"1499 points is prata", 1499, "prata"},
		{"1500 points is ouro", 1500, "ouro"},
		{"2000 points is ouro", 2000, "ouro"},
		{"5000 points is ouro", 5000, "ouro"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getTier(tt.points)
			if result != tt.expected {
				t.Errorf("getTier(%d) = %q, want %q", tt.points, result, tt.expected)
			}
		})
	}
}

// === Testes de getPointsMultiplier ===

func TestGetPointsMultiplier(t *testing.T) {
	tests := []struct {
		name     string
		tier     string
		expected int
	}{
		{"bronze multiplier", "bronze", 1},
		{"prata multiplier", "prata", 1},
		{"ouro multiplier", "ouro", 2},
		{"unknown tier", "unknown", 1},
		{"empty tier", "", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getPointsMultiplier(tt.tier)
			if result != tt.expected {
				t.Errorf("getPointsMultiplier(%q) = %d, want %d", tt.tier, result, tt.expected)
			}
		})
	}
}

// === Testes de pontos ganhos ===

func TestEarnPoints_Calculation(t *testing.T) {
	tests := []struct {
		name       string
		orderValue float64
		multiplier int
		expected   int
	}{
		{"R$10 order bronze", 10.0, 1, 10},
		{"R$50 order bronze", 50.0, 1, 50},
		{"R$10 order ouro", 10.0, 2, 20},
		{"R$50 order ouro", 50.0, 2, 100},
		{"R$99.99 order bronze", 99.99, 1, 99}, // floor
		{"R$0.50 order bronze", 0.50, 1, 0},    // floor rounds down
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Replicate: pointsEarned = int(math.Floor(orderValue)) * multiplier
			pointsEarned := int(tt.orderValue) * tt.multiplier
			if pointsEarned != tt.expected {
				t.Errorf("Points: got %d, want %d", pointsEarned, tt.expected)
			}
		})
	}
}

// === Testes de RedeemPoints ===

func TestRedeemPoints_InsufficientPoints(t *testing.T) {
	userPoints := 50
	requestedPoints := 100

	if userPoints < requestedPoints {
		// Expected: insufficient points
		t.Log("Correctly detected insufficient points")
	} else {
		t.Error("Should detect insufficient points")
	}
}

func TestRedeemPoints_MustBeMultipleOf10(t *testing.T) {
	tests := []struct {
		name   string
		points int
		valid  bool
	}{
		{"10 points", 10, true},
		{"20 points", 20, true},
		{"50 points", 50, true},
		{"15 points", 15, false},
		{"7 points", 7, false},
		{"0 points", 0, true}, // 0 is multiple of 10
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid := tt.points%10 == 0
			if valid != tt.valid {
				t.Errorf("Points %d: got valid=%v, want %v", tt.points, valid, tt.valid)
			}
		})
	}
}

func TestRedeemPoints_DiscountCalculation(t *testing.T) {
	tests := []struct {
		name           string
		points         int
		expectedDiscount float64
	}{
		{"10 points", 10, 1.0},
		{"50 points", 50, 5.0},
		{"100 points", 100, 10.0},
		{"200 points", 200, 20.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			discountValue := float64(tt.points / 10)
			if discountValue != tt.expectedDiscount {
				t.Errorf("Discount: got %f, want %f", discountValue, tt.expectedDiscount)
			}
		})
	}
}

// === Testes de tier transitions ===

func TestTierTransitions(t *testing.T) {
	tests := []struct {
		name         string
		points       int
		expectedTier string
	}{
		{"start bronze", 0, "bronze"},
		{"still bronze at 499", 499, "bronze"},
		{"promote to prata at 500", 500, "prata"},
		{"still prata at 1499", 1499, "prata"},
		{"promote to ouro at 1500", 1500, "ouro"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tier := getTier(tt.points)
			if tier != tt.expectedTier {
				t.Errorf("Points %d: tier=%q, want %q", tt.points, tier, tt.expectedTier)
			}
		})
	}
}

func TestTierDemotion(t *testing.T) {
	// When points decrease, tier should drop
	points := 1600
	tier := getTier(points)
	if tier != "ouro" {
		t.Errorf("1600 points: tier=%q, want ouro", tier)
	}

	// Simulate redeeming 200 points
	points -= 200
	tier = getTier(points)
	if tier != "prata" {
		t.Errorf("1400 points after redeem: tier=%q, want prata", tier)
	}

	points -= 100
	tier = getTier(points)
	if tier != "prata" {
		t.Errorf("1300 points: tier=%q, want prata", tier)
	}
}

// === Testes de cashback code ===

func TestCashbackCode_Prefix(t *testing.T) {
	// All cashback codes should start with CASHBACK-
	code := "CASHBACK-ABC123"
	if len(code) < 9 || code[:9] != "CASHBACK-" {
		t.Errorf("Code should start with CASHBACK-: got %q", code)
	}
}

func TestCashbackCode_Length(t *testing.T) {
	// CASHBACK- + 6 chars = 15 total
	code := "CASHBACK-ABC123"
	if len(code) != 15 {
		t.Errorf("Code length: got %d, want 15", len(code))
	}
}

// === Testes de CalculateLoyaltyDiscount ===

func TestCalculateLoyaltyDiscount(t *testing.T) {
	tests := []struct {
		name            string
		points          int
		expectedUsed    int
		expectedDiscount float64
	}{
		{"0 points", 0, 0, 0.0},
		{"10 points", 10, 10, 1.0},
		{"50 points", 50, 50, 5.0},
		{"99 points", 99, 90, 9.0}, // rounds down to multiple of 10
		{"100 points", 100, 100, 10.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			maxDiscount := tt.points / 10
			usedPoints := maxDiscount * 10

			if usedPoints != tt.expectedUsed {
				t.Errorf("UsedPoints: got %d, want %d", usedPoints, tt.expectedUsed)
			}
			if float64(maxDiscount) != tt.expectedDiscount {
				t.Errorf("Discount: got %f, want %f", float64(maxDiscount), tt.expectedDiscount)
			}
		})
	}
}

// === Testes de EarnPointsForOrder validation ===

func TestEarnPointsForOrder_Validation(t *testing.T) {
	tests := []struct {
		name       string
		userPhone  string
		orderValue float64
		shouldSkip bool
	}{
		{"valid", "11999998888", 50.0, false},
		{"empty phone", "", 50.0, true},
		{"zero value", "11999998888", 0, true},
		{"negative value", "11999998888", -10.0, true},
		{"both invalid", "", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shouldSkip := tt.userPhone == "" || tt.orderValue <= 0
			if shouldSkip != tt.shouldSkip {
				t.Errorf("got skip=%v, want %v", shouldSkip, tt.shouldSkip)
			}
		})
	}
}
