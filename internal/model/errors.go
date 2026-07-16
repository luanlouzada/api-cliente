package model

import "errors"

var (
	ErrCustomerNotFound             = errors.New("cliente não encontrado")
	ErrCustomerNameRequired         = errors.New("nome é obrigatório")
	ErrCustomerNameTooShort         = errors.New("nome deve ter pelo menos 3 caracteres")
	ErrCustomerNameTooLong          = errors.New("nome deve ter no máximo 255 caracteres")
	ErrCustomerEmailRequired        = errors.New("email é obrigatório")
	ErrCustomerEmailInvalid         = errors.New("email deve ser válido")
	ErrCustomerEmailTooLong         = errors.New("email deve ter no máximo 255 caracteres")
	ErrCustomerPhoneRequired        = errors.New("telefone é obrigatório")
	ErrCustomerPhoneInvalid         = errors.New("telefone deve ser válido")
	ErrCustomerPhoneTooLong         = errors.New("telefone deve ter no máximo 30 caracteres")
	ErrCustomerEmailAlreadyExists   = errors.New("email já existe")
	ErrInvalidCustomerID            = errors.New("id deve ser um UUID válido")
	ErrCustomerPasswordRequired     = errors.New("senha é obrigatória")
	ErrCustomerPasswordTooShort     = errors.New("senha deve ter pelo menos 8 caracteres")
	ErrCustomerPasswordTooLong      = errors.New("senha deve ter no máximo 72 bytes")
	ErrCustomerPasswordHashRequired = errors.New("hash da senha é obrigatório")
	ErrInvalidCredentials           = errors.New("credenciais inválidas")
	ErrRefreshTokenInvalid          = errors.New("refresh token inválido")
	ErrRefreshTokenExpired          = errors.New("refresh token expirado")
	ErrRefreshTokenReused           = errors.New("refresh token reutilizado")
	ErrForbidden                    = errors.New("acesso negado")
)

// IsValidationError informa se err, mesmo quando embrulhado com %w, representa
// uma falha nos dados enviados pelo cliente. Erros de autenticação, autorização,
// conflito e persistência não são classificados como validação.
func IsValidationError(err error) bool {
	return errors.Is(err, ErrCustomerNameRequired) ||
		errors.Is(err, ErrCustomerNameTooShort) ||
		errors.Is(err, ErrCustomerNameTooLong) ||
		errors.Is(err, ErrCustomerEmailRequired) ||
		errors.Is(err, ErrCustomerEmailInvalid) ||
		errors.Is(err, ErrCustomerEmailTooLong) ||
		errors.Is(err, ErrCustomerPhoneRequired) ||
		errors.Is(err, ErrCustomerPhoneInvalid) ||
		errors.Is(err, ErrCustomerPhoneTooLong) ||
		errors.Is(err, ErrInvalidCustomerID) ||
		errors.Is(err, ErrCustomerPasswordRequired) ||
		errors.Is(err, ErrCustomerPasswordTooShort) ||
		errors.Is(err, ErrCustomerPasswordTooLong)
}
