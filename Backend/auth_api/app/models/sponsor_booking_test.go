package models

import (
	"reflect"
	"testing"
	"time"
)

func TestSponsorDayRange(t *testing.T) {
	got, err := SponsorDayRange("2026-09-29", 3)
	if err != nil || !reflect.DeepEqual(got, []string{"2026-09-29", "2026-09-30", "2026-10-01"}) {
		t.Fatalf("virada de mês: %v %v", got, err)
	}
	if _, err := SponsorDayRange("29/09/2026", 1); err == nil {
		t.Fatal("formato inválido deveria falhar")
	}
	if _, err := SponsorDayRange("2026-09-29", 0); err == nil {
		t.Fatal("0 dias deveria falhar")
	}
}

// Cada loja fica o mesmo tempo em 1º lugar ao longo do dia.
func TestRotateSponsors_Justo(t *testing.T) {
	ids := []uint{10, 20, 30}
	first := map[uint]int{}
	base := time.Date(2026, 9, 25, 0, 30, 0, 0, StoreLocation())
	for h := 0; h < 24; h++ {
		order := RotateSponsors(ids, base.Add(time.Duration(h)*time.Hour))
		if len(order) != 3 {
			t.Fatalf("rodízio perdeu loja: %v", order)
		}
		first[order[0]]++
	}
	for _, id := range ids {
		if first[id] != 8 {
			t.Fatalf("loja %d ficou %d h em 1º (esperava 8): %v", id, first[id], first)
		}
	}
}

func listing(id uint, open bool) EstablishmentListing {
	return EstablishmentListing{Establishment: Establishment{ID: id}, OpeningStatus: OpeningStatus{IsOpen: open}}
}

func ids(items []EstablishmentListing) []uint {
	out := []uint{}
	for _, it := range items {
		out = append(out, it.ID)
	}
	return out
}

func TestOrderListing(t *testing.T) {
	items := []EstablishmentListing{
		listing(1, true), listing(2, false), listing(3, true), listing(4, true), listing(5, false),
	}
	// Rodízio: 4 primeiro, depois 5 (fechada) e 3.
	got := OrderListing(items, []uint{4, 5, 3})
	if want := []uint{4, 3, 1, 2, 5}; !reflect.DeepEqual(ids(got), want) {
		t.Fatalf("ordem %v, esperava %v", ids(got), want)
	}
	selo := map[uint]bool{}
	for _, it := range got {
		selo[it.ID] = it.IsSponsored
	}
	if !selo[4] || !selo[3] || selo[5] || selo[1] {
		t.Fatalf("selo errado (fechada não leva selo): %v", selo)
	}
	// Sem patrocínio: abertas na ordem recebida, fechadas no fim.
	if want := []uint{1, 3, 4, 2, 5}; !reflect.DeepEqual(ids(OrderListing(items, nil)), want) {
		t.Fatalf("sem patrocínio: %v", ids(OrderListing(items, nil)))
	}
}
