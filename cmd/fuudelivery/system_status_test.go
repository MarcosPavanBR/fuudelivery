package main

import (
	"strings"
	"testing"
)

func TestBuildSystemStatus(t *testing.T) {
	env := map[string]string{"ASAAS_WEBHOOK_TOKEN": "s3cr3t-val", "SPONSOR_DAILY_PRICE": "abc"}
	get := func(k string) string { return env[k] }

	items := buildSystemStatus(get, []string{"asaas", "pagarme"}, false, true)
	by := map[string]statusItem{}
	for _, it := range items {
		by[it.Key] = it
		if strings.Contains(it.Detail+it.Fix, "s3cr3t-val") {
			t.Fatal("valor de variável vazou no status")
		}
	}
	if by["storage"].OK || by["storage"].Fix == "" {
		t.Fatalf("storage desligado devia pedir configuração: %+v", by["storage"])
	}
	if !by["payments"].OK || !strings.Contains(by["payments"].Detail, "asaas, pagarme") {
		t.Fatalf("gateways: %+v", by["payments"])
	}
	if !by["webhook_asaas"].OK || by["webhook_pagarme"].OK {
		t.Fatalf("webhooks: %+v / %+v", by["webhook_asaas"], by["webhook_pagarme"])
	}
	if by["sponsor"].OK {
		t.Fatalf("preço inválido devia aparecer: %+v", by["sponsor"])
	}

	none := buildSystemStatus(func(string) string { return "" }, nil, true, true)
	for _, it := range none {
		if it.Key == "payments" && it.OK {
			t.Fatal("sem gateway não pode ficar ok")
		}
		if it.Key == "sponsor" && !it.OK {
			t.Fatal("destaque sem env usa padrão e fica ok")
		}
	}
}
