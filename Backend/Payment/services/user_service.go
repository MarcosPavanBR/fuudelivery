package services

import (
	"time"

	"github.com/carloshomar/vercardapio/payment/models"
	"github.com/carloshomar/vercardapio/payment/repository"
)

type UserService struct{}

func NewUserService() *UserService {
	return &UserService{}
}

func (us *UserService) Login(email, password string) (*models.LoginResponse, error) {
	user, err := repository.GetUserByEmail(email)
	if err != nil {
		return nil, err
	}

	return &models.LoginResponse{
		Token: "dummy-token",
		User:  *user,
	}, nil
}

func (us *UserService) GetUser(id string) (*models.User, error) {
	return repository.GetUserByID(id)
}

func (us *UserService) ListUsers() ([]models.User, error) {
	return repository.ListUsers()
}

func (us *UserService) CreateUser(user *models.User) error {
	user.Active = true
	user.CreatedAt = time.Now()
	user.UpdatedAt = time.Now()
	return repository.CreateUser(user)
}
