package dto

import "time"

// LoginRequest representa exclusivamente os campos aceitos pelo endpoint de login.
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// RefreshTokenRequest representa o corpo compartilhado pelos endpoints de refresh e logout.
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// AuthenticationResponse é o contrato público devolvido ao criar ou renovar uma sessão.
// Ele contém segredos de autenticação e, por isso, deve sempre ser enviado com cache desabilitado.
type AuthenticationResponse struct {
	AccessToken      string           `json:"access_token"`
	TokenType        string           `json:"token_type"`
	ExpiresAt        time.Time        `json:"expires_at"`
	RefreshToken     string           `json:"refresh_token"`
	RefreshExpiresAt time.Time        `json:"refresh_expires_at"`
	SessionExpiresAt time.Time        `json:"session_expires_at"`
	Customer         CustomerResponse `json:"customer"`
}
