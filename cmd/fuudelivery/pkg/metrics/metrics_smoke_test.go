package metrics

import (
	"strings"
	"testing"
)

// Prova que as métricas novas saem de verdade no texto Prometheus, e não só
// compilam. Métrica que existe no código e não aparece no /metrics é a mesma
// coisa que não existir.
func TestPrometheusText_TrazMetricasDaRedeDeSeguranca(t *testing.T) {
	DLQDepthFunc = func() int { return 7 }
	out := PrometheusText()

	for _, esperado := range []string{
		"fuudelivery_reconciliation_healed_total",
		"fuudelivery_reconciliation_failed_total",
		"fuudelivery_reconciliation_pending",
		"fuudelivery_orders_stuck",
		"fuudelivery_reconciliation_last_run_seconds",
		"fuudelivery_dispatch_dlq_depth 7",
	} {
		if !strings.Contains(out, esperado) {
			t.Errorf("/metrics não trouxe %q", esperado)
		}
	}

	// Sem o hook, a métrica de DLQ simplesmente não aparece (em vez de mentir 0).
	DLQDepthFunc = nil
	if strings.Contains(PrometheusText(), "fuudelivery_dispatch_dlq_depth") {
		t.Error("sem o hook ligado, dispatch_dlq_depth não deve aparecer")
	}
}
