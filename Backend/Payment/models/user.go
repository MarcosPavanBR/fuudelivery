// Package models - user.go
// Define a estrutura de usuarios do Payment Service e seus tipos de papel (role).
package models

import "time"

// UserRole define o nivel de acesso do usuario no sistema.
type UserRole string

const (
	RoleAdmin    UserRole = "admin"    // Acesso total: configura regras, gerencia usuarios
	RoleOperator UserRole = "operator" // Operador: aprova/rejeita pagamentos
	RoleViewer   UserRole = "viewer"   // Visualizador: apenas consulta relatorios
)

// User representa um usuario do sistema de pagamentos.
// Pode ser um administrador, operador ou visualizador.
type User struct {
	ID        string   `json:"id" bson:"_id,omitempty"`    // ID unico (pode ser email ou UUID)
	Email     string   `json:"email" bson:"email"`         // Email (usado como login)
	Name      string   `json:"name" bson:"name"`           // Nome completo
	Password  string   `json:"-" bson:"password"`          // Hash da senha (nunca retorna na API)
	Role      UserRole `json:"role" bson:"role"`           // Nivel de acesso
	Active    bool     `json:"active" bson:"active"`       // Se a conta esta ativa
	CreatedAt time.Time `json:"created_at" bson:"created_at"` // Data de criacao
	UpdatedAt time.Time `json:"updated_at" bson:"updated_at"` // Data da ultima atualizacao
}

// LoginRequest representa os dados enviados para autenticacao.
type LoginRequest struct {
	Email    string `json:"email"`    // Email do usuario
	Password string `json:"password"` // Senha em texto plano (sera comparada com o hash)
}

// LoginResponse representa a resposta apos autenticacao bem-sucedida.
type LoginResponse struct {
	Token string `json:"token"` // Token JWT para autenticacao
	User  User   `json:"user"`  // Dados do usuario autenticado
}
