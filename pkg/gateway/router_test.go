package gateway

import (
	"context"
	"errors"
	"testing"
	"time"
)

// ============================================================================
// Testes do Router — o núcleo do roteamento de pagamento.
//
// Antes destes testes, pkg/gateway tinha cobertura APENAS de money.go: o
// router e o circuit breaker, que decidem por onde o dinheiro passa, não
// tinham um único teste.
//
// fakeGateway implementa a interface Gateway inteira para permitir testar
// seleção/fallback sem tocar em nenhuma API externa.
// ============================================================================

type fakeGateway struct {
	name         string
	methods      map[PaymentMethod]bool
	split        bool
	preAuth      bool
	err          error // se != nil, CreateTransaction falha
	createCalls  int
	lastRequest  *TransactionRequest
	returnStatus TransactionStatus
}

func newFakeGateway(name string, methods ...PaymentMethod) *fakeGateway {
	m := make(map[PaymentMethod]bool, len(methods))
	for _, method := range methods {
		m[method] = true
	}
	return &fakeGateway{name: name, methods: m, returnStatus: StatusPaid}
}

func (f *fakeGateway) Name() string { return f.name }

func (f *fakeGateway) CreateTransaction(ctx context.Context, req *TransactionRequest) (*TransactionResponse, error) {
	f.createCalls++
	f.lastRequest = req
	if f.err != nil {
		return nil, f.err
	}
	return &TransactionResponse{Status: f.returnStatus, Gateway: f.name}, nil
}

func (f *fakeGateway) CaptureTransaction(ctx context.Context, gatewayID string, amount int64) error {
	return nil
}
func (f *fakeGateway) RefundTransaction(ctx context.Context, gatewayID string, amount int64) (*RefundResponse, error) {
	return &RefundResponse{}, nil
}
func (f *fakeGateway) VoidTransaction(ctx context.Context, gatewayID string) error { return nil }
func (f *fakeGateway) GetTransactionStatus(ctx context.Context, gatewayID string) (TransactionStatus, error) {
	return StatusPaid, nil
}
func (f *fakeGateway) CreateRecipient(ctx context.Context, req *RecipientRequest) (*RecipientResponse, error) {
	return &RecipientResponse{}, nil
}
func (f *fakeGateway) UpdateRecipient(ctx context.Context, recipientID string, req *RecipientRequest) error {
	return nil
}
func (f *fakeGateway) GetRecipientBalance(ctx context.Context, recipientID string) (int64, int64, error) {
	return 0, 0, nil
}
func (f *fakeGateway) ValidateWebhook(body []byte, headers map[string]string) bool { return true }
func (f *fakeGateway) ParseWebhook(body []byte) (*WebhookEvent, error)             { return &WebhookEvent{}, nil }
func (f *fakeGateway) SupportsMethod(method PaymentMethod) bool                    { return f.methods[method] }
func (f *fakeGateway) SupportsSplit() bool                                         { return f.split }
func (f *fakeGateway) SupportsPreAuth() bool                                       { return f.preAuth }
func (f *fakeGateway) Supports3DS() bool                                           { return false }
func (f *fakeGateway) SupportsEscrow() bool                                        { return false }
func (f *fakeGateway) MaxSplitRecipients() int                                     { return 10 }

// pixRequest monta uma cobrança PIX simples (sem split, com captura).
func pixRequest() *TransactionRequest {
	return &TransactionRequest{PaymentMethod: MethodPIX, Amount: 1999, Capture: true}
}

func TestRouter_UsaPrimeiroGatewayElegivel(t *testing.T) {
	primary := newFakeGateway("primary", MethodPIX)
	secondary := newFakeGateway("secondary", MethodPIX)
	r := NewRouter(primary, secondary)

	resp, err := r.CreateTransactionWithFallback(context.Background(), pixRequest())
	if err != nil {
		t.Fatalf("esperava sucesso, veio erro: %v", err)
	}
	if resp.Gateway != "primary" {
		t.Errorf("esperava o primeiro gateway da ordem, veio %s", resp.Gateway)
	}
	if secondary.createCalls != 0 {
		t.Error("segundo gateway não devia ser chamado quando o primeiro funciona")
	}
}

func TestRouter_FazFallbackQuandoPrimeiroFalha(t *testing.T) {
	primary := newFakeGateway("primary", MethodPIX)
	primary.err = errors.New("gateway fora do ar")
	secondary := newFakeGateway("secondary", MethodPIX)
	r := NewRouter(primary, secondary)

	resp, err := r.CreateTransactionWithFallback(context.Background(), pixRequest())
	if err != nil {
		t.Fatalf("esperava fallback bem-sucedido, veio erro: %v", err)
	}
	if resp.Gateway != "secondary" {
		t.Errorf("esperava fallback para secondary, veio %s", resp.Gateway)
	}
	if primary.createCalls != 1 {
		t.Errorf("primary devia ter sido tentado uma vez, foi %d", primary.createCalls)
	}
}

func TestRouter_PulaGatewayQueNaoSuportaOMetodo(t *testing.T) {
	pixOnly := newFakeGateway("pix-only", MethodPIX)
	cardCapable := newFakeGateway("card", MethodCreditCard)
	r := NewRouter(pixOnly, cardCapable)

	req := &TransactionRequest{PaymentMethod: MethodCreditCard, Amount: 5000, Capture: true}
	resp, err := r.CreateTransactionWithFallback(context.Background(), req)
	if err != nil {
		t.Fatalf("esperava sucesso no gateway de cartão: %v", err)
	}
	if resp.Gateway != "card" {
		t.Errorf("esperava gateway de cartão, veio %s", resp.Gateway)
	}
	if pixOnly.createCalls != 0 {
		t.Error("gateway que não suporta o método não pode ser chamado")
	}
}

func TestRouter_PulaGatewaySemSuporteASplit(t *testing.T) {
	noSplit := newFakeGateway("no-split", MethodPIX)
	withSplit := newFakeGateway("with-split", MethodPIX)
	withSplit.split = true
	r := NewRouter(noSplit, withSplit)

	req := pixRequest()
	req.SplitRules = []SplitRule{{RecipientID: "rec_1", FixedValue: 500}}

	resp, err := r.CreateTransactionWithFallback(context.Background(), req)
	if err != nil {
		t.Fatalf("esperava sucesso no gateway com split: %v", err)
	}
	if resp.Gateway != "with-split" {
		t.Errorf("esperava with-split, veio %s", resp.Gateway)
	}
	if noSplit.createCalls != 0 {
		t.Error("gateway sem split não pode receber cobrança com SplitRules")
	}
}

func TestRouter_SemGatewayElegivelRetornaErroDedicado(t *testing.T) {
	pixOnly := newFakeGateway("pix-only", MethodPIX)
	r := NewRouter(pixOnly)

	req := &TransactionRequest{PaymentMethod: MethodCreditCard, Amount: 5000, Capture: true}
	_, err := r.CreateTransactionWithFallback(context.Background(), req)

	if !errors.Is(err, ErrNoGatewayAvailable) {
		t.Errorf("esperava ErrNoGatewayAvailable, veio %v", err)
	}
}

func TestRouter_TodosFalhamRetornaErroDeFalha(t *testing.T) {
	a := newFakeGateway("a", MethodPIX)
	a.err = errors.New("falhou a")
	b := newFakeGateway("b", MethodPIX)
	b.err = errors.New("falhou b")
	r := NewRouter(a, b)

	_, err := r.CreateTransactionWithFallback(context.Background(), pixRequest())
	if !errors.Is(err, ErrGatewayFailed) {
		t.Errorf("esperava ErrGatewayFailed quando todos falham, veio %v", err)
	}
	if a.createCalls != 1 || b.createCalls != 1 {
		t.Errorf("os dois deviam ter sido tentados: a=%d b=%d", a.createCalls, b.createCalls)
	}
}

func TestRouter_PulaGatewayComCircuitoAberto(t *testing.T) {
	flaky := newFakeGateway("flaky", MethodPIX)
	flaky.err = errors.New("instável")
	backup := newFakeGateway("backup", MethodPIX)
	r := NewRouter(flaky, backup)

	// 5 falhas abrem o circuito do flaky (NewCircuitBreaker(5, 1min)).
	for i := 0; i < 5; i++ {
		if _, err := r.CreateTransactionWithFallback(context.Background(), pixRequest()); err != nil {
			t.Fatalf("fallback devia ter salvado a chamada %d: %v", i, err)
		}
	}
	callsAntes := flaky.createCalls

	// Próxima chamada: circuito aberto, flaky nem deve ser tentado.
	if _, err := r.CreateTransactionWithFallback(context.Background(), pixRequest()); err != nil {
		t.Fatalf("esperava sucesso via backup: %v", err)
	}
	if flaky.createCalls != callsAntes {
		t.Errorf("gateway com circuito aberto não pode ser chamado (antes=%d depois=%d)",
			callsAntes, flaky.createCalls)
	}
}

func TestRouter_PreencheIdempotencyKeyQuandoAusente(t *testing.T) {
	gw := newFakeGateway("gw", MethodPIX)
	r := NewRouter(gw)

	req := pixRequest()
	req.IdempotencyKey = ""

	if _, err := r.CreateTransactionWithFallback(context.Background(), req); err != nil {
		t.Fatalf("esperava sucesso: %v", err)
	}
	if gw.lastRequest.IdempotencyKey == "" {
		t.Error("router deve gerar IdempotencyKey quando o chamador não manda — é o que evita cobrança duplicada em retry")
	}
}

// TestRouter_GatewayNilNaoDerrubaACadeia é a trava de regressão do bug do
// typed-nil: cmd/fuudelivery/main.go descartava o erro dos construtores
// (`gw, _ := abacatepay.NewGateway()`) e, sem credencial, passava um ponteiro
// nil para NewRouter. Como o parâmetro é a interface Gateway, o valor vira um
// "typed nil" (tipo != nil, valor == nil) e passa por qualquer `if gw != nil`
// — mas estoura ao desreferenciar o client interno, abortando toda a cadeia
// de fallback.
//
// Este teste prova as duas metades: que o typed nil realmente engana a
// checagem de nil, e que um gateway que entra em panic derruba a chamada. A
// correção vive no chamador (buildPaymentGateways checa o erro ANTES de
// converter para interface), então o que travamos aqui é o entendimento do
// mecanismo.
func TestRouter_GatewayNilNaoDerrubaACadeia(t *testing.T) {
	t.Run("typed nil engana a checagem de nil", func(t *testing.T) {
		var concreto *fakeGateway // nil
		var iface Gateway = concreto

		if concreto != nil {
			t.Fatal("o ponteiro concreto é nil")
		}
		if iface == nil {
			t.Fatal("ESTE é o ponto: a interface NÃO é nil mesmo guardando um ponteiro nil — " +
				"por isso a checagem tem que ser no erro do construtor, antes da conversão")
		}
	})

	t.Run("gateway que entra em panic aborta a chamada", func(t *testing.T) {
		// panicGateway simula o adapter real desreferenciando g.client nil.
		panicky := &panicGateway{fakeGateway: *newFakeGateway("panicky", MethodPIX)}
		backup := newFakeGateway("backup", MethodPIX)
		r := NewRouter(panicky, backup)

		defer func() {
			if rec := recover(); rec == nil {
				t.Error("esperava panic propagando — o router não tem recover interno, " +
					"então gateway inválido NÃO pode chegar até aqui")
			}
			if backup.createCalls != 0 {
				t.Error("o backup nunca é alcançado: o panic aborta a cadeia inteira")
			}
		}()

		_, _ = r.CreateTransactionWithFallback(context.Background(), pixRequest())
	})
}

// panicGateway reproduz o comportamento do adapter com client nil.
type panicGateway struct{ fakeGateway }

func (p *panicGateway) CreateTransaction(ctx context.Context, req *TransactionRequest) (*TransactionResponse, error) {
	panic("runtime error: invalid memory address or nil pointer dereference")
}

// ============================================================================
// Circuit breaker
// ============================================================================

func TestCircuitBreaker_AbreDepoisDoLimiteDeFalhas(t *testing.T) {
	cb := NewCircuitBreaker(3, time.Minute)

	for i := 0; i < 2; i++ {
		cb.RecordFailure()
		if cb.IsOpen() {
			t.Fatalf("circuito não devia abrir com %d falhas (limite 3)", i+1)
		}
	}

	cb.RecordFailure() // 3ª
	if !cb.IsOpen() {
		t.Error("circuito devia abrir ao atingir o limite de falhas")
	}
}

func TestCircuitBreaker_SucessoFechaEZeraContagem(t *testing.T) {
	cb := NewCircuitBreaker(3, time.Minute)

	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordSuccess() // zera

	cb.RecordFailure()
	cb.RecordFailure()
	if cb.IsOpen() {
		t.Error("sucesso deve zerar a contagem — 2 falhas depois dele não podem abrir o circuito de limite 3")
	}
}

func TestCircuitBreaker_MeioAbertoDepoisDoTimeout(t *testing.T) {
	// Timeout curtíssimo para não deixar o teste lento.
	cb := NewCircuitBreaker(1, 10*time.Millisecond)

	cb.RecordFailure()
	if !cb.IsOpen() {
		t.Fatal("circuito devia estar aberto")
	}

	time.Sleep(20 * time.Millisecond)
	if cb.IsOpen() {
		t.Error("passado o timeout, o circuito deve permitir a requisição de teste (half-open)")
	}
}
