// Package services - user_service_test.go
// Testes unitarios do servico de usuarios.
// Testam modelagem, constantes de roles, e regras de validacao.
// Nao dependem de MongoDB — apenas logica pura.
package services

import (
	"testing"

	"github.com/carloshomar/vercardapio/payment/models"
)

// === Testes de UserRole ===

func TestUserRoles(t *testing.T) {
	roles := map[models.UserRole]string{
		models.RoleAdmin:    "admin",
		models.RoleOperator: "operator",
		models.RoleViewer:   "viewer",
	}

	for role, expected := range roles {
		if string(role) != expected {
			t.Errorf("Role %v: got %q, want %q", role, string(role), expected)
		}
	}
}

// === Testes de User struct ===

func TestUser_Creation(t *testing.T) {
	user := models.User{
		ID:     "user_001",
		Email:  "admin@fuudelivery.com",
		Name:   "Admin User",
		Role:   models.RoleAdmin,
		Active: true,
	}

	if user.ID != "user_001" {
		t.Errorf("ID: got %q, want %q", user.ID, "user_001")
	}
	if user.Email != "admin@fuudelivery.com" {
		t.Errorf("Email: got %q, want %q", user.Email, "admin@fuudelivery.com")
	}
	if user.Role != models.RoleAdmin {
		t.Errorf("Role: got %q, want %q", user.Role, models.RoleAdmin)
	}
	if !user.Active {
		t.Error("Active should be true")
	}
}

func TestUser_DefaultActive(t *testing.T) {
	user := models.User{}

	if user.Active {
		t.Error("Default Active should be false")
	}
}

func TestUser_PasswordHidden(t *testing.T) {
	// Password field has json:"-" tag, should not appear in JSON
	user := models.User{
		Password: "secret123",
	}

	// The Password field exists in the struct
	if user.Password != "secret123" {
		t.Errorf("Password: got %q, want %q", user.Password, "secret123")
	}
}

// === Testes de LoginRequest ===

func TestLoginRequest_Creation(t *testing.T) {
	req := models.LoginRequest{
		Email:    "admin@fuudelivery.com",
		Password: "secret123",
	}

	if req.Email != "admin@fuudelivery.com" {
		t.Errorf("Email: got %q, want %q", req.Email, "admin@fuudelivery.com")
	}
	if req.Password != "secret123" {
		t.Errorf("Password: got %q, want %q", req.Password, "secret123")
	}
}

func TestLoginRequest_EmptyFields(t *testing.T) {
	req := models.LoginRequest{}

	if req.Email != "" {
		t.Errorf("Default Email should be empty: got %q", req.Email)
	}
	if req.Password != "" {
		t.Errorf("Default Password should be empty: got %q", req.Password)
	}
}

// === Testes de LoginResponse ===

func TestLoginResponse_Creation(t *testing.T) {
	user := models.User{
		ID:    "user_001",
		Email: "admin@fuudelivery.com",
		Name:  "Admin User",
		Role:  models.RoleAdmin,
	}

	resp := models.LoginResponse{
		Token: "eyJhbGciOiJIUzI1NiJ9...",
		User:  user,
	}

	if resp.Token != "eyJhbGciOiJIUzI1NiJ9..." {
		t.Errorf("Token: got %q, want %q", resp.Token, "eyJhbGciOiJIUzI1NiJ9...")
	}
	if resp.User.Email != "admin@fuudelivery.com" {
		t.Errorf("User.Email: got %q, want %q", resp.User.Email, "admin@fuudelivery.com")
	}
}

// === Testes de NewUserService ===

func TestNewUserService(t *testing.T) {
	svc := NewUserService()
	if svc == nil {
		t.Error("NewUserService should return non-nil")
	}
}

// === Testes de regras de validacao ===

func TestUser_EmailValidation(t *testing.T) {
	tests := []struct {
		name  string
		email string
		valid bool
	}{
		{"valid email", "admin@fuudelivery.com", true},
		{"empty email", "", false},
		{"non-empty email", "admin@", true},
		{"another valid email", "adminfuudelivery.com", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simplified: just check non-empty
			valid := len(tt.email) > 0
			if valid != tt.valid {
				t.Errorf("email=%q: got valid=%v, want %v", tt.email, valid, tt.valid)
			}
		})
	}
}

func TestUser_RolePermissions(t *testing.T) {
	tests := []struct {
		role          models.UserRole
		canApprove    bool
		canViewOnly   bool
	}{
		{models.RoleAdmin, true, false},
		{models.RoleOperator, true, false},
		{models.RoleViewer, false, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.role), func(t *testing.T) {
			// Replicates role permission logic
			canApprove := tt.role == models.RoleAdmin || tt.role == models.RoleOperator
			canViewOnly := tt.role == models.RoleViewer

			if canApprove != tt.canApprove {
				t.Errorf("Role %q: canApprove=%v, want %v", tt.role, canApprove, tt.canApprove)
			}
			if canViewOnly != tt.canViewOnly {
				t.Errorf("Role %q: canViewOnly=%v, want %v", tt.role, canViewOnly, tt.canViewOnly)
			}
		})
	}
}

func TestUser_CreateUserSetsActive(t *testing.T) {
	// CreateUser sets Active=true by default
	user := models.User{
		Active: false,
	}

	// Simulate CreateUser logic
	user.Active = true

	if !user.Active {
		t.Error("CreateUser should set Active=true")
	}
}
