package middlewares

import (
	"os"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestJWTGenerationAndValidation(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret-key-for-ci")
	defer os.Unsetenv("JWT_SECRET")

	claims := jwt.MapClaims{
		"id":    float64(42),
		"name":  "Test User",
		"email": "test@example.com",
		"role":  "admin",
		"exp":   time.Now().UTC().Add(time.Hour * 24 * 7).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(os.Getenv("JWT_SECRET")))
	if err != nil {
		t.Fatalf("Failed to sign token: %v", err)
	}

	// Validate the token
	parsed, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		return []byte(os.Getenv("JWT_SECRET")), nil
	})
	if err != nil {
		t.Fatalf("Failed to parse token: %v", err)
	}

	if !parsed.Valid {
		t.Error("Token should be valid")
	}

	parsedClaims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		t.Fatal("Failed to get claims")
	}

	if parsedClaims["id"].(float64) != 42 {
		t.Errorf("Expected id=42, got %v", parsedClaims["id"])
	}
	if parsedClaims["role"].(string) != "admin" {
		t.Errorf("Expected role=admin, got %v", parsedClaims["role"])
	}
}

func TestJWTExpiredToken(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret-key-for-ci")
	defer os.Unsetenv("JWT_SECRET")

	claims := jwt.MapClaims{
		"id":   float64(1),
		"name": "Expired User",
		"exp":  time.Now().UTC().Add(-time.Hour).Unix(), // 1 hour ago
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(os.Getenv("JWT_SECRET")))
	if err != nil {
		t.Fatalf("Failed to sign token: %v", err)
	}

	// Validate the expired token
	parsed, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		return []byte(os.Getenv("JWT_SECRET")), nil
	})

	if err == nil && parsed.Valid {
		t.Error("Expired token should not be valid")
	}
}

func TestJWTWrongSecret(t *testing.T) {
	os.Setenv("JWT_SECRET", "correct-secret")
	defer os.Unsetenv("JWT_SECRET")

	claims := jwt.MapClaims{
		"id":   float64(1),
		"name": "User",
		"exp":  time.Now().UTC().Add(time.Hour).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, _ := token.SignedString([]byte("correct-secret"))

	// Try to validate with wrong secret
	os.Setenv("JWT_SECRET", "wrong-secret")
	parsed, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		return []byte(os.Getenv("JWT_SECRET")), nil
	})

	if err == nil && parsed.Valid {
		t.Error("Token with wrong secret should not be valid")
	}
}

func TestJWTRoleExtraction(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret-key")
	defer os.Unsetenv("JWT_SECRET")

	roles := []string{"admin", "user", "deliveryman", "establishment"}

	for _, role := range roles {
		claims := jwt.MapClaims{
			"id":   float64(1),
			"role": role,
			"exp":  time.Now().UTC().Add(time.Hour).Unix(),
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, _ := token.SignedString([]byte(os.Getenv("JWT_SECRET")))

		parsed, _ := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
			return []byte(os.Getenv("JWT_SECRET")), nil
		})

		parsedClaims := parsed.Claims.(jwt.MapClaims)
		if parsedClaims["role"].(string) != role {
			t.Errorf("Expected role=%s, got %v", role, parsedClaims["role"])
		}
	}
}
