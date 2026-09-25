package models

import (
	"os"
	"sync"
	"time"

	// Embute a base de fusos no binário: a imagem do Render pode não ter
	// /usr/share/zoneinfo, e sem ela LoadLocation falharia e o horário da
	// loja voltaria a ser calculado em UTC (3 h de diferença).
	_ "time/tzdata"
)

// storeLocation é o fuso em que a grade de horários das lojas é lida.
// O servidor roda em UTC; a loja cadastra "08:00" pensando no relógio dela.
// STORE_TIMEZONE permite outra praça; o padrão é o horário de Brasília.
var (
	storeLocOnce sync.Once
	storeLoc     *time.Location
)

func StoreLocation() *time.Location {
	storeLocOnce.Do(func() {
		name := os.Getenv("STORE_TIMEZONE")
		if name == "" {
			name = "America/Sao_Paulo"
		}
		loc, err := time.LoadLocation(name)
		if err != nil {
			loc = time.FixedZone("BRT", -3*60*60)
		}
		storeLoc = loc
	})
	return storeLoc
}

// OpeningStatus é o que o app do cliente precisa para a vitrine.
type OpeningStatus struct {
	IsOpen bool `json:"is_open"`
	// OpensAt: próximo horário de abertura em até 7 dias ("18:00"), quando
	// fechada pela grade. Vazio se aberta ou se não há horário cadastrado.
	OpensAt string `json:"opens_at,omitempty"`
	// OpensDay: 0 = hoje, 1 = amanhã, ... (acompanha OpensAt).
	OpensDay int `json:"opens_day,omitempty"`
}

func validHHMM(s string) bool {
	return len(s) == 5 && s[2] == ':' &&
		s[0] >= '0' && s[0] <= '2' && s[1] >= '0' && s[1] <= '9' &&
		s[3] >= '0' && s[3] <= '5' && s[4] >= '0' && s[4] <= '9'
}

// dayWindow devolve a janela do dia da grade em minutos e se o turno
// "vira" a meia-noite (fechamento antes da abertura).
func dayWindow(h BusinessHours) (open, close int, overnight, ok bool) {
	if !h.IsOpen || !validHHMM(h.OpenTime) || !validHHMM(h.CloseTime) {
		return 0, 0, false, false
	}
	open = parseTimeToMinutes(h.OpenTime)
	close = parseTimeToMinutes(h.CloseTime)
	if open == close {
		// Abertura = fechamento: aberto 24 h nesse dia ("00:00 às 00:00").
		return 0, 24 * 60, false, true
	}
	return open, close, close < open, true
}

func inBreak(h BusinessHours, m int) bool {
	if !validHHMM(h.BreakStartTime) || !validHHMM(h.BreakEndTime) {
		return false
	}
	bs, be := parseTimeToMinutes(h.BreakStartTime), parseTimeToMinutes(h.BreakEndTime)
	return bs < be && m >= bs && m < be
}

// OpeningAt calcula se a loja está aberta no instante t, pela grade.
//
//   - manualOpen é o botão "Aberto" do painel (open_data). Desligado =
//     fechada, qualquer que seja a grade (é o "pausar a loja agora").
//   - Sem nenhum dia cadastrado, vale só o botão (loja antiga, sem grade).
//   - Turno que passa da meia-noite (18:00–02:00): a madrugada pertence ao
//     dia em que o turno começou.
func OpeningAt(manualOpen bool, hours []BusinessHours, t time.Time) OpeningStatus {
	if !manualOpen {
		return OpeningStatus{IsOpen: false}
	}
	if len(hours) == 0 {
		return OpeningStatus{IsOpen: true}
	}
	byDay := map[int]BusinessHours{}
	for _, h := range hours {
		byDay[h.DayOfWeek] = h
	}

	t = t.In(StoreLocation())
	wd := int(t.Weekday())
	m := t.Hour()*60 + t.Minute()

	if h, ok := byDay[wd]; ok {
		if o, c, overnight, valid := dayWindow(h); valid {
			if (!overnight && m >= o && m < c) || (overnight && m >= o) {
				if !inBreak(h, m) {
					return OpeningStatus{IsOpen: true}
				}
			}
		}
	}
	if h, ok := byDay[(wd+6)%7]; ok {
		if _, c, overnight, valid := dayWindow(h); valid && overnight && m < c {
			return OpeningStatus{IsOpen: true}
		}
	}

	// Fechada: procura a próxima abertura (hoje mais tarde ou nos próximos dias).
	for d := 0; d < 7; d++ {
		h, ok := byDay[(wd+d)%7]
		if !ok {
			continue
		}
		o, _, _, valid := dayWindow(h)
		if !valid {
			continue
		}
		if d == 0 {
			if o > m {
				return OpeningStatus{OpensAt: h.OpenTime}
			}
			// Hoje, depois do intervalo.
			if inBreak(h, m) {
				return OpeningStatus{OpensAt: h.BreakEndTime}
			}
			continue
		}
		return OpeningStatus{OpensAt: h.OpenTime, OpensDay: d}
	}
	return OpeningStatus{}
}

// EstablishmentOpeningNow lê o botão e a grade da loja e devolve o status
// agora. Loja inexistente devolve erro.
func EstablishmentOpeningNow(establishmentID uint) (OpeningStatus, error) {
	var est Establishment
	if err := DB.First(&est, establishmentID).Error; err != nil {
		return OpeningStatus{}, err
	}
	var hours []BusinessHours
	if err := DB.Where("establishment_id = ?", establishmentID).Find(&hours).Error; err != nil {
		return OpeningStatus{}, err
	}
	return OpeningAt(est.OpenData != nil, hours, time.Now()), nil
}
