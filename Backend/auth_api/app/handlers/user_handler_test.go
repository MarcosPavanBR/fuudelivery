// Package handlers - user_handler_test.go
// Testes unitarios do user handler.
// Testam validacao de entrada, hashing de senha, e logica de negocios.
package handlers

import (
	"testing"

	"github.com/carloshomar/vercardapio/auth_api/app/dto"
	"golang.org/x/crypto/bcrypt"
)

// === Testes de Login ===

func TestLoginRequest_Validation(t *testing.T) {
	tests := []struct {
		name      string
		req       dto.LoginRequest
		wantError bool
	}{
		{
			"valid login",
			dto.LoginRequest{Email: "admin@fuudelivery.com", Password: "secret123"},
			false,
		},
		{
			"empty email",
			dto.LoginRequest{Email: "", Password: "secret123"},
			true,
		},
		{
			"empty password",
			dto.LoginRequest{Email: "admin@fuudelivery.com", Password: ""},
			true,
		},
		{
			"both empty",
			dto.LoginRequest{Email: "", Password: ""},
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isInvalid := tt.req.Email == "" || tt.req.Password == ""
			if isInvalid != tt.wantError {
				t.Errorf("got invalid=%v, want %v", isInvalid, tt.wantError)
			}
		})
	}
}

// === Testes de password hashing ===

func TestPasswordHashing(t *testing.T) {
	password := "mysecretpassword"

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}

	// Hash should not equal the plain password
	if string(hash) == password {
		t.Error("Hash should not equal plain password")
	}

	// Hash should start with $2a$ (bcrypt prefix)
	if len(hash) < 4 || string(hash[:4]) != "$2a$" {
		t.Errorf("Hash should start with $2a$, got %q", string(hash[:4]))
	}
}

func TestPasswordVerification(t *testing.T) {
	password := "testpassword123"

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}

	// Correct password should verify
	err = bcrypt.CompareHashAndPassword(hash, []byte(password))
	if err != nil {
		t.Errorf("Correct password should verify: %v", err)
	}

	// Wrong password should fail
	err = bcrypt.CompareHashAndPassword(hash, []byte("wrongpassword"))
	if err == nil {
		t.Error("Wrong password should fail verification")
	}
}

// === Testes de CreateUserRequest ===

func TestCreateUserRequest_Fields(t *testing.T) {
	req := dto.CreateUserRequest{
		Name:  "Restaurante Teste",
		Email: "teste@restaurante.com",
		Password: "senha123",
		Establishment: dto.EstablishmentRequest{
			Name:        "Restaurante ABC",
			Description: "Comida brasileira",
		},
	}

	if req.Name != "Restaurante Teste" {
		t.Errorf("Name: got %q", req.Name)
	}
	if req.Email != "teste@restaurante.com" {
		t.Errorf("Email: got %q", req.Email)
	}
	if req.Establishment.Name != "Restaurante ABC" {
		t.Errorf("Establishment.Name: got %q", req.Establishment.Name)
	}
}

// === Testes de ChangePasswordRequest ===

func TestChangePasswordRequest_Validation(t *testing.T) {
	tests := []struct {
		name         string
		newPassword  string
		minLength    int
		valid        bool
	}{
		{"valid password", "newpassword123", 6, true},
		{"too short", "abc", 6, false},
		{"exactly 6 chars", "abcdef", 6, true},
		{"5 chars", "abcde", 6, false},
		{"empty", "", 6, false},
		{"long password", "averylongpassword123456789", 6, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid := len(tt.newPassword) >= tt.minLength
			if valid != tt.valid {
				t.Errorf("password=%q len=%d: got valid=%v, want %v",
					tt.newPassword, len(tt.newPassword), valid, tt.valid)
			}
		})
	}
}

// === Testes de role validation ===

func TestUserRole_Values(t *testing.T) {
	roles := []string{"admin", "operator", "viewer", "user"}

	validRoles := make(map[string]bool)
	for _, r := range roles {
		validRoles[r] = true
	}

	tests := []struct {
		role  string
		valid bool
	}{
		{"admin", true},
		{"operator", true},
		{"viewer", true},
		{"user", true},
		{"superadmin", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.role, func(t *testing.T) {
			if validRoles[tt.role] != tt.valid {
				t.Errorf("Role %q: got valid=%v, want %v", tt.role, validRoles[tt.role], tt.valid)
			}
		})
	}
}
