package models

import "time"

type UserRole string

const (
	RoleAdmin     UserRole = "admin"
	RoleOperator  UserRole = "operator"
	RoleViewer    UserRole = "viewer"
)

type User struct {
	ID       string   `json:"id" bson:"_id,omitempty"`
	Email    string   `json:"email" bson:"email"`
	Name     string   `json:"name" bson:"name"`
	Password string   `json:"-" bson:"password"`
	Role     UserRole `json:"role" bson:"role"`
	Active   bool     `json:"active" bson:"active"`
	CreatedAt  time.Time `json:"created_at" bson:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" bson:"updated_at"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}
