// Package search implementa o primeiro marco da Fase 3 (busca) do roadmap de
// modernizacao: busca full-text basica em estabelecimentos e produtos usando
// ILIKE no PostgreSQL (sem depender de Atlas Search/indice vetorial ainda).
//
// A busca e dividida em duas partes:
//  1. Score puro (score.go) — funcoes sem IO, testaveis unitariamente;
//  2. Handler (handler.go) — consulta os bancos e aplica o score.
//
// Evolucao prevista: indice $text no Mongo / pg_trgm, e depois busca vetorial
// com embeddings para "Fuu AI" (assistente de pedido).
package search

import (
	"strings"
)

// maxResults limita o numero de resultados por categoria.
const maxResults = 20

// ScoreType indica onde o termo foi encontrado (para ordenacao relevante).
type ScoreType int

const (
	// ScoreNone o termo nao bateu.
	ScoreNone ScoreType = 0
	// ScoreContains bateu em qualquer parte do campo.
	ScoreContains ScoreType = 1
	// ScorePrefix bateu como prefixo do campo (ex.: "piz" -> "pizza").
	ScorePrefix ScoreType = 2
	// ScoreExact bateu exatamente no campo inteiro.
	ScoreExact ScoreType = 3
)

// fieldScore pontua um campo: nome pesa mais que descricao/endereco.
func fieldScore(query, field string, isName bool) ScoreType {
	if field == "" || query == "" {
		return ScoreNone
	}
	f := strings.ToLower(strings.TrimSpace(field))
	q := strings.ToLower(strings.TrimSpace(query))
	if f == q {
		return ScoreExact
	}
	if strings.HasPrefix(f, q) {
		return ScorePrefix
	}
	if strings.Contains(f, q) {
		return ScoreContains
	}
	return ScoreNone
}

// itemScore combina o score do nome (peso 3) e do campo secundario (peso 1).
func itemScore(query string, name, secondary string) int {
	nameScore := fieldScore(query, name, true)
	secScore := fieldScore(query, secondary, false)
	score := int(nameScore)*3 + int(secScore)*1
	return score
}

// normalizedTerm limpa e normaliza o termo de busca.
func normalizedTerm(query string) string {
	return strings.TrimSpace(strings.ToLower(query))
}

// isSearchable valida se o termo tem tamanho util.
func isSearchable(query string) bool {
	q := normalizedTerm(query)
	return len(q) >= 2
}
