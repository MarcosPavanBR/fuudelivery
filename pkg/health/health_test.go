package health

import (
	"testing"
)

func TestOverallStatus_AllUp(t *testing.T) {
	status := OverallStatus(
		Check{Name: "mongodb", Status: "up"},
		Check{Name: "postgres", Status: "up"},
	)
	if status != "up" {
		t.Errorf("expected 'up', got '%s'", status)
	}
}

func TestOverallStatus_OneDown(t *testing.T) {
	status := OverallStatus(
		Check{Name: "mongodb", Status: "up"},
		Check{Name: "postgres", Status: "down"},
	)
	if status != "down" {
		t.Errorf("expected 'down', got '%s'", status)
	}
}

func TestOverallStatus_OneDegraded(t *testing.T) {
	status := OverallStatus(
		Check{Name: "mongodb", Status: "up"},
		Check{Name: "postgres", Status: "degraded"},
	)
	if status != "degraded" {
		t.Errorf("expected 'degraded', got '%s'", status)
	}
}

func TestOverallStatus_DownOverridesDegraded(t *testing.T) {
	status := OverallStatus(
		Check{Name: "mongodb", Status: "down"},
		Check{Name: "postgres", Status: "degraded"},
	)
	if status != "down" {
		t.Errorf("expected 'down', got '%s'", status)
	}
}

func TestOverallStatus_SingleCheck(t *testing.T) {
	status := OverallStatus(Check{Name: "redis", Status: "up"})
	if status != "up" {
		t.Errorf("expected 'up', got '%s'", status)
	}
}

func TestOverallStatus_EmptyChecks(t *testing.T) {
	status := OverallStatus()
	if status != "up" {
		t.Errorf("expected 'up' with no checks, got '%s'", status)
	}
}

func TestOverallStatus_AllDown(t *testing.T) {
	status := OverallStatus(
		Check{Name: "mongodb", Status: "down"},
		Check{Name: "postgres", Status: "down"},
		Check{Name: "redis", Status: "down"},
	)
	if status != "down" {
		t.Errorf("expected 'down', got '%s'", status)
	}
}

func TestOverallStatus_AllDegraded(t *testing.T) {
	status := OverallStatus(
		Check{Name: "mongodb", Status: "degraded"},
		Check{Name: "postgres", Status: "degraded"},
	)
	if status != "degraded" {
		t.Errorf("expected 'degraded', got '%s'", status)
	}
}

func TestCheck_NilDatabase(t *testing.T) {
	// DatabaseCheck with nil db should return down
	check := DatabaseCheck(nil)
	if check.Status != "down" {
		t.Errorf("expected 'down' for nil db, got '%s'", check.Status)
	}
	if check.Error != "database not configured" {
		t.Errorf("expected 'database not configured' error, got '%s'", check.Error)
	}
}

func TestCheck_NilRedis(t *testing.T) {
	// RedisCheck with nil client should return down
	check := RedisCheck(nil)
	if check.Status != "down" {
		t.Errorf("expected 'down' for nil client, got '%s'", check.Status)
	}
	if check.Error != "redis not configured" {
		t.Errorf("expected 'redis not configured' error, got '%s'", check.Error)
	}
}

func TestCheck_RedisGeoNil(t *testing.T) {
	check := RedisGeoCheck(nil)
	if check.Status != "down" {
		t.Errorf("expected 'down' for nil client, got '%s'", check.Status)
	}
}

func TestCheck_BatchNilDB(t *testing.T) {
	check := BatchCheck(nil)
	if check.Status != "down" {
		t.Errorf("expected 'down' for nil db, got '%s'", check.Status)
	}
	if check.Error != "database not configured" {
		t.Errorf("expected 'database not configured' error, got '%s'", check.Error)
	}
}

func TestOverallStatus_Lifecycle(t *testing.T) {
	// Simulates a full lifecycle:
	// 1. All services start healthy
	// 2. MongoDB goes down
	// 3. MongoDB recovers but Postgres degrades
	// 4. Everything recovers

	// Step 1: All up
	s1 := OverallStatus(
		Check{Name: "mongodb", Status: "up"},
		Check{Name: "postgres", Status: "up"},
		Check{Name: "redis", Status: "up"},
	)
	if s1 != "up" {
		t.Fatalf("step 1: expected 'up', got '%s'", s1)
	}

	// Step 2: MongoDB down
	s2 := OverallStatus(
		Check{Name: "mongodb", Status: "down"},
		Check{Name: "postgres", Status: "up"},
		Check{Name: "redis", Status: "up"},
	)
	if s2 != "down" {
		t.Fatalf("step 2: expected 'down', got '%s'", s2)
	}

	// Step 3: MongoDB recovered, Postgres degraded
	s3 := OverallStatus(
		Check{Name: "mongodb", Status: "up"},
		Check{Name: "postgres", Status: "degraded"},
		Check{Name: "redis", Status: "up"},
	)
	if s3 != "degraded" {
		t.Fatalf("step 3: expected 'degraded', got '%s'", s3)
	}

	// Step 4: Everything recovered
	s4 := OverallStatus(
		Check{Name: "mongodb", Status: "up"},
		Check{Name: "postgres", Status: "up"},
		Check{Name: "redis", Status: "up"},
	)
	if s4 != "up" {
		t.Fatalf("step 4: expected 'up', got '%s'", s4)
	}
}

// ── GatewayCheck: reflete a cadeia REAL do router ──
//
// Antes, esta função lia ABACATE_PAY_API_KEY, PAGARME_API_KEY etc. por conta
// própria e adivinhava a disponibilidade. Quem monta a cadeia de verdade é
// buildPaymentGateways(), que também descarta gateway cujo construtor falhou:
// credencial presente porém inválida deixava a cadeia VAZIA com o /health
// dizendo 200 "up", toda cobrança falhando e o load balancer achando o
// serviço saudável. Agora a lista vem de Router.Gateways().

func TestGatewayCheck_CadeiaVaziaFicaDown(t *testing.T) {
	for _, registered := range [][]string{nil, {}} {
		check := GatewayCheck(registered)
		if check.Status != "down" {
			t.Fatalf("cadeia sem gateway deve ficar down, veio %q", check.Status)
		}
	}
}

func TestGatewayCheck_CadeiaComGatewayFicaUp(t *testing.T) {
	check := GatewayCheck([]string{"pagarme", "asaas"})
	if check.Status != "up" {
		t.Fatalf("cadeia com gateway deve ficar up, veio %q", check.Status)
	}
}

// A consequência que interessa: cadeia vazia derruba o status geral, e é isso
// que faz o /health devolver 503 em vez de 200.
func TestGatewayCheck_VaziaDerrubaOStatusGeral(t *testing.T) {
	status := OverallStatus(
		Check{Name: "postgres", Status: "up"},
		GatewayCheck(nil),
	)
	if status != "down" {
		t.Fatalf("Postgres up mas sem gateway deve dar down (503), veio %q", status)
	}
}
