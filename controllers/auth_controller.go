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
)

type CustomerAuthRepository interface {
	Create(ctx context.Context, customer model.Customer) (model.Customer, error)
	FindByEmail(ctx context.Context, email string) (model.Customer, error)
}

type TokenIssuer interface {
	Generate(customer model.Customer) (string, time.Time, error)
}

type AuthController struct {
	repository  CustomerAuthRepository
	tokenIssuer TokenIssuer
}

func NewAuthController(repository CustomerAuthRepository, tokenIssuer TokenIssuer) *AuthController {
	return &AuthController{
		repository:  repository,
		tokenIssuer: tokenIssuer,
	}
}

func writeAuthError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, model.ErrCustomerEmailAlreadyExists):
		http.Error(w, err.Error(), http.StatusConflict)
	case errors.Is(err, model.ErrInvalidCredentials):
		http.Error(w, err.Error(), http.StatusUnauthorized)
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

	customer, err = c.repository.Create(r.Context(), customer)
	if err != nil {
		writeAuthError(w, err)
		return
	}

	c.writeTokenResponse(w, http.StatusCreated, customer)
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

	c.writeTokenResponse(w, http.StatusOK, customer)
}

func (c *AuthController) writeTokenResponse(w http.ResponseWriter, status int, customer model.Customer) {
	token, expiresAt, err := c.tokenIssuer.Generate(customer)
	if err != nil {
		writeAuthError(w, err)
		return
	}

	response := dto.AuthResponse{
		AccessToken: token,
		TokenType:   "Bearer",
		ExpiresAt:   expiresAt,
		Customer:    dto.NewCustomerResponse(customer),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(response)
}
