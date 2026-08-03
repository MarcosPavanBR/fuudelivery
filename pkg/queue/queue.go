// Package queue fornece uma fila de mensagens compartilhada entre os microservicos.
// Suporta Redis Streams como transport primario e Go channels como fallback
// quando Redis nao esta configurado. Elimina a duplicacao de logica Redis que existia
// em payment_api/webhook.go, Payment/queue/redis_queue.go e cmd/fuudelivery/pkg/queue.
//
// # Redis Streams (transport primario)
//
// Cada fila e um Redis Stream (`queue:<nome>`) consumido por um consumer group
// (`fuudelivery-consumers`). Isso garante:
//   - Entrega at-least-once: mensagens confirmadas explicitamente (XAck);
//   - Persistencia: mensagens nao se perdem em restart/deploy (ficam no stream);
//   - Retry com limite: handlers que falham sao tentados ate maxRetries;
//   - Dead-letter queue: apos maxRetries, a mensagem vai para `queue:<nome>:dlq`;
//   - Reclaim: mensagens pendentes ociosas (consumer crashou ou handler demorou)
//     sao reivindicadas via XClaim e processadas novamente;
//   - Entrega serializada: run() e reclaimLoop() nunca executam o handler em
//     paralelo para o mesmo consumer (lock interno).
//
// # Requisitos do handler
//
// Os handlers podem ser executados mais de uma vez para a mesma mensagem
// (redelivery at-least-once apos crash/reclaim) e, em ultima instancia, devem ser
// idempotentes. A entrega e serializada por consumer, mas diferentes consumers
// do mesmo grupo processam em paralelo — o handler deve ser seguro para
// concorrencia se usar estado compartilhado.
//
// # Uso basico
//
//	q := queue.New()
//	defer q.Close()
//	q.Publish("orders", []byte(`{"id":"123"}`))
//	q.Subscribe("orders", func(msg []byte) { fmt.Println(string(msg)) })
//
// Para controle de erro do handler (retry/DLQ), use SubscribeFunc:
//
//	q.SubscribeFunc("orders", func(msg []byte) error {
//		if err := process(msg); err != nil {
//			return err // mensagem sera retentada e, apos maxRetries, vai para a DLQ
//		}
//		return nil // sucesso: mensagem confirmada (XAck)
//	})
package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-redis/redis/v8"
)

const (
	// groupName e o consumer group compartilhado por todas as filas Redis.
	groupName = "fuudelivery-consumers"
)

// Parametros de confiabilidade (variaveis para permitir override em testes).
var (
	// maxRetries e o numero maximo de tentativas antes de mover a mensagem para a DLQ.
	maxRetries int64 = 3

	// reclaimIdle e o tempo de inatividade apos o qual uma mensagem pendente
	// (handler falhou ou consumer crashou) e reivindicada para nova tentativa.
	reclaimIdle = 30 * time.Second

	// reclaimInterval e a frequencia da varredura de mensagens pendentes.
	reclaimInterval = 15 * time.Second
)

// Queue e uma fila de mensagens com Redis Streams como transport e Go channels como fallback.
type Queue struct {
	client         *redis.Client
	useRedis       bool
	internalQueues map[string]chan []byte
	mu             sync.Mutex
	ctx            context.Context
	cancel         context.CancelFunc

	// Contadores atômicos para observabilidade (/metrics).
	// Accessíveis via Metrics(); incrementados em Publish/process/handleFailure.
	metrics struct {
		published int64
		delivered int64
		failed    int64
		dlq       int64
		reclaimed int64
	}
}

var (
	defaultQueue *Queue
	defaultOnce  sync.Once
)

// New retorna a instancia singleton da Queue.
// Se REDIS_URL nao estiver configurado ou Redis nao estiver acessivel,
// usa Go channels internos como fallback.
// A conexao Redis e inicializada sob demanda na primeira chamada.
func New() *Queue {
	defaultOnce.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		q := &Queue{
			internalQueues: make(map[string]chan []byte),
			ctx:            ctx,
			cancel:         cancel,
		}

		redisURL := os.Getenv("REDIS_URL")
		if redisURL == "" {
			log.Println("[QUEUE] REDIS_URL nao configurado, usando Go channels (fallback)")
			defaultQueue = q
			return
		}

		opts, err := redis.ParseURL(redisURL)
		if err != nil {
			log.Printf("[QUEUE] Erro ao parsear REDIS_URL: %v, usando Go channels", err)
			defaultQueue = q
			return
		}

		client := redis.NewClient(opts)
		pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer pingCancel()

		if err := client.Ping(pingCtx).Err(); err != nil {
			log.Printf("[QUEUE] Erro ao conectar no Redis: %v, usando Go channels", err)
			defaultQueue = q
			return
		}

		q.client = client
		q.useRedis = true
		log.Println("[QUEUE] Conectado ao Redis")
		defaultQueue = q
	})
	return defaultQueue
}

// streamKey retorna a chave do Redis Stream de uma fila.
func streamKey(queueName string) string {
	return "queue:" + queueName
}

// dlqKey retorna a chave da dead-letter queue de uma fila.
func dlqKey(queueName string) string {
	return "queue:" + queueName + ":dlq"
}

// retryHashKey retorna a chave do hash que conta tentativas de cada mensagem.
func retryHashKey(queueName string) string {
	return "queue:retry:" + queueName
}

// Publish envia uma mensagem para a fila identificada por queueName.
// Se Redis estiver disponivel, usa XAdd (Stream). Caso contrario, usa Go channels.
func (q *Queue) Publish(queueName string, data []byte) error {
	if q.useRedis && q.client != nil {
		atomic.AddInt64(&q.metrics.published, 1)
		return q.client.XAdd(q.ctx, &redis.XAddArgs{
			Stream: streamKey(queueName),
			Values: map[string]interface{}{"payload": data},
		}).Err()
	}

	// Fallback: Go channel
	q.mu.Lock()
	ch, ok := q.internalQueues[queueName]
	if !ok {
		ch = make(chan []byte, 100)
		q.internalQueues[queueName] = ch
	}
	q.mu.Unlock()

	select {
	case ch <- data:
	default:
		log.Printf("[QUEUE] Fila %s cheia, mensagem descartada", queueName)
	}
	return nil
}

// PublishJSON serializa o payload para JSON e publica na fila.
func (q *Queue) PublishJSON(queueName string, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return q.Publish(queueName, data)
}

// Subscribe consome mensagens da fila queueName. O handler nao retorna erro,
// portanto a mensagem e sempre confirmada apos a execucao. Se o handler precisa
// reportar falha para ativar retry/DLQ, use SubscribeFunc.
func (q *Queue) Subscribe(queueName string, handler func([]byte)) {
	q.SubscribeFunc(queueName, func(msg []byte) error {
		handler(msg)
		return nil
	})
}

// SubscribeFunc consome mensagens da fila queueName com um handler que pode
// retornar erro. Em caso de erro a mensagem NAO e confirmada (fica pendente),
// e apos maxRetries tentativas e movida para a dead-letter queue.
// Se Redis estiver disponivel, usa consumer group (XReadGroup). Caso contrario,
// usa Go channels. Roda em goroutine(s). Retorna imediatamente.
func (q *Queue) SubscribeFunc(queueName string, handler func([]byte) error) {
	if q.useRedis && q.client != nil {
		q.ensureGroup(queueName)
		consumer := newConsumer(q, queueName, handler)
		go consumer.run()
		go consumer.reclaimLoop()
		return
	}

	// Fallback: Go channel
	q.mu.Lock()
	ch, ok := q.internalQueues[queueName]
	if !ok {
		ch = make(chan []byte, 100)
		q.internalQueues[queueName] = ch
	}
	q.mu.Unlock()

	go func() {
		log.Printf("[QUEUE] Consumer Go channel iniciado em %s...", queueName)
		for msg := range ch {
			if err := handler(msg); err != nil {
				log.Printf("[QUEUE] Handler falhou em %s: %v", queueName, err)
			}
		}
	}()
}

// ensureGroup cria o consumer group do stream (idempotente).
// Se o group ja existe (BUSYGROUP), ignora.
func (q *Queue) ensureGroup(queueName string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := q.client.XGroupCreateMkStream(ctx, streamKey(queueName), groupName, "0").Err()
	if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
		log.Printf("[QUEUE] Erro ao criar consumer group %s/%s: %v", queueName, groupName, err)
	}
}

// consumer representa um consumidor de um stream Redis dentro do consumer group.
type consumer struct {
	q         *Queue
	queueName string
	name      string
	handler   func([]byte) error

	// deliveryMu serializa a entrega de mensagens: garante que run() e reclaimLoop()
	// nunca executem o handler em paralelo para o mesmo consumer (evita double
	// processing de uma mensagem cujo handler demore mais que reclaimIdle).
	deliveryMu sync.Mutex
}

// newConsumer cria um consumidor com nome unico (hostname-pid).
func newConsumer(q *Queue, queueName string, handler func([]byte) error) *consumer {
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "unknown"
	}
	return &consumer{
		q:         q,
		queueName: queueName,
		name:      fmt.Sprintf("%s-%d", host, os.Getpid()),
		handler:   handler,
	}
}

// run consome mensagens novas (>) do stream e as entrega ao handler.
// Roda em loop ate o contexto ser cancelado (Close/Shutdown).
func (c *consumer) run() {
	stream := streamKey(c.queueName)
	log.Printf("[QUEUE] Consumer Redis Streams iniciado em %s (consumer=%s)...", c.queueName, c.name)
	for {
		if c.q.ctx.Err() != nil {
			log.Printf("[QUEUE] Consumer encerrado em %s (shutdown)", c.queueName)
			return
		}

		streams, err := c.q.client.XReadGroup(c.q.ctx, &redis.XReadGroupArgs{
			Group:    groupName,
			Consumer: c.name,
			Streams:  []string{stream, ">"},
			Count:    10,
			Block:    2 * time.Second,
		}).Result()
		if err != nil {
			if err == redis.Nil {
				// Timeout do Block: nenhuma mensagem nova, segue o loop.
				continue
			}
			if c.q.ctx.Err() != nil {
				return
			}
			log.Printf("[QUEUE] Erro no XReadGroup (%s): %v, reconectando em 5s", c.queueName, err)
			time.Sleep(5 * time.Second)
			continue
		}

		// XReadGroup retorna []XStream; cada stream carrega suas []XMessage.
		for _, s := range streams {
			for _, msg := range s.Messages {
				c.process(msg)
			}
		}
	}
}

// process entrega uma mensagem ao handler e trata o resultado:
// sucesso -> XAck (e limpa o contador de tentativas); falha -> retry (nao confirma)
// e contabiliza tentativas.
// O lock deliveryMu serializa a entrega entre run() e reclaimLoop().
func (c *consumer) process(msg redis.XMessage) {
	c.deliveryMu.Lock()
	defer c.deliveryMu.Unlock()

	if err := c.safeHandle(msg); err != nil {
		c.handleFailure(msg, err)
		return
	}
	atomic.AddInt64(&c.q.metrics.delivered, 1)
	if err := c.q.client.XAck(c.q.ctx, streamKey(c.queueName), groupName, msg.ID).Err(); err != nil {
		log.Printf("[QUEUE] Erro ao confirmar mensagem %s em %s: %v", msg.ID, c.queueName, err)
		return
	}
	// Limpa o contador de tentativas caso a mensagem tenha falhado antes
	// e agora foi processada com sucesso (evita vazamento de chaves no Redis).
	if err := c.q.client.HDel(c.q.ctx, retryHashKey(c.queueName), msg.ID).Err(); err != nil {
		log.Printf("[QUEUE] Erro ao limpar contador de %s em %s: %v", msg.ID, c.queueName, err)
	}
}

// safeHandle executa o handler protegendo contra panic (que vira erro de retry).
func (c *consumer) safeHandle(msg redis.XMessage) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic no handler: %v", r)
		}
	}()
	return c.handler(payloadOf(msg))
}

// handleFailure contabiliza a falha da mensagem. Apos maxRetries, move a
// mensagem para a DLQ e a confirma no stream original (evita reprocessamento eterno).
func (c *consumer) handleFailure(msg redis.XMessage, handlerErr error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	atomic.AddInt64(&c.q.metrics.failed, 1)

	count, err := c.q.client.HIncrBy(ctx, retryHashKey(c.queueName), msg.ID, 1).Result()
	if err != nil {
		log.Printf("[QUEUE] Erro ao contabilizar tentativa em %s: %v", c.queueName, err)
		return
	}
	log.Printf("[QUEUE] Handler falhou em %s (msg=%s tentativa=%d/%d): %v",
		c.queueName, msg.ID, count, maxRetries, handlerErr)

	if count < maxRetries {
		// Nao confirma: a mensagem fica pendente e sera reivindicada pelo reclaimLoop
		// apos reclaimIdle, gerando a proxima tentativa.
		return
	}

	// Limite de tentativas atingido: move para a DLQ e confirma a original.
	if err := c.q.client.XAdd(ctx, &redis.XAddArgs{
		Stream: dlqKey(c.queueName),
		Values: map[string]interface{}{
			"payload":  string(payloadOf(msg)),
			"original": msg.ID,
			"reason":   handlerErr.Error(),
		},
	}).Err(); err != nil {
		log.Printf("[QUEUE] Erro ao mover msg %s para DLQ %s: %v", msg.ID, c.queueName, err)
		return
	}
	if err := c.q.client.XAck(ctx, streamKey(c.queueName), groupName, msg.ID).Err(); err != nil {
		log.Printf("[QUEUE] Erro ao confirmar msg %s apos DLQ em %s: %v", msg.ID, c.queueName, err)
	}
	if err := c.q.client.HDel(ctx, retryHashKey(c.queueName), msg.ID).Err(); err != nil {
		log.Printf("[QUEUE] Erro ao limpar contador de %s em %s: %v", msg.ID, c.queueName, err)
	}
	// Contador so incrementa apos o XAdd bem-sucedido (reflete DLQ real).
	atomic.AddInt64(&c.q.metrics.dlq, 1)
	log.Printf("[QUEUE] Mensagem %s movida para a DLQ de %s apos %d tentativas", msg.ID, c.queueName, maxRetries)
}

// reclaimLoop reivindica mensagens pendentes ociosas (consumers que crasharam ou
// falhas que ficaram pendentes), garantindo entrega at-least-once mesmo apos
// restart/deploy. Usa XPendingExt + XClaim (compativel com go-redis v8.11.5).
func (c *consumer) reclaimLoop() {
	stream := streamKey(c.queueName)
	ticker := time.NewTicker(reclaimInterval)
	defer ticker.Stop()
	log.Printf("[QUEUE] Reclaim loop iniciado em %s (idle=%s, intervalo=%s)", c.queueName, reclaimIdle, reclaimInterval)
	for {
		select {
		case <-c.q.ctx.Done():
			log.Printf("[QUEUE] Reclaim loop encerrado em %s (shutdown)", c.queueName)
			return
		case <-ticker.C:
		}

		// Lista as mensagens pendentes do consumer group com tempo de inatividade.
		pendings, err := c.q.client.XPendingExt(c.q.ctx, &redis.XPendingExtArgs{
			Stream: stream,
			Group:  groupName,
			Start:  "-",
			End:    "+",
			Count:  100,
		}).Result()
		if err != nil {
			if c.q.ctx.Err() != nil {
				return
			}
			log.Printf("[QUEUE] Erro no XPendingExt (%s): %v", c.queueName, err)
			continue
		}

		// Seleciona apenas as que estao ociosas ha mais de reclaimIdle.
		staleIDs := make([]string, 0, len(pendings))
		for _, p := range pendings {
			if p.Idle >= reclaimIdle {
				staleIDs = append(staleIDs, p.ID)
			}
		}
		if len(staleIDs) == 0 {
			continue
		}

		// Reivindica as mensagens e reprocessa (redelivery at-least-once).
		atomic.AddInt64(&c.q.metrics.reclaimed, int64(len(staleIDs)))
		claimed, err := c.q.client.XClaim(c.q.ctx, &redis.XClaimArgs{
			Stream:   stream,
			Group:    groupName,
			Consumer: c.name,
			MinIdle:  reclaimIdle,
			Messages: staleIDs,
		}).Result()
		if err != nil {
			if c.q.ctx.Err() != nil {
				return
			}
			log.Printf("[QUEUE] Erro no XClaim (%s): %v", c.queueName, err)
			continue
		}
		for _, msg := range claimed {
			c.process(msg)
		}
	}
}

// payloadOf extrai o campo "payload" de uma mensagem do stream.
func payloadOf(msg redis.XMessage) []byte {
	switch v := msg.Values["payload"].(type) {
	case string:
		return []byte(v)
	case []byte:
		return v
	default:
		return []byte(fmt.Sprintf("%v", v))
	}
}

// Stats e um snapshot dos contadores de observabilidade da fila.
type Stats struct {
	// Published total de mensagens publicadas (XAdd + fallback Go channel).
	Published int64
	// Delivered total de mensagens entregues com sucesso ao handler (XAck).
	Delivered int64
	// Failed total de falhas de handler (retry contabilizado).
	Failed int64
	// DLQ total de mensagens movidas para a dead-letter queue.
	DLQ int64
	// Reclaimed total de mensagens pendentes reivindicadas (crash recovery).
	Reclaimed int64
}

// Stats retorna um snapshot atomico dos contadores da fila.
func (q *Queue) Stats() Stats {
	return Stats{
		Published: atomic.LoadInt64(&q.metrics.published),
		Delivered: atomic.LoadInt64(&q.metrics.delivered),
		Failed:    atomic.LoadInt64(&q.metrics.failed),
		DLQ:       atomic.LoadInt64(&q.metrics.dlq),
		Reclaimed: atomic.LoadInt64(&q.metrics.reclaimed),
	}
}

// Close encerra a conexao com Redis (se ativa) e cancela todas as goroutines de consumer.
func (q *Queue) Close() {
	if q.cancel != nil {
		q.cancel()
	}
	if q.client != nil {
		q.client.Close()
		log.Println("[QUEUE] Conexao Redis encerrada")
	}
}

// IsRedis retorna true se a fila esta usando Redis como transport.
func (q *Queue) IsRedis() bool {
	return q.useRedis
}

// GetClient retorna o cliente Redis da fila (nil se nao estiver usando Redis).
// Utilizado pelo health endpoint para reutilizar a conexao existente.
func (q *Queue) GetClient() *redis.Client {
	return q.client
}

// GetClient retorna o cliente Redis da fila singleton.
func GetClient() *redis.Client {
	return New().GetClient()
}

// StatsSnapshot retorna o snapshot de contadores da fila singleton.
// Utilizado pelo endpoint /metrics do monolith.
func StatsSnapshot() Stats {
	return New().Stats()
}

// --- Package-level convenience functions ---
// Estas funcoes operam no singleton defaultQueue para compatibilidade
// com chamadas como queue.Init(), queue.Publish(), queue.Subscribe().

// Init inicializa a fila singleton. Chamar New() e suficiente;
// esta funcao existe para compatibilidade com codigo legado.
func Init() {
	New()
}

// Publish envia uma mensagem usando a fila singleton.
func Publish(queueName string, data []byte) error {
	return New().Publish(queueName, data)
}

// PublishJSON serializa e envia uma mensagem usando a fila singleton.
func PublishJSON(queueName string, payload interface{}) error {
	return New().PublishJSON(queueName, payload)
}

// Subscribe consome mensagens da fila usando a fila singleton.
func Subscribe(queueName string, handler func([]byte)) {
	New().Subscribe(queueName, handler)
}

// SubscribeFunc consome mensagens da fila usando a fila singleton, com retry/DLQ.
func SubscribeFunc(queueName string, handler func([]byte) error) {
	New().SubscribeFunc(queueName, handler)
}

// CloseQueue encerra a conexao da fila singleton.
func CloseQueue() {
	New().Close()
}
