package dto

// GeneratePasswordResetCodeRequest — corpo do POST /admin/password-reset/code.
// O admin informa o tipo de conta e o identificador (telefone ou email);
// recebe o código de uso único EM CLARO uma única vez.
type GeneratePasswordResetCodeRequest struct {
	UserType   string `json:"user_type"`   // client | user | delivery_man
	Identifier string `json:"identifier"` // telefone (client/delivery_man) ou email/telefone (user)
}

// PasswordResetRequest — corpo do POST /auth/reset-password (endpoint público,
// usado pela página /resetar-senha do WebRestaurant).
type PasswordResetRequest struct {
	UserType    string `json:"user_type"`
	Identifier  string `json:"identifier"`
	Code        string `json:"code"`
	NewPassword string `json:"new_password"`
}
