package main

import (
	"errors"
	"sync"
	"testing"
)

// fakeWriter NÃO é thread-safe de propósito: o append no slice é o "race
// bait". Se o safeConn falhar em serializar as escritas, `go test -race`
// detecta o acesso concorrente e o teste falha. Com o mutex, todas as
// chamadas chegam sequenciais aqui.
type fakeWriter struct {
	writes [][]byte
	failAt int // índice (1-based) em que WriteMessage deve falhar; 0 = nunca
}

func (f *fakeWriter) WriteMessage(messageType int, data []byte) error {
	f.writes = append(f.writes, data)
	if f.failAt > 0 && len(f.writes) == f.failAt {
		return errors.New("fake write failure")
	}
	return nil
}

func TestSafeConn_ConcurrentWritesAreSerialized(t *testing.T) {
	const (
		goroutines   = 8
		perGoroutine = 50
	)

	fake := &fakeWriter{}
	sc := &safeConn{conn: fake}

	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				if err := sc.WriteMessage(1, []byte("msg")); err != nil {
					t.Errorf("escrita inesperada falhou: %v", err)
				}
			}
		}(g)
	}
	wg.Wait()

	if got := len(fake.writes); got != goroutines*perGoroutine {
		t.Fatalf("esperava %d escritas, recebi %d", goroutines*perGoroutine, got)
	}
}

func TestSafeConn_ErrorPropagates(t *testing.T) {
	fake := &fakeWriter{failAt: 2}
	sc := &safeConn{conn: fake}

	if err := sc.WriteMessage(1, []byte("ok")); err != nil {
		t.Fatalf("primeira escrita deveria passar: %v", err)
	}
	if err := sc.WriteMessage(1, []byte("boom")); err == nil {
		t.Fatal("segunda escrita deveria propagar o erro do writer")
	}
	// O mutex não pode ficar preso após um erro — a próxima escrita passa.
	if err := sc.WriteMessage(1, []byte("recupera")); err != nil {
		t.Fatalf("escrita pós-erro deveria funcionar: %v", err)
	}
}
