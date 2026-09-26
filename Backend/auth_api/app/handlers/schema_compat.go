package handlers

import "database/sql"

// Compatibilidade com os dois schemas de `users` que existem:
//
//   - produção: tabela herdada, com "createdAt"/"updatedAt" (camelCase,
//     NOT NULL) e a coluna role do tipo enum "Role";
//   - banco novo (AutoMigrate — CI, instalação nova): sem essas colunas e com
//     role texto.
//
// O cadastro escrevia SQL só para o primeiro: `'"Role"'::regtype` DÁ ERRO
// quando o tipo não existe, e em Postgres o erro aborta a transação inteira
// ("current transaction is aborted") — o cadastro de restaurante falhava em
// todo banco novo, e a lista de usuários do admin também. As consultas abaixo
// não erram em nenhum dos dois: perguntam ao catálogo antes.

type rowQueryer interface {
	QueryRow(query string, args ...any) *sql.Row
}

// usersHasCamelTimestamps diz se users tem "createdAt"/"updatedAt".
func usersHasCamelTimestamps(q rowQueryer) bool {
	var n int
	_ = q.QueryRow(`SELECT COUNT(*) FROM information_schema.columns
		WHERE table_schema = current_schema() AND table_name = 'users'
		AND column_name IN ('createdAt', 'updatedAt')`).Scan(&n)
	return n == 2
}

// userRoleValue devolve o valor de role para uma conta de loja: 'restaurant'
// se o enum "Role" existir e tiver esse rótulo; o primeiro rótulo do enum se
// existir sem ele; "user" quando role é texto.
func userRoleValue(q rowQueryer) string {
	const labels = `SELECT e.enumlabel FROM pg_enum e JOIN pg_type t ON t.oid = e.enumtypid
		WHERE t.typname = 'Role'`
	var v string
	_ = q.QueryRow(labels + ` AND e.enumlabel = 'restaurant'`).Scan(&v)
	if v == "" {
		_ = q.QueryRow(labels + ` ORDER BY e.enumsortorder LIMIT 1`).Scan(&v)
	}
	if v == "" {
		v = "user"
	}
	return v
}

// insertUserSQL monta o INSERT de users com ou sem os timestamps camelCase.
func insertUserSQL(q rowQueryer, withPhone bool) string {
	cols := "id, name, email, password, role"
	vals := "$1, $2, $3, $4, $5"
	if withPhone {
		cols = "id, name, email, password, role, phone"
		vals = "$1, $2, $3, $4, $5, $6"
	}
	if usersHasCamelTimestamps(q) {
		cols += `, "createdAt", "updatedAt"`
		vals += ", NOW(), NOW()"
	}
	return "INSERT INTO users (" + cols + ") VALUES (" + vals + ")"
}
