package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"cliente-api/auth"
	"cliente-api/dto"
	"cliente-api/model"

	"github.com/google/uuid"
)

type CustomerAuthRepository interface {
	CreateWithRefreshToken(ctx context.Context, customer model.Customer, token model.RefreshToken) (model.Customer, error)
	FindByEmail(ctx context.Context, email string) (model.Customer, error)
	CreateRefreshToken(ctx context.Context, token model.RefreshToken) error
	RotateRefreshToken(ctx context.Context, currentHash []byte, replacement model.RefreshToken) (model.Customer, model.RefreshToken, error)
	RevokeRefreshToken(ctx context.Context, tokenHash []byte) error
}

type TokenIssuer interface {
	Generate(customer model.Customer) (string, time.Time, error)
}

type RefreshTokenIssuer interface {
	Generate() (string, model.RefreshToken, error)
	Hash(rawToken string) ([]byte, error)
}

type AuthController struct {
	repository         CustomerAuthRepository
	tokenIssuer        TokenIssuer
	refreshTokenIssuer RefreshTokenIssuer
}

func NewAuthController(
	repository CustomerAuthRepository,
	tokenIssuer TokenIssuer,
	refreshTokenIssuer RefreshTokenIssuer,
) *AuthController {
	return &AuthController{
		repository:         repository,
		tokenIssuer:        tokenIssuer,
		refreshTokenIssuer: refreshTokenIssuer,
	}
}

func writeAuthError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, model.ErrCustomerEmailAlreadyExists):
		http.Error(w, err.Error(), http.StatusConflict)
	case errors.Is(err, model.ErrInvalidCredentials):
		http.Error(w, err.Error(), http.StatusUnauthorized)
	case errors.Is(err, model.ErrRefreshTokenInvalid),
		errors.Is(err, model.ErrRefreshTokenExpired),
		errors.Is(err, model.ErrRefreshTokenReused):
		http.Error(w, model.ErrRefreshTokenInvalid.Error(), http.StatusUnauthorized)
	case errors.Is(err, auth.ErrInvalidContentType),
		errors.Is(err, auth.ErrBodyTooLarge):
		http.Error(w, err.Error(), http.StatusBadRequest)
	default:
		log.Printf("erro interno ao processar autenticacao: %v", err)
		http.Error(w, "erro interno do servidor", http.StatusInternalServerError)
	}
}

func (c *AuthController) Register(w http.ResponseWriter, r *http.Request) {
	var request dto.CreateCustomerRequest
	if err := auth.DecodeJSONBody(w, r, &request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := model.ValidateCustomerPassword(request.Password); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	passwordHash, err := auth.HashPassword(request.Password)
	if err != nil {
		writeAuthError(w, err)
		return
	}

	customer, err := model.NewCustomer(request.Name, request.Email, request.Phone, passwordHash)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	rawRefreshToken, refreshToken, err := c.newRefreshToken(uuid.Nil)
	if err != nil {
		writeAuthError(w, err)
		return
	}

	customer, err = c.repository.CreateWithRefreshToken(r.Context(), customer, refreshToken)
	if err != nil {
		writeAuthError(w, err)
		return
	}

	accessToken, accessExpiresAt, err := c.tokenIssuer.Generate(customer)
	if err != nil {
		writeAuthError(w, err)
		return
	}

	c.writeTokenResponse(
		w,
		http.StatusCreated,
		customer,
		accessToken,
		accessExpiresAt,
		rawRefreshToken,
		refreshToken,
	)
}

func (c *AuthController) Login(w http.ResponseWriter, r *http.Request) {
	var request dto.LoginRequest
	if err := auth.DecodeJSONBody(w, r, &request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	email := model.NormalizeCustomerEmail(request.Email)
	if err := model.ValidateCustomerEmail(email); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if request.Password == "" {
		http.Error(w, model.ErrCustomerPasswordRequired.Error(), http.StatusBadRequest)
		return
	}

	customer, err := c.repository.FindByEmail(r.Context(), email)
	if errors.Is(err, model.ErrCustomerNotFound) {
		writeAuthError(w, model.ErrInvalidCredentials)
		return
	}
	if err != nil {
		writeAuthError(w, err)
		return
	}

	if !auth.CheckPassword(request.Password, customer.PasswordHash) {
		writeAuthError(w, model.ErrInvalidCredentials)
		return
	}

	c.issueTokenPair(r.Context(), w, http.StatusOK, customer)
}

func (c *AuthController) Refresh(w http.ResponseWriter, r *http.Request) {
	var request dto.RefreshTokenRequest
	if err := auth.DecodeJSONBody(w, r, &request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	currentHash, err := c.refreshTokenIssuer.Hash(request.RefreshToken)
	if err != nil {
		writeAuthError(w, err)
		return
	}

	rawReplacement, replacement, err := c.refreshTokenIssuer.Generate()
	if err != nil {
		writeAuthError(w, err)
		return
	}

	customer, replacement, err := c.repository.RotateRefreshToken(r.Context(), currentHash, replacement)
	if err != nil {
		writeAuthError(w, err)
		return
	}

	accessToken, accessExpiresAt, err := c.tokenIssuer.Generate(customer)
	if err != nil {
		writeAuthError(w, err)
		return
	}

	c.writeTokenResponse(
		w,
		http.StatusOK,
		customer,
		accessToken,
		accessExpiresAt,
		rawReplacement,
		replacement,
	)
}

func (c *AuthController) Logout(w http.ResponseWriter, r *http.Request) {
	var request dto.RefreshTokenRequest
	if err := auth.DecodeJSONBody(w, r, &request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	tokenHash, err := c.refreshTokenIssuer.Hash(request.RefreshToken)
	if err == nil {
		if err := c.repository.RevokeRefreshToken(r.Context(), tokenHash); err != nil {
			writeAuthError(w, err)
			return
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

func (c *AuthController) issueTokenPair(
	ctx context.Context,
	w http.ResponseWriter,
	status int,
	customer model.Customer,
) {
	accessToken, accessExpiresAt, err := c.tokenIssuer.Generate(customer)
	if err != nil {
		writeAuthError(w, err)
		return
	}

	rawRefreshToken, refreshToken, err := c.newRefreshToken(customer.ID)
	if err != nil {
		writeAuthError(w, err)
		return
	}

	if err := c.repository.CreateRefreshToken(ctx, refreshToken); err != nil {
		writeAuthError(w, err)
		return
	}

	c.writeTokenResponse(
		w,
		status,
		customer,
		accessToken,
		accessExpiresAt,
		rawRefreshToken,
		refreshToken,
	)
}

func (c *AuthController) newRefreshToken(customerID uuid.UUID) (string, model.RefreshToken, error) {
	rawToken, token, err := c.refreshTokenIssuer.Generate()
	if err != nil {
		return "", model.RefreshToken{}, err
	}

	familyID, err := uuid.NewV7()
	if err != nil {
		return "", model.RefreshToken{}, err
	}
	token.CustomerID = customerID
	token.FamilyID = familyID
	return rawToken, token, nil
}

func (c *AuthController) writeTokenResponse(
	w http.ResponseWriter,
	status int,
	customer model.Customer,
	accessToken string,
	accessExpiresAt time.Time,
	rawRefreshToken string,
	refreshToken model.RefreshToken,
) {

	response := dto.AuthResponse{
		AccessToken:      accessToken,
		TokenType:        "Bearer",
		ExpiresAt:        accessExpiresAt,
		RefreshToken:     rawRefreshToken,
		RefreshExpiresAt: refreshToken.ExpiresAt,
		SessionExpiresAt: refreshToken.FamilyExpiresAt,
		Customer:         dto.NewCustomerResponse(customer),
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("erro ao escrever resposta de autenticacao: %v", err)
	}
}
