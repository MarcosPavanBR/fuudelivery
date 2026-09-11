// Package metrics expoe metricas do monolith em formato Prometheus text
// (zero dependencias externas — contadores atomicos em memoria).
//
// O endpoint GET /metrics e consumido por ferramentas de observabilidade
// (Prometheus/Grafana, BetterStack, UptimeRobot) e cobre:
//   - metricas HTTP: requisicoes por rota/status e duracao;
//   - metricas da fila: publicadas, entregues, falhas, DLQ e reclaim
//     (via queue.Metrics(), contadores do pkg/queue).
//
// Para tracing distribuido (OpenTelemetry/OTLP), veja o README — o env
// OTEL_EXPORTER_OTLP_ENDPOINT habilita exportacao quando um collector
// estiver disponivel (Render free tier nao oferece collector nativo).
package metrics

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v2"

	paymentHandlers "github.com/carloshomar/fuudelivery/payment_api/app/handlers"
	"github.com/carloshomar/fuudelivery/pkg/queue"
)

// httpMetric agrega contador e soma de duracao para uma rota+status.
type httpMetric struct {
	count int64
	msSum int64
}

// Registry guarda os contadores HTTP do processo.
type Registry struct {
	mu      sync.RWMutex
	byRoute map[string]*httpMetric
	started time.Time
}

var registry = &Registry{
	byRoute: make(map[string]*httpMetric),
	started: time.Now(),
}

func (r *Registry) record(route string, status int, dur time.Duration) {
	key := route + "|" + strconv.Itoa(status)
	r.mu.Lock()
	m, ok := r.byRoute[key]
	if !ok {
		m = &httpMetric{}
		r.byRoute[key] = m
	}
	atomic.AddInt64(&m.count, 1)
	atomic.AddInt64(&m.msSum, dur.Milliseconds())
	r.mu.Unlock()
}

func (r *Registry) snapshot() map[string]*httpMetric {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string]*httpMetric, len(r.byRoute))
	for k, v := range r.byRoute {
		cp := &httpMetric{count: atomic.LoadInt64(&v.count), msSum: atomic.LoadInt64(&v.msSum)}
		out[k] = cp
	}
	return out
}

// Middleware registra rota, status e duracao de cada requisicao.
// Usa a rota real (c.Route().Path) — nao o path bruto — para nao explodir
// a cardinalidade com IDs dinâmicos. Rotas sem match (404) sao agrupadas
// sob "unmatched" em vez do path bruto com IDs.
func Middleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		err := c.Next()
		route := c.Route().Path
		if route == "" {
			route = "unmatched"
		}
		registry.record(route, c.Response().StatusCode(), time.Since(start))
		return err
	}
}

// PrometheusText renderiza as metricas em formato Prometheus text.
func PrometheusText() string {
	var b strings.Builder

	// --- Metricas de processo ---
	fmt.Fprintf(&b, "# HELP fuudelivery_uptime_seconds Tempo desde o inicio do processo.\n")
	fmt.Fprintf(&b, "# TYPE fuudelivery_uptime_seconds gauge\n")
	fmt.Fprintf(&b, "fuudelivery_uptime_seconds %d\n", int64(time.Since(registry.started).Seconds()))

	// --- Metricas HTTP ---
	fmt.Fprintf(&b, "# HELP fuudelivery_http_requests_total Total de requisicoes HTTP por rota e status.\n")
	fmt.Fprintf(&b, "# TYPE fuudelivery_http_requests_total counter\n")
	fmt.Fprintf(&b, "# HELP fuudelivery_http_request_duration_ms Soma de duracao das requisicoes (para media).\n")
	fmt.Fprintf(&b, "# TYPE fuudelivery_http_request_duration_ms counter\n")

	keys := make([]string, 0, len(registry.snapshot()))
	snap := registry.snapshot()
	for k := range snap {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		parts := strings.SplitN(k, "|", 2)
		route, status := parts[0], parts[1]
		m := snap[k]
		// %q produz uma string entre aspas com escaping valido para labels Prometheus.
		fmt.Fprintf(&b, "fuudelivery_http_requests_total{route=%q,status=%q} %d\n",
			route, status, atomic.LoadInt64(&m.count))
		fmt.Fprintf(&b, "fuudelivery_http_request_duration_ms{route=%q,status=%q} %d\n",
			route, status, atomic.LoadInt64(&m.msSum))
	}

	// --- Metricas da fila (pkg/queue) ---
	writeQueueMetrics(&b)

	// --- Rede de seguranca do caminho do dinheiro ---
	writeReconciliationMetrics(&b)

	return b.String()
}

// DLQDepthFunc devolve quantos pedidos estao esperando entregador na DLQ do
// dispatch. E um hook porque o motor de matching vive no pacote main, que este
// pacote nao pode importar; o main atribui isto na inicializacao.
var DLQDepthFunc func() int

// writeReconciliationMetrics expoe a rede de seguranca do pagamento.
//
// Estas quatro linhas sao o que transforma o job de reconciliacao de "mais um
// log" em algo acionavel:
//
//   - healed_total subindo com frequencia NAO e boa noticia. Significa que o
//     caminho sincrono do webhook esta falhando e a rede esta segurando a
//     queda. A causa e que precisa ser investigada.
//   - pending que sobe e nao volta a zero significa que a propria reconciliacao
//     esta falhando — dinheiro parado sem ninguem para buscar.
//   - orders_stuck e pedido pago que nunca recebeu entregador.
//   - dlq_depth e a fila de pedidos que nao acharam entregador.
func writeReconciliationMetrics(b *strings.Builder) {
	healed, failed, last, lastRun := paymentHandlers.ReconciliationSnapshot()

	fmt.Fprintf(b, "# HELP fuudelivery_reconciliation_healed_total Pagamentos liquidados pela reconciliacao (o webhook sincrono falhou).\n")
	fmt.Fprintf(b, "# TYPE fuudelivery_reconciliation_healed_total counter\n")
	fmt.Fprintf(b, "fuudelivery_reconciliation_healed_total %d\n", healed)

	fmt.Fprintf(b, "# HELP fuudelivery_reconciliation_failed_total Tentativas de liquidacao que continuaram falhando.\n")
	fmt.Fprintf(b, "# TYPE fuudelivery_reconciliation_failed_total counter\n")
	fmt.Fprintf(b, "fuudelivery_reconciliation_failed_total %d\n", failed)

	fmt.Fprintf(b, "# HELP fuudelivery_reconciliation_pending Pagamentos CONFIRMED sem credito na carteira na ultima passada.\n")
	fmt.Fprintf(b, "# TYPE fuudelivery_reconciliation_pending gauge\n")
	fmt.Fprintf(b, "fuudelivery_reconciliation_pending %d\n", last.Pending)

	fmt.Fprintf(b, "# HELP fuudelivery_orders_stuck Pedidos pagos que continuam sem entregador.\n")
	fmt.Fprintf(b, "# TYPE fuudelivery_orders_stuck gauge\n")
	fmt.Fprintf(b, "fuudelivery_orders_stuck %d\n", last.StuckOrders)

	fmt.Fprintf(b, "# HELP fuudelivery_reconciliation_last_run_seconds Segundos desde a ultima passada (0 = nunca rodou).\n")
	fmt.Fprintf(b, "# TYPE fuudelivery_reconciliation_last_run_seconds gauge\n")
	var desde int64
	if !lastRun.IsZero() {
		desde = int64(time.Since(lastRun).Seconds())
	}
	fmt.Fprintf(b, "fuudelivery_reconciliation_last_run_seconds %d\n", desde)

	if DLQDepthFunc != nil {
		fmt.Fprintf(b, "# HELP fuudelivery_dispatch_dlq_depth Pedidos na dead-letter queue do dispatch (sem entregador).\n")
		fmt.Fprintf(b, "# TYPE fuudelivery_dispatch_dlq_depth gauge\n")
		fmt.Fprintf(b, "fuudelivery_dispatch_dlq_depth %d\n", DLQDepthFunc())
	}
}

// writeQueueMetrics adiciona os contadores da fila (pkg/queue singleton).
func writeQueueMetrics(b *strings.Builder) {
	qm := queue.StatsSnapshot()
	fmt.Fprintf(b, "# HELP fuudelivery_queue_published_total Mensagens publicadas na fila.\n")
	fmt.Fprintf(b, "# TYPE fuudelivery_queue_published_total counter\n")
	fmt.Fprintf(b, "fuudelivery_queue_published_total %d\n", qm.Published)

	fmt.Fprintf(b, "# HELP fuudelivery_queue_publish_errors_total Falhas ao publicar (Redis down, fallback cheio).\n")
	fmt.Fprintf(b, "# TYPE fuudelivery_queue_publish_errors_total counter\n")
	fmt.Fprintf(b, "fuudelivery_queue_publish_errors_total %d\n", qm.PublishErrors)

	fmt.Fprintf(b, "# HELP fuudelivery_queue_delivered_total Mensagens entregues com sucesso (XAck).\n")
	fmt.Fprintf(b, "# TYPE fuudelivery_queue_delivered_total counter\n")
	fmt.Fprintf(b, "fuudelivery_queue_delivered_total %d\n", qm.Delivered)

	fmt.Fprintf(b, "# HELP fuudelivery_queue_failed_total Falhas de handler (retry).\n")
	fmt.Fprintf(b, "# TYPE fuudelivery_queue_failed_total counter\n")
	fmt.Fprintf(b, "fuudelivery_queue_failed_total %d\n", qm.Failed)

	fmt.Fprintf(b, "# HELP fuudelivery_queue_dlq_total Mensagens movidas para a dead-letter queue.\n")
	fmt.Fprintf(b, "# TYPE fuudelivery_queue_dlq_total counter\n")
	fmt.Fprintf(b, "fuudelivery_queue_dlq_total %d\n", qm.DLQ)

	fmt.Fprintf(b, "# HELP fuudelivery_queue_reclaimed_total Mensagens pendentes reivindicadas (crash recovery).\n")
	fmt.Fprintf(b, "# TYPE fuudelivery_queue_reclaimed_total counter\n")
	fmt.Fprintf(b, "fuudelivery_queue_reclaimed_total %d\n", qm.Reclaimed)
}

// Handler e o handler HTTP de GET /metrics.
func Handler(c *fiber.Ctx) error {
	c.Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	return c.SendString(PrometheusText())
}
