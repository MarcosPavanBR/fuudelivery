package idempotency

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// IdempotencyManager gerencia chaves de idempotência no Redis
type IdempotencyManager struct {
	client *redis.Client
	ttl    time.Duration
}

// IdempotencyResult representa o resultado de uma operação idempotente
type IdempotencyResult struct {
	Key       string
	Processed bool
	Data      []byte
	CreatedAt time.Time
}

// NewIdempotencyManager cria um novo gerenciador de idempotência
func NewIdempotencyManager(client *redis.Client, ttl time.Duration) *IdempotencyManager {
	return &IdempotencyManager{
		client: client,
		ttl:    ttl,
	}
}

// GenerateKey gera uma chave de idempotência única baseada no payload
func (m *IdempotencyManager) GenerateKey(prefix string, data ...string) string {
	hash := sha256.New()
	for _, d := range data {
		hash.Write([]byte(d))
	}
	return fmt.Sprintf("%s:%s", prefix, hex.EncodeToString(hash.Sum(nil)))
}

// TryAcquire tenta adquirir um lock de idempotência
// Retorna true se esta é a primeira vez que a chave está sendo processada
// Retorna false se a chave já foi processada (operaçãoduplicada)
func (m *IdempotencyManager) TryAcquire(ctx context.Context, key string) (bool, error) {
	acquired, err := m.client.SetNX(ctx, key, "processing", m.ttl).Result()
	if err != nil {
		return false, fmt.Errorf("failed to acquire idempotency lock: %w", err)
	}
	return acquired, nil
}

// StoreResult armazena o resultado de uma operação processada
func (m *IdempotencyManager) StoreResult(ctx context.Context, key string, result []byte) error {
	pipe := m.client.Pipeline()
	
	// Armazena o resultado
	pipe.Set(ctx, key+":result", result, m.ttl)
	
	// Marca como processado
	pipe.Set(ctx, key, "processed", m.ttl)
	
	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to store idempotency result: %w", err)
	}
	
	return nil
}

// GetResult recupera o resultado de uma operação já processada
func (m *IdempotencyManager) GetResult(ctx context.Context, key string) ([]byte, error) {
	data, err := m.client.Get(ctx, key+":result").Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get idempotency result: %w", err)
	}
	return data, nil
}

// IsProcessed verifica se uma chave já foi processada
func (m *IdempotencyManager) IsProcessed(ctx context.Context, key string) (bool, error) {
	status, err := m.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return false, nil
		}
		return false, fmt.Errorf("failed to check idempotency status: %w", err)
	}
	return status == "processed", nil
}

// ProcessWithIdempotency executa uma operação com garantia de idempotência
// Se a operação já foi executada antes, retorna o resultado cachedo
func (m *IdempotencyManager) ProcessWithIdempotency(
	ctx context.Context,
	key string,
	operation func() ([]byte, error),
) ([]byte, bool, error) {
	// Verifica se já foi processado
	processed, err := m.IsProcessed(ctx, key)
	if err != nil {
		return nil, false, err
	}
	
	if processed {
		// Retorna resultado cachedo
		result, err := m.GetResult(ctx, key)
		if err != nil {
			return nil, false, err
		}
		return result, true, nil
	}
	
	// Tenta adquirir lock
	acquired, err := m.TryAcquire(ctx, key)
	if err != nil {
		return nil, false, err
	}
	
	if !acquired {
		// Outra instância está processando, espera e verifica novamente
		time.Sleep(100 * time.Millisecond)
		return m.ProcessWithIdempotency(ctx, key, operation)
	}
	
	// Executa a operação
	result, err := operation()
	if err != nil {
		// Libera o lock em caso de erro
		m.client.Del(ctx, key)
		return nil, false, err
	}
	
	// Armazena o resultado
	if err := m.StoreResult(ctx, key, result); err != nil {
		return nil, false, err
	}
	
	return result, false, nil
}

// Release libera um lock de idempotência (em caso de erro)
func (m *IdempotencyManager) Release(ctx context.Context, key string) error {
	err := m.client.Del(ctx, key).Err()
	if err != nil {
		return fmt.Errorf("failed to release idempotency lock: %w", err)
	}
	return nil
}

// Cleanup limpa chaves expiradas (pode ser chamado periodicamente)
func (m *IdempotencyManager) Cleanup(ctx context.Context, pattern string) (int64, error) {
	var cursor uint64
	var count int64
	
	for {
		keys, nextCursor, err := m.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return count, fmt.Errorf("failed to scan keys: %w", err)
		}
		
		for _, key := range keys {
			ttl, err := m.client.TTL(ctx, key).Result()
			if err != nil {
				continue
			}
			
			if ttl < 0 {
				// Chave sem TTL, define TTL padrão
				m.client.Expire(ctx, key, m.ttl)
			}
		}
		
		count += int64(len(keys))
		
		if nextCursor == 0 {
			break
		}
		cursor = nextCursor
	}
	
	return count, nil
}
