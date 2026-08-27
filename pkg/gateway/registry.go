package gateway

import (
	"fmt"
	"sync"
)

// ═══════════════════════════════════════════════════════════════
// REGISTRY
// ═══════════════════════════════════════════════════════════════

// Registry gerencia o registro de gateways disponíveis no sistema.
// É usado no startup do monolito para registrar todos os gateways
// habilitados via feature flags (PAYMENT_GATEWAY_PRIMARY, etc.)
//
// Thread-safe: pode ser usado concorrentemente.
//
// Uso no main.go:
//
//	reg := gateway.NewRegistry()
//	if os.Getenv("PAGARME_API_KEY") != "" {
//	    reg.Register(pagarme.NewGateway(...))
//	}
//	if os.Getenv("ASAAS_API_KEY") != "" {
//	    reg.Register(asaas.NewGateway(...))
//	}
//	if os.Getenv("ABACATE_PAY_API_KEY") != "" {
//	    reg.Register(abacatepay.NewGateway(...))
//	}
type Registry struct {
	mu       sync.RWMutex
	gateways map[string]Gateway
	primary  string
	fallback string
}

// NewRegistry cria um novo registry vazio.
func NewRegistry() *Registry {
	return &Registry{
		gateways: make(map[string]Gateway),
	}
}

// Register registra um gateway no registry.
// Se um gateway com o mesmo nome já existir, sobrescreve.
//
// Parameters:
//   - g: implementação do gateway a ser registrada
//
// Retorna error se o gateway for nil.
func (r *Registry) Register(g Gateway) error {
	if g == nil {
		return fmt.Errorf("gateway: cannot register nil gateway")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	name := g.Name()
	r.gateways[name] = g

	return nil
}

// Get retorna um gateway pelo nome.
//
// Retorna error se o gateway não estiver registrado.
func (r *Registry) Get(name string) (Gateway, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	g, ok := r.gateways[name]
	if !ok {
		return nil, fmt.Errorf("gateway: %q not registered", name)
	}
	return g, nil
}

// MustGet retorna um gateway pelo nome ou panic.
// Usado apenas em testes e configurações garantidas.
func (r *Registry) MustGet(name string) Gateway {
	g, err := r.Get(name)
	if err != nil {
		panic(fmt.Sprintf("gateway: %q not registered: %v", name, err))
	}
	return g
}

// SetPrimary define o gateway primário (usado pelo Router).
func (r *Registry) SetPrimary(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.gateways[name]; !ok {
		return fmt.Errorf("gateway: %q not registered, cannot set as primary", name)
	}

	r.primary = name
	return nil
}

// SetFallback define o gateway de fallback (usado pelo Router).
func (r *Registry) SetFallback(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.gateways[name]; !ok {
		return fmt.Errorf("gateway: %q not registered, cannot set as fallback", name)
	}

	r.fallback = name
	return nil
}

// Primary retorna o nome do gateway primário.
func (r *Registry) Primary() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.primary
}

// Fallback retorna o nome do gateway de fallback.
func (r *Registry) Fallback() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.fallback
}

// All retorna todos os gateways registrados.
func (r *Registry) All() map[string]Gateway {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]Gateway, len(r.gateways))
	for k, v := range r.gateways {
		result[k] = v
	}
	return result
}

// Names retorna os nomes de todos os gateways registrados.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.gateways))
	for name := range r.gateways {
		names = append(names, name)
	}
	return names
}

// Count retorna o número de gateways registrados.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.gateways)
}

// Remove remove um gateway do registry.
func (r *Registry) Remove(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.gateways, name)
}

// ═══════════════════════════════════════════════════════════════
// CONVENIÊNCIA: criar Router a partir do Registry
// ═══════════════════════════════════════════════════════════════

// NewRouterFromRegistry cria um Router usando a ordem:
// 1. Gateway primário (se definido)
// 2. Todos os outros gateways (exceto primário e fallback)
// 3. Gateway de fallback (se definido)
func (r *Registry) NewRouterFromRegistry() (*Router, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var ordered []Gateway

	// 1. Primário primeiro
	if r.primary != "" {
		if g, ok := r.gateways[r.primary]; ok {
			ordered = append(ordered, g)
		}
	}

	// 2. Outros (exceto primário e fallback)
	for name, g := range r.gateways {
		if name == r.primary || name == r.fallback {
			continue
		}
		ordered = append(ordered, g)
	}

	// 3. Fallback por último
	if r.fallback != "" {
		if g, ok := r.gateways[r.fallback]; ok {
			ordered = append(ordered, g)
		}
	}

	if len(ordered) == 0 {
		return nil, fmt.Errorf("gateway: no gateways registered")
	}

	return NewRouter(ordered...), nil
}
