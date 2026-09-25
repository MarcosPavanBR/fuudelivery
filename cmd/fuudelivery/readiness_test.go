package main

import (
	"testing"
	"time"
)

func TestStillStarting(t *testing.T) {
	prevDone := initDone.Load()
	t.Cleanup(func() { initDone.Store(prevDone) })

	initDone.Store(false)
	if !stillStarting(processStart.Add(time.Minute)) {
		t.Error("inicialização em andamento deve responder starting")
	}
	if stillStarting(processStart.Add(maxStartingWindow + time.Second)) {
		t.Error("inicialização travada não pode se esconder atrás de starting")
	}
	initDone.Store(true)
	if stillStarting(processStart.Add(time.Minute)) {
		t.Error("inicialização concluída responde com os checks reais")
	}
}
