// Package services - user_service.go
// Servico de autenticacao e gestao de usuarios do Payment Service.
package services

import (
	"time"

	"github.com/carloshomar/vercardapio/payment/config"
	"github.com/carloshomar/vercardapio/payment/middleware"
	"github.com/carloshomar/vercardapio/payment/models"
	"github.com/carloshomar/vercardapio/payment/repository"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// UserService e responsavel pelas operacoes de usuario.
type UserService struct{}

// NewUserService cria uma nova instancia do servico.
func NewUserService() *UserService {
	return &UserService{}
}

// Login autentica um usuario pelo email e retorna um token JWT valido.
func (us *UserService) Login(email, password string) (*models.LoginResponse, error) {
	user, err := repository.GetUserByEmail(email)
	if err != nil {
		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return nil, err
	}

	claims := &middleware.Claims{
		UserID: user.ID,
		Email:  user.Email,
		Role:   string(user.Role),
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(config.AppConfig.JWTSecret))
	if err != nil {
		return nil, err
	}

	return &models.LoginResponse{
		Token: tokenString,
		User:  *user,
	}, nil
}

// GetUser busca um usuario pelo ID.
func (us *UserService) GetUser(id string) (*models.User, error) {
	return repository.GetUserByID(id)
}

// ListUsers retorna todos os usuarios cadastrados.
func (us *UserService) ListUsers() ([]models.User, error) {
	return repository.ListUsers()
}

// CreateUser insere um novo usuario no banco.
func (us *UserService) CreateUser(user *models.User) error {
	user.Active = true
	user.CreatedAt = time.Now()
	user.UpdatedAt = time.Now()
	return repository.CreateUser(user)
}
