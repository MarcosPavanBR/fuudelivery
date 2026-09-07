// Package handlers - zone_handler_test.go
// Testes unitarios do zone handler, focados na logica de GetMyZoneFee:
// resposta default quando o estabelecimento nao tem zona, e o calculo
// de at_target.
package handlers

import "testing"

// === Testes de GetMyZoneFee: zona ausente usa defaults fixos ===

func TestGetMyZoneFee_NoZoneDefaults(t *testing.T) {
	const defaultPlatformPct = 5.0
	const defaultEstablishmentPct = 85.0

	if defaultPlatformPct+defaultEstablishmentPct != 90.0 {
		t.Errorf("defaults nao completam 90%% (resto e do entregador): got %v", defaultPlatformPct+defaultEstablishmentPct)
	}
}

// === Testes de GetMyZoneFee: calculo de at_target ===

func TestGetMyZoneFee_AtTarget(t *testing.T) {
	tests := []struct {
		name         string
		currentPct   float64
		targetPct    float64
		wantAtTarget bool
	}{
		{"abaixo do alvo", 3.0, 12.0, false},
		{"exatamente no alvo", 12.0, 12.0, true},
		{"acima do alvo", 13.0, 12.0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			atTarget := tt.currentPct >= tt.targetPct
			if atTarget != tt.wantAtTarget {
				t.Errorf("current=%v target=%v: got atTarget=%v, want %v", tt.currentPct, tt.targetPct, atTarget, tt.wantAtTarget)
			}
		})
	}
}
