package gateway

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"
)

// ═══════════════════════════════════════════════════════════════
// ERROS
// ═══════════════════════════════════════════════════════════════

var (
	// ErrNoGatewayAvailable é retornado quando nenhum gateway disponível
	// suporta o método de pagamento solicitado.
	ErrNoGatewayAvailable = errors.New("gateway: no gateway available for this payment method")

	// ErrCircuitOpen é retornado quando o circuit breaker de todos os
	// gateways está aberto.
	ErrCircuitOpen = errors.New("gateway: all circuit breakers open")

	// ErrGatewayFailed é retornado quando todos os gateways falharam.
	ErrGatewayFailed = errors.New("gateway: all gateways failed")
)

// ═══════════════════════════════════════════════════════════════
// ROUTER
// ═══════════════════════════════════════════════════════════════

// RouterSelectionStrategy define a estratégia de seleção de gateway.
type RouterSelectionStrategy int

const (
	// StrategyOrdered usa a ordem de registro (primeiro = prioritário).
	StrategyOrdered RouterSelectionStrategy = iota

	// StrategyLowestLatency seleciona o gateway com menor latência média.
	StrategyLowestLatency

	// StrategyLowestCost seleciona o gateway com menor custo para o método.
	StrategyLowestCost
)

// RouterSelection é o resultado da seleção de gateway.
type RouterSelection struct {
	Gateway  Gateway
	Strategy string // Motivo da seleção: "ordered", "fallback", "circuit_breaker"
}

// Router roteia transações de pagamento para o melhor gateway disponível.
//
// Funcionamento:
//  1. Verifica circuit breaker de cada gateway
//  2. Filtra por suporte ao método de pagamento
//  3. Filtra por suporte a split (se necessário)
//  4. Seleciona o primeiro gateway que atende todos os critérios
//  5. Se falhar, tenta o próximo (fallback chain)
//
// Thread-safe: pode ser usado concorrentemente.
type Router struct {
	mu        sync.RWMutex
	gateways  []gatewayEntry
	strategy  RouterSelectionStrategy
	fallbacks []Gateway // Gateways de último recurso (ex: AbacatePay PIX)
}

type gatewayEntry struct {
	gateway Gateway
	cb      *CircuitBreaker
	weight  int // Para ordenação (menor = mais prioritário)
}

// NewRouter cria um novo router com a ordem de prioridade dos gateways.
//
// Exemplo:
//
//	router := gateway.NewRouter(pagarmeGW, asaasGW, abacatepayGW)
//	// pagarme é prioritário, asaas é alternativo, abacatepay é fallback
func NewRouter(gateways ...Gateway) *Router {
	r := &Router{
		strategy: StrategyOrdered,
	}

	for i, g := range gateways {
		r.gateways = append(r.gateways, gatewayEntry{
			gateway: g,
			cb:      NewCircuitBreaker(5, 1*time.Minute), // 5 falhas = 1min open
			weight:  i,
		})
	}

	return r
}

// SetStrategy define a estratégia de seleção de gateway.
func (r *Router) SetStrategy(s RouterSelectionStrategy) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.strategy = s
}

// Select retorna o melhor gateway disponível para a requisição.
//
// Critérios de seleção (em ordem):
//  1. Circuit breaker fechado (não bloqueando)
//  2. Suporta o método de pagamento
//  3. Suporta split (se req tem SplitRules)
//  4. Suporta pré-autorização (se cartão com Capture=false)
//
// Retorna ErrNoGatewayAvailable se nenhum gateway atende.
func (r *Router) Select(method PaymentMethod, requiresSplit bool, requiresPreAuth bool) (Gateway, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, entry := range r.gateways {
		// Pula se circuit breaker está aberto
		if entry.cb.IsOpen() {
			continue
		}

		// Pula se não suporta o método
		if !entry.gateway.SupportsMethod(method) {
			continue
		}

		// Pula se requer split mas gateway não suporta
		if requiresSplit && !entry.gateway.SupportsSplit() {
			continue
		}

		// Pula se requer pré-autorização mas gateway não suporta
		if requiresPreAuth && !entry.gateway.SupportsPreAuth() {
			continue
		}

		return entry.gateway, nil
	}

	return nil, ErrNoGatewayAvailable
}

// CreateTransactionWithFallback tenta criar transação com fallback automático.
//
// Tenta cada gateway na ordem de prioridade. Se um falhar, tenta o próximo.
// Registra sucesso/falha no circuit breaker de cada gateway.
func (r *Router) CreateTransactionWithFallback(
	ctx context.Context,
	req *TransactionRequest,
) (*TransactionResponse, error) {

	requiresSplit := len(req.SplitRules) > 0
	requiresPreAuth := req.PaymentMethod != MethodPIX && !req.Capture

	r.mu.RLock()
	entries := make([]gatewayEntry, len(r.gateways))
	copy(entries, r.gateways)
	r.mu.RUnlock()

	var lastErr error

	for _, entry := range entries {
		// Pula se circuit breaker aberto
		if entry.cb.IsOpen() {
			log.Printf("[ROUTER] Gateway %s skipped: circuit breaker open",
				entry.gateway.Name())
			continue
		}

		// Pula se não suporta o método
		if !entry.gateway.SupportsMethod(req.PaymentMethod) {
			continue
		}

		// Pula se requer split mas gateway não suporta
		if requiresSplit && !entry.gateway.SupportsSplit() {
			log.Printf("[ROUTER] Gateway %s skipped: no split support",
				entry.gateway.Name())
			continue
		}

		// Pula se requer pre-auth mas gateway não suporta
		if requiresPreAuth && !entry.gateway.SupportsPreAuth() {
			log.Printf("[ROUTER] Gateway %s skipped: no pre-auth support",
				entry.gateway.Name())
			continue
		}

		// Tenta criar transação com timeout
		log.Printf("[ROUTER] Trying gateway %s for method=%s split=%v",
			entry.gateway.Name(), req.PaymentMethod, requiresSplit)

		resp, err := entry.gateway.CreateTransaction(ctx, req)
		if err != nil {
			entry.cb.RecordFailure()
			lastErr = fmt.Errorf("gateway %s: %w", entry.gateway.Name(), err)
			log.Printf("[ROUTER] Gateway %s failed: %v", entry.gateway.Name(), err)
			continue // tenta próximo gateway
		}

		// Sucesso
		entry.cb.RecordSuccess()
		log.Printf("[ROUTER] Gateway %s succeeded: status=%s",
			entry.gateway.Name(), resp.Status)
		return resp, nil
	}

	if lastErr != nil {
		return nil, fmt.Errorf("%w: %v", ErrGatewayFailed, lastErr)
	}
	return nil, ErrNoGatewayAvailable
}

// RecordSuccess registra sucesso para o gateway especificado.
func (r *Router) RecordSuccess(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.gateways {
		if r.gateways[i].gateway.Name() == name {
			r.gateways[i].cb.RecordSuccess()
			return
		}
	}
}

// RecordFailure registra falha para o gateway especificado.
func (r *Router) RecordFailure(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.gateways {
		if r.gateways[i].gateway.Name() == name {
			r.gateways[i].cb.RecordFailure()
			return
		}
	}
}

// Gateways retorna a lista de gateways registrados (para monitoramento).
func (r *Router) Gateways() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, len(r.gateways))
	for i, entry := range r.gateways {
		names[i] = entry.gateway.Name()
	}
	return names
}

// CircuitBreakerState retorna o estado do circuit breaker de um gateway.
func (r *Router) CircuitBreakerState(name string) (CircuitState, int) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, entry := range r.gateways {
		if entry.gateway.Name() == name {
			return entry.cb.State(), entry.cb.FailCount()
		}
	}
	return StateClosed, 0
}

// ResetCircuitBreaker reseta o circuit breaker de um gateway específico.
// Útil para forçar retry manual após resolver um problema.
func (r *Router) ResetCircuitBreaker(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i := range r.gateways {
		if r.gateways[i].gateway.Name() == name {
			r.gateways[i].cb.Reset()
			log.Printf("[ROUTER] Circuit breaker reset for gateway %s", name)
			return
		}
	}
}
