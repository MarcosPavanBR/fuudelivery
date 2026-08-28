package gateway

import (
	"sync"
	"time"
)

// CircuitState representa o estado do circuit breaker.
type CircuitState int

const (
	// StateClosed é o estado normal: todas as requisições passam.
	StateClosed CircuitState = iota

	// StateOpen é o estado de falha: requisições são rejeitadas.
	StateOpen

	// StateHalfOpen é o estado de teste: uma requisição é permitida.
	StateHalfOpen
)

// String retorna a representação em texto do estado.
func (s CircuitState) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half_open"
	default:
		return "unknown"
	}
}

// CircuitBreaker implementa o padrão Circuit Breaker para proteger contra
// falhas em cascade. Se um gateway falhar N vezes em um período, o circuit
// breaker "abre" e rejeita requisições temporariamente.
//
// Após um período de cooldown, entra em half-open e permite uma única
// requisição de teste. Se sucesso, fecha o circuit. Se falha, reabre.
//
// Uso:
//
//	cb := NewCircuitBreaker(5, 1*time.Minute) // 5 falhas = open por 1min
//	if cb.IsOpen() {
//	    return ErrCircuitOpen
//	}
//	// ... fazer requisição ...
//	cb.RecordSuccess() // ou cb.RecordFailure()
type CircuitBreaker struct {
	mu           sync.Mutex
	state        CircuitState
	failCount    int
	threshold    int           // Falhas para abrir o circuit
	cooldown     time.Duration // Tempo para tentar half-open
	lastFailure  time.Time
	halfOpenUsed bool // Se já usou a requisição de teste no half-open
}

// NewCircuitBreaker cria um novo circuit breaker.
//
// Parameters:
//   - threshold: número de falhas consecutivas para abrir o circuit (recomendado: 5)
//   - cooldown: tempo para transicionar de open para half-open (recomendado: 1min)
func NewCircuitBreaker(threshold int, cooldown time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		state:     StateClosed,
		threshold: threshold,
		cooldown:  cooldown,
	}
}

// IsOpen retorna true se o circuit breaker estiver bloqueando requisições.
//
// Comportamento:
//   - StateClosed: sempre retorna false (requisições passam)
//   - StateOpen: retorna true se cooldown não expirou; se expirou, transiciona
//     para StateHalfOpen e retorna false (permite 1 requisição de teste)
//   - StateHalfOpen: retorna true se já usou a requisição de teste
func (cb *CircuitBreaker) IsOpen() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		return false

	case StateOpen:
		if time.Since(cb.lastFailure) > cb.cooldown {
			cb.state = StateHalfOpen
			cb.halfOpenUsed = false
			return false // permite 1 requisição de teste
		}
		return true

	case StateHalfOpen:
		if cb.halfOpenUsed {
			return true // já usou a requisição de teste
		}
		cb.halfOpenUsed = true
		return false // primeira requisição em half-open
	}

	return false
}

// RecordSuccess registra uma requisição bem-sucedida.
// No StateClosed: reseta contador de falhas.
// No StateHalfOpen: fecha o circuit (volta ao normal).
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failCount = 0
	cb.state = StateClosed
	cb.halfOpenUsed = false
}

// RecordFailure registra uma falha.
// No StateClosed: incrementa contador. Se >= threshold, abre o circuit.
// No StateHalfOpen: reabre o circuit imediatamente.
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failCount++
	cb.lastFailure = time.Now()

	switch cb.state {
	case StateClosed:
		if cb.failCount >= cb.threshold {
			cb.state = StateOpen
		}
	case StateHalfOpen:
		// Falha em half-open: reabre o circuit
		cb.state = StateOpen
		cb.halfOpenUsed = false
	}
}

// State retorna o estado atual do circuit breaker.
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}

// FailCount retorna o número atual de falhas consecutivas.
func (cb *CircuitBreaker) FailCount() int {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.failCount
}

// Reset reseta o circuit breaker para o estado inicial (closed).
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.state = StateClosed
	cb.failCount = 0
	cb.halfOpenUsed = false
}
