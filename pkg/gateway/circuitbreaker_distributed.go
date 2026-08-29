package gateway

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// DistributedCircuitBreaker implementa circuit breaker usando Redis como backend compartilhado
// Ideal para ambientes com múltiplas instâncias onde cada instância precisa ter o mesmo estado
type DistributedCircuitBreaker struct {
	client    *redis.Client
	gatewayID string
	threshold int
	cooldown  time.Duration
	ttl       time.Duration
}

// NewDistributedCircuitBreaker cria um novo circuit breaker distribuído
// Parameters:
//   - client: cliente Redis
//   - gatewayID: identificador único do gateway (ex: "pagarme", "asaas")
//   - threshold: número de falhas consecutivas para abrir o circuit (recomendado: 5)
//   - cooldown: tempo em estado open antes de tentar half-open (recomendado: 1min)
//   - ttl: tempo de vida das chaves no Redis (recomendado: 10min)
func NewDistributedCircuitBreaker(
	client *redis.Client,
	gatewayID string,
	threshold int,
	cooldown time.Duration,
	ttl time.Duration,
) *DistributedCircuitBreaker {
	return &DistributedCircuitBreaker{
		client:    client,
		gatewayID: gatewayID,
		threshold: threshold,
		cooldown:  cooldown,
		ttl:       ttl,
	}
}

// keys retorna as chaves Redis usadas por este circuit breaker
func (cb *DistributedCircuitBreaker) keys() (failCountKey, stateKey, lastFailureKey string) {
	prefix := "circuit_breaker:" + cb.gatewayID
	return prefix + ":fail_count", prefix + ":state", prefix + ":last_failure"
}

// IsOpen verifica se o circuit breaker está aberto
// Usa script Lua para garantir atomicidade na verificação e transição de estados
func (cb *DistributedCircuitBreaker) IsOpen(ctx context.Context) (bool, error) {
	failCountKey, stateKey, lastFailureKey := cb.keys()

	// Script Lua para verificar estado e transicionar se necessário
	script := `
local fail_count_key = KEYS[1]
local state_key = KEYS[2]
local last_failure_key = KEYS[3]
local cooldown = tonumber(ARGV[1])
local now = tonumber(ARGV[2])

-- Obtém estado atual
local state = redis.call('GET', state_key)
if not state then
	state = 'closed'
end

-- Se estiver open, verifica se cooldown expirou
if state == 'open' then
	local last_failure = tonumber(redis.call('GET', last_failure_key) or '0')
	if (now - last_failure) > cooldown then
		-- Transiciona para half-open
		redis.call('SET', state_key, 'half_open')
		redis.call('EXPIRE', state_key, ARGV[3])
		return 'half_open'
	end
	return 'open'
end

-- Se estiver half-open, verifica se já foi usado
if state == 'half_open' then
	return 'half_open'
end

-- StateClosed
return 'closed'
`

	now := time.Now().Unix()
	ttlSeconds := int(cb.ttl.Seconds())

	result, err := cb.client.Eval(ctx, script, []string{failCountKey, stateKey, lastFailureKey}, cb.cooldown.Seconds(), now, ttlSeconds).Result()
	if err != nil {
		if err == redis.Nil {
			return false, nil
		}
		return false, fmt.Errorf("failed to check circuit breaker state: %w", err)
	}

	state, ok := result.(string)
	if !ok {
		return false, fmt.Errorf("unexpected circuit breaker state type: %T", result)
	}

	return state == "open" || state == "half_open_used", nil
}

// RecordSuccess registra sucesso e fecha o circuit breaker
func (cb *DistributedCircuitBreaker) RecordSuccess(ctx context.Context) error {
	failCountKey, stateKey, lastFailureKey := cb.keys()

	pipe := cb.client.Pipeline()
	pipe.Del(ctx, failCountKey)
	pipe.Set(ctx, stateKey, "closed", cb.ttl)
	pipe.Del(ctx, lastFailureKey)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to record success: %w", err)
	}

	return nil
}

// RecordFailure registra falha e pode abrir o circuit breaker
func (cb *DistributedCircuitBreaker) RecordFailure(ctx context.Context) error {
	failCountKey, stateKey, lastFailureKey := cb.keys()

	script := `
local fail_count_key = KEYS[1]
local state_key = KEYS[2]
local last_failure_key = KEYS[3]
local threshold = tonumber(ARGV[1])
local cooldown = tonumber(ARGV[2])
local ttl = tonumber(ARGV[3])
local now = tonumber(ARGV[4])

-- Incrementa contador de falhas
local fail_count = tonumber(redis.call('INCR', fail_count_key) or '0')
redis.call('EXPIRE', fail_count_key, ttl)

-- Registra timestamp da última falha
redis.call('SET', last_failure_key, now)
redis.call('EXPIRE', last_failure_key, ttl)

-- Se atingiu threshold, abre o circuit
if fail_count >= threshold then
	redis.call('SET', state_key, 'open')
	redis.call('EXPIRE', state_key, ttl)
	return 'open'
end

-- Verifica estado atual
local state = redis.call('GET', state_key)
if state == 'half_open' then
	-- Falha em half-open: reabre
	redis.call('SET', state_key, 'open')
	redis.call('EXPIRE', state_key, ttl)
	return 'open'
end

return 'closed'
`

	now := time.Now().Unix()
	ttlSeconds := int(cb.ttl.Seconds())

	result, err := cb.client.Eval(ctx, script, []string{failCountKey, stateKey, lastFailureKey}, cb.threshold, cb.cooldown.Seconds(), ttlSeconds, now).Result()
	if err != nil {
		return fmt.Errorf("failed to record failure: %w", err)
	}

	state, ok := result.(string)
	if !ok {
		return fmt.Errorf("unexpected circuit breaker state type: %T", result)
	}

	if state == "open" {
		log.Printf("[CircuitBreaker:%s] Circuit OPENED after %d failures", cb.gatewayID, cb.threshold)
	}

	return nil
}

// State retorna o estado atual do circuit breaker
func (cb *DistributedCircuitBreaker) State(ctx context.Context) (string, error) {
	_, stateKey, _ := cb.keys()

	state, err := cb.client.Get(ctx, stateKey).Result()
	if err != nil {
		if err == redis.Nil {
			return "closed", nil
		}
		return "", fmt.Errorf("failed to get circuit breaker state: %w", err)
	}

	return state, nil
}

// Reset reseta o circuit breaker para o estado inicial
func (cb *DistributedCircuitBreaker) Reset(ctx context.Context) error {
	failCountKey, stateKey, lastFailureKey := cb.keys()

	pipe := cb.client.Pipeline()
	pipe.Del(ctx, failCountKey)
	pipe.Del(ctx, stateKey)
	pipe.Del(ctx, lastFailureKey)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to reset circuit breaker: %w", err)
	}

	return nil
}

// FailCount retorna o número atual de falhas consecutivas
func (cb *DistributedCircuitBreaker) FailCount(ctx context.Context) (int, error) {
	failCountKey, _, _ := cb.keys()

	count, err := cb.client.Get(ctx, failCountKey).Int()
	if err != nil {
		if err == redis.Nil {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to get fail count: %w", err)
	}

	return count, nil
}

// AllowRequest verifica se uma requisição pode ser feita
// Retorna true se permitido, false se bloqueado pelo circuit breaker
func (cb *DistributedCircuitBreaker) AllowRequest(ctx context.Context) (bool, error) {
	isOpen, err := cb.IsOpen(ctx)
	if err != nil {
		return false, err
	}

	if isOpen {
		return false, nil
	}

	return true, nil
}

// ExecuteWithCircuitBreaker executa uma função com proteção do circuit breaker
func (cb *DistributedCircuitBreaker) ExecuteWithCircuitBreaker(
	ctx context.Context,
	fn func() error,
) error {
	allowed, err := cb.AllowRequest(ctx)
	if err != nil {
		return fmt.Errorf("circuit breaker check failed: %w", err)
	}

	if !allowed {
		return ErrCircuitOpen
	}

	err = fn()
	if err != nil {
		recordErr := cb.RecordFailure(ctx, err)
		if recordErr != nil {
			log.Printf("[CircuitBreaker:%s] Failed to record failure: %v", cb.gatewayID, recordErr)
		}
		return err
	}

	recordErr := cb.RecordSuccess(ctx)
	if recordErr != nil {
		log.Printf("[CircuitBreaker:%s] Failed to record success: %v", cb.gatewayID, recordErr)
	}

	return nil
}
