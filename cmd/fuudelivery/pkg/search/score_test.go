package search

import "testing"

func TestFieldScore(t *testing.T) {
	cases := []struct {
		name   string
		query  string
		field  string
		isName bool
		want   ScoreType
	}{
		{"exact match", "pizza", "Pizza", true, ScoreExact},
		{"exact case-insensitive", "PIZZA", "pizza", true, ScoreExact},
		{"prefix", "piz", "Pizza", true, ScorePrefix},
		{"contains", "izz", "Pizza", true, ScoreContains},
		{"no match", "burger", "Pizza", true, ScoreNone},
		{"empty query", "", "Pizza", true, ScoreNone},
		{"empty field", "pizza", "", true, ScoreNone},
	}
	for _, tc := range cases {
		got := fieldScore(tc.query, tc.field, tc.isName)
		if got != tc.want {
			t.Errorf("%s: fieldScore(%q,%q) = %d, want %d", tc.name, tc.query, tc.field, got, tc.want)
		}
	}
}

func TestItemScoreNameWeightsMore(t *testing.T) {
	// "pizza" no nome deve pesar mais que na descricao.
	nameHit := itemScore("pizza", "Pizza Hut", "Delivery de massas")
	descHit := itemScore("pizza", "Restaurante Central", "Pizza artesanal e massas")
	if nameHit <= descHit {
		t.Errorf("nome deveria pesar mais: nameHit=%d descHit=%d", nameHit, descHit)
	}
}

func TestIsSearchable(t *testing.T) {
	if isSearchable("") {
		t.Error("string vazia nao deveria ser searchable")
	}
	if isSearchable("a") {
		t.Error("1 char nao deveria ser searchable")
	}
	if !isSearchable("pizza") {
		t.Error("termo valido deveria ser searchable")
	}
	if isSearchable("   ") {
		t.Error("espacos em branco nao deveriam ser searchable")
	}
}

func TestNormalizedTerm(t *testing.T) {
	if got := normalizedTerm("  Pizza  "); got != "pizza" {
		t.Errorf("normalizedTerm = %q, want %q", got, "pizza")
	}
}
