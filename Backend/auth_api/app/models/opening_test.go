package models

import (
	"testing"
	"time"
)

// Horário de Brasília (UTC-3) — o servidor roda em UTC.
func brt(t *testing.T, s string) time.Time {
	t.Helper()
	v, err := time.ParseInLocation("2006-01-02 15:04", s, StoreLocation())
	if err != nil {
		t.Fatal(err)
	}
	return v.UTC() // o servidor enxerga em UTC; OpeningAt tem de converter
}

func day(d int, open, close string) BusinessHours {
	return BusinessHours{DayOfWeek: d, IsOpen: true, OpenTime: open, CloseTime: close}
}

// 2026-09-25 é sexta (5).
func TestOpeningAt_UsaHorarioDeBrasilia(t *testing.T) {
	hours := []BusinessHours{day(5, "08:00", "22:00")}
	// 07:30 em Brasília = 10:30 UTC. Lendo em UTC a loja pareceria aberta.
	if st := OpeningAt(true, hours, brt(t, "2026-09-25 07:30")); st.IsOpen || st.OpensAt != "08:00" || st.OpensDay != 0 {
		t.Fatalf("07:30 BRT: esperava fechada, abre 08:00 hoje; veio %+v", st)
	}
	if st := OpeningAt(true, hours, brt(t, "2026-09-25 21:59")); !st.IsOpen {
		t.Fatal("21:59 BRT deveria estar aberta")
	}
	if st := OpeningAt(true, hours, brt(t, "2026-09-25 22:00")); st.IsOpen {
		t.Fatal("22:00 BRT (fechamento) deveria estar fechada")
	}
}

func TestOpeningAt_TurnoQueViraAMadrugada(t *testing.T) {
	hours := []BusinessHours{day(5, "18:00", "02:00")} // sexta 18h → sábado 2h
	cases := map[string]bool{
		"2026-09-25 17:59": false,
		"2026-09-25 18:00": true,
		"2026-09-25 23:30": true,
		"2026-09-26 01:59": true, // sábado de madrugada, turno de sexta
		"2026-09-26 02:00": false,
	}
	for at, want := range cases {
		if got := OpeningAt(true, hours, brt(t, at)).IsOpen; got != want {
			t.Errorf("%s: aberta=%v, esperava %v", at, got, want)
		}
	}
}

func TestOpeningAt_BotaoManualEIntervaloESemGrade(t *testing.T) {
	hours := []BusinessHours{day(5, "08:00", "22:00")}
	if OpeningAt(false, hours, brt(t, "2026-09-25 12:00")).IsOpen {
		t.Fatal("botão desligado deveria fechar a loja mesmo no horário")
	}
	if !OpeningAt(true, nil, brt(t, "2026-09-25 03:00")).IsOpen {
		t.Fatal("sem grade cadastrada, vale só o botão")
	}
	h := day(5, "11:00", "23:00")
	h.BreakStartTime, h.BreakEndTime = "15:00", "18:00"
	st := OpeningAt(true, []BusinessHours{h}, brt(t, "2026-09-25 16:00"))
	if st.IsOpen || st.OpensAt != "18:00" {
		t.Fatalf("no intervalo: esperava fechada, reabre 18:00; veio %+v", st)
	}
}

func TestOpeningAt_ProximaAbertura(t *testing.T) {
	// Fecha sábado (6) e domingo (0); segunda (1) abre 10:00.
	hours := []BusinessHours{
		day(5, "08:00", "22:00"),
		{DayOfWeek: 6, IsOpen: false},
		{DayOfWeek: 0, IsOpen: false},
		day(1, "10:00", "20:00"),
	}
	st := OpeningAt(true, hours, brt(t, "2026-09-25 23:00")) // sexta à noite
	if st.IsOpen || st.OpensAt != "10:00" || st.OpensDay != 3 {
		t.Fatalf("esperava abre segunda (daqui a 3 dias) 10:00; veio %+v", st)
	}
	// Todos os dias fechados: sem próxima abertura.
	fechado := []BusinessHours{{DayOfWeek: 5, IsOpen: false}}
	if st := OpeningAt(true, fechado, brt(t, "2026-09-25 12:00")); st.IsOpen || st.OpensAt != "" {
		t.Fatalf("esperava fechada sem previsão; veio %+v", st)
	}
}

func TestOpeningAt_AberturaIgualFechamentoEh24h(t *testing.T) {
	hours := []BusinessHours{day(5, "00:00", "00:00")}
	for _, at := range []string{"2026-09-25 00:00", "2026-09-25 12:00", "2026-09-25 23:59"} {
		if !OpeningAt(true, hours, brt(t, at)).IsOpen {
			t.Errorf("%s: 24 h deveria estar aberta", at)
		}
	}
}
