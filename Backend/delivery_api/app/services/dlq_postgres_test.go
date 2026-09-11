//go:build integration

package services

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// setupTestDB cria um Postgres temporário via testcontainers para testes de DLQ.
func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	uri := os.Getenv("POSTGRES_TEST_URI")
	if uri != "" {
		db, err := gorm.Open(postgres.Open(uri), &gorm.Config{PrepareStmt: false})
		if err != nil {
			t.Fatalf("falha ao conectar ao Postgres de teste: %v", err)
		}
		// Cria a tabela se não existir (em vez de rodar a migration completa)
		db.Exec(`CREATE TABLE IF NOT EXISTS unmatched_orders (
			id BIGSERIAL PRIMARY KEY,
			order_id VARCHAR(64) NOT NULL,
			establishment_lat DOUBLE PRECISION NOT NULL,
			establishment_lng DOUBLE PRECISION NOT NULL,
			zone_id INT NOT NULL DEFAULT 0,
			created_at BIGINT NOT NULL,
			retry_count INT NOT NULL DEFAULT 0,
			last_attempt_at BIGINT NOT NULL,
			metadata JSONB,
			created_tz TIMESTAMPTZ NOT NULL DEFAULT now()
		)`)
		// Isolamento entre testes. Sem isto, os testes só são corretos sob
		// testcontainers (um container novo por teste); com POSTGRES_TEST_URI
		// apontando para um banco compartilhado, cada teste herda as linhas do
		// anterior e o PopNext (ORDER BY created_at ASC) devolve o pedido de
		// OUTRO teste. Foi o que mascarou o bug do metadata jsonb.
		db.Exec("TRUNCATE unmatched_orders")
		return db
	}

	ctx := context.Background()
	req := testcontainers.ContainerRequest{
		Image:        "postgres:16-alpine",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_DB":       "dlq_test",
			"POSTGRES_USER":     "test",
			"POSTGRES_PASSWORD": "test",
		},
		WaitingFor: wait.ForListeningPort("5432/tcp").WithStartupTimeout(60 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("falha ao criar container Postgres: %v", err)
	}
	t.Cleanup(func() { container.Terminate(ctx) })

	host, _ := container.Host(ctx)
	port, _ := container.MappedPort(ctx, "5432")
	dsn := fmt.Sprintf("host=%s port=%s user=test password=test dbname=dlq_test sslmode=disable", host, port.Port())

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{PrepareStmt: false})
	if err != nil {
		t.Fatalf("falha ao conectar: %v", err)
	}

	// Cria a tabela
	db.Exec(`CREATE TABLE IF NOT EXISTS unmatched_orders (
		id BIGSERIAL PRIMARY KEY,
		order_id VARCHAR(64) NOT NULL,
		establishment_lat DOUBLE PRECISION NOT NULL,
		establishment_lng DOUBLE PRECISION NOT NULL,
		zone_id INT NOT NULL DEFAULT 0,
		created_at BIGINT NOT NULL,
		retry_count INT NOT NULL DEFAULT 0,
		last_attempt_at BIGINT NOT NULL,
		metadata JSONB,
		created_tz TIMESTAMPTZ NOT NULL DEFAULT now()
	)`)
	db.Exec("TRUNCATE unmatched_orders")

	return db
}

func TestPostgresDLQStore_PushAndLen(t *testing.T) {
	db := setupTestDB(t)
	dlq := NewPostgresDLQStore(db)

	dlq.Push(&UnmatchedOrder{OrderID: "order-1", EstablishmentLat: -23.5, EstablishmentLng: -46.6, CreatedAt: time.Now().UnixMilli(), LastAttemptAt: time.Now().UnixMilli()})
	dlq.Push(&UnmatchedOrder{OrderID: "order-2", EstablishmentLat: -23.5, EstablishmentLng: -46.6, CreatedAt: time.Now().UnixMilli(), LastAttemptAt: time.Now().UnixMilli()})

	if dlq.Len() != 2 {
		t.Errorf("DLQ Len: got %d, want 2", dlq.Len())
	}
}

func TestPostgresDLQStore_PopNext_Empty(t *testing.T) {
	db := setupTestDB(t)
	dlq := NewPostgresDLQStore(db)

	result := dlq.PopNext()
	if result != nil {
		t.Errorf("PopNext on empty DLQ: got %v, want nil", result)
	}
}

func TestPostgresDLQStore_PopNext_MaxRetriesExceeded(t *testing.T) {
	db := setupTestDB(t)
	dlq := NewPostgresDLQStore(db)

	dlq.Push(&UnmatchedOrder{
		OrderID:          "order-exhausted",
		EstablishmentLat: -23.5,
		EstablishmentLng: -46.6,
		CreatedAt:        time.Now().UnixMilli() - 120000,
		RetryCount:       3,
		LastAttemptAt:    time.Now().UnixMilli() - 120000,
	})

	result := dlq.PopNext()
	if result != nil {
		t.Errorf("PopNext should skip exhausted orders, got %v", result)
	}
	if dlq.Len() != 1 {
		t.Errorf("DLQ should still have exhausted order: Len=%d, want 1", dlq.Len())
	}
}

func TestPostgresDLQStore_PopNext_RecentAttempt(t *testing.T) {
	db := setupTestDB(t)
	dlq := NewPostgresDLQStore(db)

	dlq.Push(&UnmatchedOrder{
		OrderID:          "order-recent",
		EstablishmentLat: -23.5,
		EstablishmentLng: -46.6,
		CreatedAt:        time.Now().UnixMilli() - 120000,
		RetryCount:       0,
		LastAttemptAt:    time.Now().UnixMilli(), // just attempted
	})

	result := dlq.PopNext()
	if result != nil {
		t.Errorf("PopNext should skip recently attempted orders, got %v", result)
	}
}

func TestPostgresDLQStore_PopNext_ReadyForRetry(t *testing.T) {
	db := setupTestDB(t)
	dlq := NewPostgresDLQStore(db)

	oldTime := time.Now().UnixMilli() - 60000 // 60s ago
	dlq.Push(&UnmatchedOrder{
		OrderID:          "order-old",
		EstablishmentLat: -23.5,
		EstablishmentLng: -46.6,
		CreatedAt:        oldTime,
		RetryCount:       1,
		LastAttemptAt:    oldTime,
	})

	result := dlq.PopNext()
	if result == nil {
		t.Fatal("PopNext should return an order ready for retry")
	}
	if result.OrderID != "order-old" {
		t.Errorf("PopNext wrong order: got %s, want order-old", result.OrderID)
	}
	if result.RetryCount != 1 {
		t.Errorf("PopNext wrong retry count: got %d, want 1", result.RetryCount)
	}
	if dlq.Len() != 0 {
		t.Errorf("DLQ should be empty after PopNext: Len=%d", dlq.Len())
	}
}

func TestPostgresDLQStore_PopNext_FIFO(t *testing.T) {
	db := setupTestDB(t)
	dlq := NewPostgresDLQStore(db)

	oldTime := time.Now().UnixMilli() - 60000
	dlq.Push(&UnmatchedOrder{OrderID: "order-first", EstablishmentLat: -23.5, EstablishmentLng: -46.6, CreatedAt: oldTime - 1000, RetryCount: 0, LastAttemptAt: oldTime})
	dlq.Push(&UnmatchedOrder{OrderID: "order-second", EstablishmentLat: -23.5, EstablishmentLng: -46.6, CreatedAt: oldTime, RetryCount: 0, LastAttemptAt: oldTime})

	result := dlq.PopNext()
	if result == nil || result.OrderID != "order-first" {
		t.Errorf("PopNext should return oldest first: got %v", result)
	}
}

func TestPostgresDLQStore_List(t *testing.T) {
	db := setupTestDB(t)
	dlq := NewPostgresDLQStore(db)

	dlq.Push(&UnmatchedOrder{OrderID: "o1", EstablishmentLat: -23.5, EstablishmentLng: -46.6, CreatedAt: time.Now().UnixMilli(), LastAttemptAt: time.Now().UnixMilli()})
	dlq.Push(&UnmatchedOrder{OrderID: "o2", EstablishmentLat: -23.5, EstablishmentLng: -46.6, CreatedAt: time.Now().UnixMilli(), LastAttemptAt: time.Now().UnixMilli()})

	list := dlq.List()
	if len(list) != 2 {
		t.Errorf("List len: got %d, want 2", len(list))
	}
}

func TestPostgresDLQStore_Cleanup(t *testing.T) {
	db := setupTestDB(t)
	dlq := NewPostgresDLQStore(db)

	// Insere um registro antigo (>5min atrás)
	oldTime := time.Now().Add(-10 * time.Minute).UnixMilli()
	dlq.Push(&UnmatchedOrder{OrderID: "order-old", EstablishmentLat: -23.5, EstablishmentLng: -46.6, CreatedAt: oldTime, LastAttemptAt: oldTime})

	// Insere um registro recente
	dlq.Push(&UnmatchedOrder{OrderID: "order-new", EstablishmentLat: -23.5, EstablishmentLng: -46.6, CreatedAt: time.Now().UnixMilli(), LastAttemptAt: time.Now().UnixMilli()})

	dlq.Cleanup(5 * time.Minute)

	if dlq.Len() != 1 {
		t.Errorf("After cleanup: got %d orders, want 1", dlq.Len())
	}

	remaining := dlq.List()
	if len(remaining) != 1 || remaining[0].OrderID != "order-new" {
		t.Errorf("Wrong order survived cleanup: got %v", remaining)
	}
}

func TestPostgresDLQStore_RetryCountIncrement(t *testing.T) {
	db := setupTestDB(t)
	dlq := NewPostgresDLQStore(db)

	// Push order with retry_count=2, old last_attempt
	dlq.Push(&UnmatchedOrder{
		OrderID:          "order-retry",
		EstablishmentLat: -23.5,
		EstablishmentLng: -46.6,
		CreatedAt:        time.Now().UnixMilli() - 120000,
		RetryCount:       2,
		LastAttemptAt:    time.Now().UnixMilli() - 60000,
	})

	// PopNext should return it (retry_count < 3)
	result := dlq.PopNext()
	if result == nil {
		t.Fatal("PopNext should return order with retry_count=2")
	}
	if result.RetryCount != 2 {
		t.Errorf("RetryCount: got %d, want 2", result.RetryCount)
	}

	// Push it back with retry_count=3
	dlq.Push(&UnmatchedOrder{
		OrderID:          result.OrderID,
		EstablishmentLat: result.EstablishmentLat,
		EstablishmentLng: result.EstablishmentLng,
		CreatedAt:        result.CreatedAt,
		RetryCount:       result.RetryCount + 1,
		LastAttemptAt:    time.Now().UnixMilli(),
	})

	// PopNext should NOT return it (retry_count >= 3)
	result2 := dlq.PopNext()
	if result2 != nil {
		t.Errorf("PopNext should skip exhausted order, got %v", result2)
	}
}

// TestMatchingEngine_DLQSobreviveARestart cobre o que a troca em
// cmd/fuudelivery/main.go garante: com a DLQ persistente, um pedido que não
// achou entregador continua lá depois de o processo morrer e subir de novo.
//
// Este é o teste da LIGAÇÃO, não do componente. A PostgresDLQStore já tinha
// testes; o que nunca teve teste foi o MatchingEngine de fato usá-la — e por
// isso o main.go ficou apontando para a DLQ in-memory sem ninguém notar.
//
// Falsificação: trocar NewMatchingEngineWithDLQ por NewMatchingEngine faz o
// segundo engine nascer com uma DLQ vazia e o teste quebra na hora.
func TestMatchingEngine_DLQSobreviveARestart(t *testing.T) {
	db := setupTestDB(t)

	pedido := &UnmatchedOrder{
		OrderID:          "pedido-que-nao-pode-sumir",
		EstablishmentLat: -23.5505,
		EstablishmentLng: -46.6333,
		ZoneID:           1,
		CreatedAt:        time.Now().UnixMilli(),
		RetryCount:       0,
		// Passado o suficiente para o PopNext (que exige 30s entre tentativas)
		// considerar o pedido pronto para retry.
		LastAttemptAt: time.Now().UnixMilli() - 60000,
	}

	// --- processo 1: o pedido não casa e vai para a DLQ ---
	engine1 := NewMatchingEngineWithDLQ(NewCourierStore(), nil, NewPostgresDLQStore(db))
	engine1.DLQ.Push(pedido)

	if got := engine1.DLQ.Len(); got != 1 {
		t.Fatalf("antes do restart: esperava 1 pedido na DLQ, obtive %d", got)
	}

	// --- o processo morre e sobe de novo (novo engine, nova store, mesmo banco) ---
	engine2 := NewMatchingEngineWithDLQ(NewCourierStore(), nil, NewPostgresDLQStore(db))

	if got := engine2.DLQ.Len(); got != 1 {
		t.Fatalf("depois do restart: o pedido sumiu da DLQ (Len=%d). "+
			"Com a DLQ in-memory é exatamente isto que acontece em produção "+
			"toda vez que o Render derruba o processo por inatividade.", got)
	}

	recuperado := engine2.DLQ.PopNext()
	if recuperado == nil {
		t.Fatal("depois do restart: PopNext devolveu nil — o pedido não é recuperável")
	}
	if recuperado.OrderID != pedido.OrderID {
		t.Fatalf("depois do restart: recuperei %q, esperava %q", recuperado.OrderID, pedido.OrderID)
	}
}
