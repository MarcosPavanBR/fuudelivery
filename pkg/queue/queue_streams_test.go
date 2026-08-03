package queue

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
)

// resetSingleton permite recriar o singleton com um novo REDIS_URL entre testes.
func resetSingleton() {
	defaultQueue = nil
	defaultOnce = sync.Once{}
}

// newTestQueue inicia um miniredis (Redis em memoria com suporte a Streams),
// configura REDIS_URL e retorna o singleton recriado com transport Redis.
func newTestQueue(t *testing.T) *Queue {
	t.Helper()
	s, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(s.Close)

	os.Setenv("REDIS_URL", "redis://"+s.Addr())
	t.Cleanup(func() { os.Unsetenv("REDIS_URL") })

	resetSingleton()
	q := New()
	if !q.IsRedis() {
		t.Fatal("esperava transport Redis ativo")
	}
	// Cleanup LIFO: q.Close (cancela ctx e fecha o cliente) e depois resetSingleton
	// (zera o singleton para o proximo teste recriar do zero).
	t.Cleanup(q.Close)
	t.Cleanup(resetSingleton)
	return q
}

// TestStreams_PublishSubscribeAck valida o ciclo feliz: publica -> consome -> XAck.
// Apos o ack, o pending entries list (PEL) do consumer group deve estar vazio.
func TestStreams_PublishSubscribeAck(t *testing.T) {
	q := newTestQueue(t)

	received := make(chan string, 1)
	q.Subscribe("orders", func(msg []byte) {
		received <- string(msg)
	})

	// da tempo do consumer subir e do group ser criado
	time.Sleep(300 * time.Millisecond)

	if err := q.Publish("orders", []byte(`{"id":"1"}`)); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	select {
	case m := <-received:
		if m != `{"id":"1"}` {
			t.Fatalf("mensagem inesperada: %s", m)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout esperando a mensagem")
	}

	// espera o XAck propagar e confirma PEL vazio (polling, nao sleep fixo)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		pend, err := q.client.XPending(context.Background(), "queue:orders", groupName).Result()
		if err != nil {
			t.Fatalf("XPending: %v", err)
		}
		if pend.Count == 0 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if pend, _ := q.client.XPending(context.Background(), "queue:orders", groupName).Result(); pend.Count != 0 {
		t.Fatalf("PEL deveria estar vazio apos o ack, count=%d", pend.Count)
	}
}

// TestStreams_FailureMovesToDLQ valida retry com limite: um handler que sempre
// falha deve tentar maxRetries vezes e entao a mensagem vai para a DLQ
// (queue:<nome>:dlq), com a original confirmada (sem reprocessamento eterno).
func TestStreams_FailureMovesToDLQ(t *testing.T) {
	// acelera os parametros de retry para o teste
	origMax, origIdle, origInterval := maxRetries, reclaimIdle, reclaimInterval
	maxRetries, reclaimIdle, reclaimInterval = 2, 150*time.Millisecond, 50*time.Millisecond
	t.Cleanup(func() { maxRetries, reclaimIdle, reclaimInterval = origMax, origIdle, origInterval })

	q := newTestQueue(t)

	var attempts atomic.Int32
	q.SubscribeFunc("payments", func(msg []byte) error {
		attempts.Add(1)
		return errors.New("falha simulada do handler")
	})

	time.Sleep(300 * time.Millisecond)
	if err := q.Publish("payments", []byte(`{"order_id":"9"}`)); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	// aguarda a mensagem chegar na DLQ (com timeout generoso)
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		msgs, err := q.client.XRange(context.Background(), "queue:payments:dlq", "-", "+").Result()
		if err == nil && len(msgs) > 0 {
			if len(msgs) != 1 {
				t.Fatalf("DLQ deveria ter 1 mensagem, tem %d", len(msgs))
			}
			payload, _ := msgs[0].Values["payload"].(string)
			if !strings.Contains(payload, `"order_id":"9"`) {
				t.Fatalf("payload inesperado na DLQ: %s", payload)
			}
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if time.Now().After(deadline) {
		t.Fatalf("mensagem nao chegou na DLQ (tentativas=%d)", attempts.Load())
	}

	// a original deve ter sido confirmada apos ir para a DLQ
	pend, err := q.client.XPending(context.Background(), "queue:payments", groupName).Result()
	if err != nil {
		t.Fatalf("XPending: %v", err)
	}
	if pend.Count != 0 {
		t.Fatalf("original nao confirmada apos DLQ: count=%d", pend.Count)
	}
	if int64(attempts.Load()) < maxRetries {
		t.Fatalf("deveria ter pelo menos %d tentativas, teve %d", maxRetries, attempts.Load())
	}
}

// TestStreams_ReclaimRedeliversPending valida a reivindicacao (XAutoClaim):
// um handler que falha na 1a tentativa e sucede na 2a deve receber a mesma
// mensagem de novo apos o reclaim idle — entrega at-least-once.
func TestStreams_ReclaimRedeliversPending(t *testing.T) {
	origIdle, origInterval := reclaimIdle, reclaimInterval
	reclaimIdle, reclaimInterval = 150*time.Millisecond, 50*time.Millisecond
	t.Cleanup(func() { reclaimIdle, reclaimInterval = origIdle, origInterval })

	q := newTestQueue(t)

	var attempts atomic.Int32
	received := make(chan string, 1)
	q.SubscribeFunc("deliveries", func(msg []byte) error {
		if attempts.Add(1) == 1 {
			return errors.New("falha transitoria")
		}
		received <- string(msg)
		return nil
	})

	time.Sleep(300 * time.Millisecond)
	if err := q.Publish("deliveries", []byte(`{"id":"7"}`)); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	select {
	case m := <-received:
		if !strings.Contains(m, `"id":"7"`) {
			t.Fatalf("mensagem inesperada: %s", m)
		}
	case <-time.After(10 * time.Second):
		t.Fatalf("timeout: mensagem nao reentregue pelo reclaim (tentativas=%d)", attempts.Load())
	}
	if attempts.Load() != 2 {
		t.Fatalf("esperava exatamente 2 tentativas (1 falha + 1 sucesso), teve %d", attempts.Load())
	}
}
