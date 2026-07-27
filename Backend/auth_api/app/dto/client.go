package dto

// RegisterClientRequest representa o payload de cadastro de cliente.
type RegisterClientRequest struct {
	Name     string `json:"name"`
	Phone    string `json:"phone"`
	Password string `json:"password"`
}

// LoginClientRequest representa o payload de login de cliente.
type LoginClientRequest struct {
	Phone    string `json:"phone"`
	Password string `json:"password"`
}
