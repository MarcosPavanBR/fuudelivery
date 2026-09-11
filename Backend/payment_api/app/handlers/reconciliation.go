package handlers

import (
	"log"
	"sync"
	"time"

	"github.com/carloshomar/fuudelivery/payment_api/app/models"
)

// Reconciliação de pagamentos: a rede de segurança do caminho do dinheiro.
//
// POR QUE ISTO EXISTE
//
// publishPaymentApproved executa em passos sequenciais, cada um com commit
// próprio. Se o processo morrer no meio, ou se o cálculo de split falhar, o
// pagamento fica CONFIRMED e a carteira do estabelecimento NÃO é creditada —
// e, como os eventos de notificação já saíram, o cliente já viu "pagamento
// confirmado" no app. Antes deste job, nada voltava para terminar o serviço:
// a única coisa que salvava era o gateway reenviar o webhook por conta
// própria, que é sorte de infraestrutura de terceiro, não desenho nosso.
//
// POR QUE NÃO É UM OUTBOX
//
// O projeto já tem a garantia mais forte onde ela importa: o UNIQUE
// uq_wallet_txns_credit_ref (sql/11) torna o crédito idempotente no banco.
// Com isso, "tentar de novo" é seguro, e uma varredura periódica resolve o
// mesmo problema que um outbox resolveria, sem reescrever o caminho de
// pagamento — que é justamente o código que mexe com dinheiro e está estável.
// Trocar uma garantia que funciona por uma a construir seria piorar o risco
// em nome da arquitetura.

const (
	// Quanto esperar antes de considerar um pagamento abandonado. Serve para
	// não correr com o webhook que ainda está executando: sem esta janela, o
	// job disputaria com o caminho síncrono e os dois tentariam creditar ao
	// mesmo tempo. A idempotência aguentaria, mas geraria erro e ruído.
	reconcileGracePeriod = 2 * time.Minute

	// Limite da varredura. Pagamento não liquidado há mais de uma semana é
	// problema para investigar à mão, não para o job ficar tentando para
	// sempre a cada 5 minutos.
	reconcileLookback = 7 * 24 * time.Hour

	// Teto por passada, para uma fila acumulada não virar uma tempestade de
	// escrita no banco num único tick.
	reconcileBatchSize = 100

	// A partir de quanto tempo um pedido pago sem entregador é digno de nota.
	stuckOrderThreshold = 15 * time.Minute
)

// ReconciliationStats é o resultado de uma passada.
type ReconciliationStats struct {
	Healed      int // pagamentos liquidados com sucesso nesta passada
	Failed      int // tentados e ainda falhando (ficam para a próxima)
	Pending     int // total encontrado por liquidar
	StuckOrders int // pedidos pagos sem entregador (só contados)
}

// Estatísticas acumuladas, para o /metrics ler. Mesmo padrão do
// queue.StatsSnapshot(): o pacote de métricas lê daqui em vez de este pacote
// importar métricas, o que evitaria um ciclo de import.
//
// Sem isto, o job seria só mais um log — e o valor de uma rede de segurança
// está em conseguir ALERTAR quando ela é acionada. healed_total subindo com
// frequência significa que o caminho síncrono do webhook está falhando, e essa
// é a causa que precisa ser investigada, não o sintoma que o job remedia.
var (
	reconcileMu      sync.RWMutex
	reconcileHealed  int64
	reconcileFailed  int64
	reconcileLast    ReconciliationStats
	reconcileLastRun time.Time
)

// ReconciliationSnapshot devolve os números acumulados e os da última passada.
func ReconciliationSnapshot() (healedTotal, failedTotal int64, last ReconciliationStats, lastRun time.Time) {
	reconcileMu.RLock()
	defer reconcileMu.RUnlock()
	return reconcileHealed, reconcileFailed, reconcileLast, reconcileLastRun
}

// ReconcilePaymentsOnce roda UMA passada e devolve o que encontrou.
//
// Exportada para ser chamada direto no teste, sem ticker: teste que depende de
// esperar um tick vira teste lento e intermitente.
func ReconcilePaymentsOnce() ReconciliationStats {
	var stats ReconciliationStats

	if models.DB == nil {
		return stats
	}

	cutoff := time.Now().Add(-reconcileGracePeriod)
	floor := time.Now().Add(-reconcileLookback)

	var pendentes []models.Payment
	err := models.DB.
		Where("status = ?", "CONFIRMED").
		Where("establishment_credited_at IS NULL").
		Where("confirmed_at < ?", cutoff).
		Where("confirmed_at > ?", floor).
		Order("confirmed_at ASC").
		Limit(reconcileBatchSize).
		Find(&pendentes).Error
	if err != nil {
		log.Printf("[RECONCILE] ERRO ao buscar pagamentos não liquidados: %v", err)
		return stats
	}

	stats.Pending = len(pendentes)

	for i := range pendentes {
		pagamento := &pendentes[i]

		// Chama o MESMO código do webhook, de propósito. Uma reimplementação
		// do split e do crédito aqui dentro seria um segundo caminho que move
		// dinheiro, e os dois divergiriam na primeira manutenção.
		if err := settlePaymentApproved(pagamento); err != nil {
			stats.Failed++
			log.Printf("[RECONCILE] Pagamento %s continua sem liquidar: %v", pagamento.AbacatePayID, err)
			continue
		}

		stats.Healed++
		log.Printf("[RECONCILE] Pagamento %s liquidado pela reconciliação (pedido %s, estabelecimento %d) — "+
			"o caminho síncrono do webhook tinha falhado",
			pagamento.AbacatePayID, pagamento.OrderID, pagamento.EstablishmentID)
	}

	stats.StuckOrders = contarPedidosParados()

	reconcileMu.Lock()
	reconcileHealed += int64(stats.Healed)
	reconcileFailed += int64(stats.Failed)
	reconcileLast = stats
	reconcileLastRun = time.Now()
	reconcileMu.Unlock()

	if stats.Healed > 0 || stats.Failed > 0 {
		log.Printf("[RECONCILE] Passada: %d liquidados, %d ainda falhando, %d pedidos pagos sem entregador",
			stats.Healed, stats.Failed, stats.StuckOrders)
	}

	return stats
}

// contarPedidosParados conta pedidos pagos que continuam sem entregador.
//
// SÓ CONTA, não despacha. A confirmação de pagamento não aciona o motor de
// matching: o despacho depende de alguém chamar POST /dispatch/trigger. Isso é
// desenho (o restaurante confirma que vai preparar antes de acionar o
// entregador), mas até aqui era um desenho SEM rede: se a chamada nunca
// acontecesse, o pedido ficava pago e parado e nada no backend percebia.
// Disparar o matching daqui mudaria regra de negócio; contar, não.
//
// Falha em silêncio de propósito. O vínculo passa por orders e batches, que
// são de outro módulo: se o esquema divergir ou a tabela não existir (o caso
// dos bancos de teste do payment_api), isto NÃO pode derrubar a liquidação do
// dinheiro, que é o que realmente importa nesta função.
func contarPedidosParados() int {
	var total int64

	// o.id é uint e p.order_id é texto — o cast é do inteiro para texto, que é
	// sempre válido; o contrário quebraria com qualquer order_id não numérico.
	const q = `
		SELECT COUNT(*)
		FROM payments p
		LEFT JOIN orders o ON o.id::text = p.order_id
		LEFT JOIN batches b ON b.id = o.batch_id
		WHERE p.status = 'CONFIRMED'
		  AND p.confirmed_at < ?
		  AND p.confirmed_at > ?
		  AND (o.id IS NULL OR o.batch_id IS NULL OR b.courier_id IS NULL)`

	err := models.DB.Raw(q,
		time.Now().Add(-stuckOrderThreshold),
		time.Now().Add(-reconcileLookback),
	).Scan(&total).Error
	if err != nil {
		log.Printf("[RECONCILE] (não crítico) não consegui contar pedidos parados: %v", err)
		return 0
	}

	return int(total)
}

// StartPaymentReconciliation roda a reconciliação periodicamente.
// Segue o padrão dos outros jobs do monólito (startRefreshTokenCleanup etc).
func StartPaymentReconciliation(interval time.Duration) {
	log.Printf("[RECONCILE] Reconciliação de pagamentos ativa (a cada %s)", interval)

	// Uma passada logo na subida: o restart pode ter sido exatamente o que
	// interrompeu uma liquidação no meio.
	ReconcilePaymentsOnce()

	ticker := time.NewTicker(interval)
	for range ticker.C {
		ReconcilePaymentsOnce()
	}
}
