package health

import (
	"os"
	"testing"
)

func TestGatewayCheck_NoGatewayConfigured(t *testing.T) {
	// Ensure no gateway keys are set
	os.Unsetenv("ABACATE_PAY_API_KEY")
	os.Unsetenv("PAGARME_API_KEY")
	os.Unsetenv("ASAAS_API_KEY")
	os.Unsetenv("MERCADOPAGO_ACCESS_TOKEN")

	check := GatewayCheck()
	if check.Status != "down" {
		t.Errorf("expected 'down' when no gateway configured, got '%s'", check.Status)
	}
	if check.Error != "no payment gateway configured" {
		t.Errorf("expected 'no payment gateway configured', got '%s'", check.Error)
	}
	if check.Name != "payment_gateways" {
		t.Errorf("expected name 'payment_gateways', got '%s'", check.Name)
	}
}

func TestGatewayCheck_OneGatewayConfigured(t *testing.T) {
	os.Setenv("ABACATE_PAY_API_KEY", "test-key")
	defer os.Unsetenv("ABACATE_PAY_API_KEY")

	check := GatewayCheck()
	if check.Status != "up" {
		t.Errorf("expected 'up' when one gateway configured, got '%s'", check.Status)
	}
	if check.Name != "payment_gateways" {
		t.Errorf("expected name 'payment_gateways', got '%s'", check.Name)
	}
}

func TestGatewayCheck_MultipleGatewaysConfigured(t *testing.T) {
	os.Setenv("ABACATE_PAY_API_KEY", "test-key-1")
	os.Setenv("PAGARME_API_KEY", "test-key-2")
	defer os.Unsetenv("ABACATE_PAY_API_KEY")
	defer os.Unsetenv("PAGARME_API_KEY")

	check := GatewayCheck()
	if check.Status != "up" {
		t.Errorf("expected 'up' when multiple gateways configured, got '%s'", check.Status)
	}
}
