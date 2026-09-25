package main

// Prontidão do processo para o /health.
//
// A inicialização (bancos dos cinco módulos, fila, storage, dispatch) roda
// numa goroutine para o /health responder já na subida. O /health dizia
// "starting" só enquanto o banco do auth era nulo; na janela até os outros
// conectarem, ele respondia com os checks reais e o monitor via
// "batches: down" a cada cold start.

import (
	"sync/atomic"
	"time"
)

var (
	processStart = time.Now()
	initDone     atomic.Bool
)

// maxStartingWindow: passado isso sem a inicialização terminar, ela travou —
// o /health volta a mostrar os checks reais em vez de "starting" para sempre.
const maxStartingWindow = 5 * time.Minute

// stillStarting diz se o /health deve responder "starting".
func stillStarting(now time.Time) bool {
	return !initDone.Load() && now.Sub(processStart) < maxStartingWindow
}
