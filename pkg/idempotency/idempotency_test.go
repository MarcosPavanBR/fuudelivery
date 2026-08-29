package idempotency

import (
"context"
"testing"
"time"

"github.com/redis/go-redis/v9"
)

func setupTestRedis() *redis.Client {
return redis.NewClient(&redis.Options{
Addr: "localhost:6379",
DB:   15, // Database de teste
})
}

func TestIdempotencyManager_GenerateKey(t *testing.T) {
manager := NewIdempotencyManager(nil, time.Minute)

key1 := manager.GenerateKey("payment", "order_123", "paid")
key2 := manager.GenerateKey("payment", "order_123", "paid")
key3 := manager.GenerateKey("payment", "order_456", "paid")

if key1 != key2 {
t.Error("Same input should generate same key")
}

if key1 == key3 {
t.Error("Different input should generate different key")
}
}

func TestIdempotencyManager_ProcessWithIdempotency(t *testing.T) {
client := setupTestRedis()
defer client.Close()

ctx := context.Background()
manager := NewIdempotencyManager(client, time.Minute)

key := manager.GenerateKey("test", "operation_1")

executionCount := 0
operation := func() ([]byte, error) {
executionCount++
return []byte("result"), nil
}

// Primeira execução
result1, cached1, err := manager.ProcessWithIdempotency(ctx, key, operation)
if err != nil {
t.Fatalf("First execution failed: %v", err)
}
if cached1 {
t.Error("First execution should not be cached")
}
if executionCount != 1 {
t.Error("Operation should execute once")
}

// Segunda execução (deve retornar cache)
result2, cached2, err := manager.ProcessWithIdempotency(ctx, key, operation)
if err != nil {
t.Fatalf("Second execution failed: %v", err)
}
if !cached2 {
t.Error("Second execution should be cached")
}
if executionCount != 1 {
t.Error("Operation should not execute again")
}

if string(result1) != string(result2) {
t.Error("Results should be equal")
}
}

func TestIdempotencyManager_IsProcessed(t *testing.T) {
client := setupTestRedis()
defer client.Close()

ctx := context.Background()
manager := NewIdempotencyManager(client, time.Minute)

key := "test:is_processed"

// Verifica que não foi processado
processed, err := manager.IsProcessed(ctx, key)
if err != nil {
t.Fatalf("Failed to check: %v", err)
}
if processed {
t.Error("Key should not be processed yet")
}

// Processa
acquired, err := manager.TryAcquire(ctx, key)
if err != nil {
t.Fatalf("Failed to acquire: %v", err)
}
if !acquired {
t.Error("Should acquire lock")
}

err = manager.StoreResult(ctx, key, []byte("test"))
if err != nil {
t.Fatalf("Failed to store: %v", err)
}

// Verifica que foi processado
processed, err = manager.IsProcessed(ctx, key)
if err != nil {
t.Fatalf("Failed to check: %v", err)
}
if !processed {
t.Error("Key should be processed")
}
}
