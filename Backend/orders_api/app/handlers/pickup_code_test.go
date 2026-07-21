package handlers

import (
	"regexp"
	"testing"
)

func TestGenerateSecureCode(t *testing.T) {
	code := generateSecureCode()

	// Must be exactly 6 digits
	if len(code) != 6 {
		t.Errorf("Expected code length 6, got %d", len(code))
	}

	// Must contain only digits
	matched, _ := regexp.MatchString(`^\d{6}$`, code)
	if !matched {
		t.Errorf("Expected 6-digit numeric code, got %q", code)
	}
}

func TestGenerateSecureCodeUniqueness(t *testing.T) {
	codes := make(map[string]bool)
	iterations := 100

	for i := 0; i < iterations; i++ {
		code := generateSecureCode()
		if codes[code] {
			t.Errorf("Duplicate code generated: %q", code)
		}
		codes[code] = true
	}

	// With 6 digits, 100 codes should all be unique
	if len(codes) != iterations {
		t.Errorf("Expected %d unique codes, got %d", iterations, len(codes))
	}
}

func TestGenerateSecureCodeDistribution(t *testing.T) {
	// Check that all digits 0-9 can appear
	digitCount := make(map[byte]int)
	samples := 1000

	for i := 0; i < samples; i++ {
		code := generateSecureCode()
		for j := 0; j < len(code); j++ {
			digitCount[code[j]]++
		}
	}

	// All 10 digits should appear at least once
	for d := byte('0'); d <= '9'; d++ {
		if digitCount[d] == 0 {
			t.Errorf("Digit %c never appeared in %d samples", d, samples)
		}
	}
}
