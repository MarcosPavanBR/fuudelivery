package handlers

import "strconv"

// validID diz se um id vindo da URL é um inteiro positivo. Valide ANTES de
// consultar e passe o id como argumento ("id = ?"), nunca como condição
// solta: no GORM, First(&x, idString) com string não numérica vira SQL
// literal — GET /establishments/0=0 devolvia a primeira loja (injeção de SQL,
// numa rota pública).
func validID(s string) bool {
	n, err := strconv.ParseUint(s, 10, 64)
	return err == nil && n > 0
}
