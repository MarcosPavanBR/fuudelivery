package handlers

import (
	"sync"
	"testing"
)

// fakeConn NÃO é thread-safe de propósito (append sem lock) — é o race bait
// que faz `go test -race` pegar se o safeConn deixar passar duas escritas
// concorrentes no mesmo *websocket.Conn.
type fakeConn struct {
	writes int
}

func (f *fakeConn) WriteMessage(messageType int, data []byte) error {
	f.writes++
	return nil
}

func TestSafeConn_BroadcastConcorrente(t *testing.T) {
	// Cenário real do chat: o read-loop do cliente A faz broadcast para B
	// enquanto o read-loop do próprio B escreve o ack message_sent — duas
	// goroutines, mesmo conn.
	fake := &fakeConn{}
	sc := &safeConn{conn: fake}

	var wg sync.WaitGroup
	for g := 0; g < 4; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 25; i++ {
				_ = sc.WriteMessage(1, []byte(`{"type":"new_message"}`))
				_ = sc.WriteMessage(1, []byte(`{"type":"message_sent"}`))
			}
		}()
	}
	wg.Wait()

	if fake.writes != 4*25*2 {
		t.Fatalf("esperava %d escritas, recebi %d", 4*25*2, fake.writes)
	}
}
