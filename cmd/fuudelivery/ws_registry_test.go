package main

import (
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

// closableWriter é um fakeWriter que registra o Close (despejo de aba antiga).
type closableWriter struct {
	fakeWriter
	closed bool
}

func (c *closableWriter) Close() error { c.closed = true; return nil }

func resetWSClients(t *testing.T) {
	t.Helper()
	wsClientsMu.Lock()
	prev := wsClients
	wsClients = make(map[wsKey][]*safeConn)
	wsClientsMu.Unlock()
	t.Cleanup(func() {
		wsClientsMu.Lock()
		wsClients = prev
		wsClientsMu.Unlock()
	})
}

func TestWSKeyForClaims(t *testing.T) {
	casos := []struct {
		nome   string
		claims jwt.MapClaims
		url    string
		quer   wsKey
		ok     bool
	}{
		{"usuário 3 da loja 7 escuta a loja 7", jwt.MapClaims{"id": float64(3), "role": "user", "account_type": "user", "establishment_id": float64(7)}, "3", wsKey{wsKindEstablishment, 7}, true},
		{"cliente 7 escuta só o próprio lugar", jwt.MapClaims{"id": float64(7), "role": "client", "account_type": "client"}, "7", wsKey{"client", 7}, true},
		{"cliente 7 com token legado", jwt.MapClaims{"id": float64(7), "role": "client"}, "7", wsKey{"client", 7}, true},
		{"entregador 7", jwt.MapClaims{"id": float64(7)}, "7", wsKey{"deliveryman", 7}, true},
		{"usuário sem loja", jwt.MapClaims{"id": float64(7), "role": "user", "account_type": "user"}, "7", wsKey{"user", 7}, true},
		{"id da URL de outro", jwt.MapClaims{"id": float64(7), "role": "client"}, "8", wsKey{}, false},
		{"admin não entra no lugar de outro id", jwt.MapClaims{"id": float64(1), "role": "admin", "account_type": "user"}, "7", wsKey{}, false},
	}
	for _, c := range casos {
		got, ok := wsKeyForClaims(c.claims, c.url)
		if ok != c.ok || got != c.quer {
			t.Errorf("%s: got (%v, %v), want (%v, %v)", c.nome, got, ok, c.quer, c.ok)
		}
	}
}

// O aviso de pedido novo da loja 7 (com dados do cliente) chegava a quem
// abrisse /ws/7 — o cliente 7, o entregador 7 ou o usuário 7 de outra loja.
func TestSendToEstablishment_SoALojaRecebe(t *testing.T) {
	resetWSClients(t)
	loja := &fakeWriter{}
	cliente7 := &fakeWriter{}
	entregador7 := &fakeWriter{}
	registerWSConn(wsKey{wsKindEstablishment, 7}, &safeConn{conn: loja})
	registerWSConn(wsKey{"client", 7}, &safeConn{conn: cliente7})
	registerWSConn(wsKey{"deliveryman", 7}, &safeConn{conn: entregador7})

	if err := sendToEstablishment(7, []byte(`{"user":{"phone":"+5511..."}}`)); err != nil {
		t.Fatal(err)
	}
	if len(loja.writes) != 1 {
		t.Fatalf("a loja 7 devia receber 1 aviso, recebeu %d", len(loja.writes))
	}
	if len(cliente7.writes) != 0 || len(entregador7.writes) != 0 {
		t.Fatalf("aviso da loja vazou: cliente=%d entregador=%d", len(cliente7.writes), len(entregador7.writes))
	}
}

// Duas abas (ou dois funcionários) da mesma loja recebem; a partir do teto, a
// conexão mais antiga é fechada. Antes, cada aba derrubava a outra.
func TestWSRegistry_VariasAbas(t *testing.T) {
	resetWSClients(t)
	key := wsKey{wsKindEstablishment, 9}
	abas := make([]*closableWriter, maxWSConnsPerKey+1)
	conns := make([]*safeConn, len(abas))
	for i := range abas {
		abas[i] = &closableWriter{}
		conns[i] = &safeConn{conn: abas[i]}
		registerWSConn(key, conns[i])
	}
	if !abas[0].closed {
		t.Fatal("acima do teto, a aba mais antiga devia ser fechada")
	}
	if err := sendToEstablishment(9, []byte("x")); err != nil {
		t.Fatal(err)
	}
	for i := 1; i < len(abas); i++ {
		if len(abas[i].writes) != 1 {
			t.Fatalf("aba %d recebeu %d avisos, want 1", i, len(abas[i].writes))
		}
	}

	// Aba que fecha sai do registro; as outras continuam.
	unregisterWSConn(key, conns[1])
	_ = sendToEstablishment(9, []byte("y"))
	if len(abas[1].writes) != 1 || len(abas[2].writes) != 2 {
		t.Fatalf("depois de sair: aba1=%d aba2=%d", len(abas[1].writes), len(abas[2].writes))
	}
	for i := 2; i < len(conns); i++ {
		unregisterWSConn(key, conns[i])
	}
	wsClientsMu.Lock()
	_, sobrou := wsClients[key]
	wsClientsMu.Unlock()
	if sobrou {
		t.Fatal("sem conexões, a chave devia sair do mapa")
	}
}

// Eventos das filas vão ao tipo certo de conta: client_id/user_id são
// clientes, courier_id é entregador.
func TestResolveStatusRecipientAccount(t *testing.T) {
	casos := []struct {
		fila string
		evt  statusEvent
		kind string
		id   int64
	}{
		{"payment_updates", statusEvent{UserID: 7}, "client", 7},
		{"order_updates", statusEvent{ClientID: 4, UserID: 7}, "client", 4},
		{"delivery_updates", statusEvent{CourierID: 5}, "deliveryman", 5},
		{"order_updates", statusEvent{CourierID: 5}, "", 0},
	}
	for _, c := range casos {
		kind, id := resolveStatusRecipientAccount(c.fila, &c.evt)
		if kind != c.kind || id != c.id {
			t.Errorf("%s %+v: got (%q, %d), want (%q, %d)", c.fila, c.evt, kind, id, c.kind, c.id)
		}
	}
}
