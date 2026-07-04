package model

import (
	"strings"
	"time"
)

// Customer representa a entidade principal da API.
type Customer struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Phone     string    `json:"phone"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// NewCustomer cria um cliente valido a partir dos dados de entrada.
func NewCustomer(name, email, phone string) (Customer, error) {
	name = strings.TrimSpace(name)
	email = strings.TrimSpace(email)
	phone = strings.TrimSpace(phone)

	if name == "" {
		return Customer{}, ErrCustomerNameRequired
	}
	if email == "" {
		return Customer{}, ErrCustomerEmailRequired
	}
	if phone == "" {
		return Customer{}, ErrCustomerPhoneRequired
	}

	return Customer{
		Name:  name,
		Email: email,
		Phone: phone,
	}, nil
}

// CreateCustomerRequest representa o corpo esperado ao criar um cliente.
type CreateCustomerRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Phone string `json:"phone"`
}

// UpdateCustomerRequest representa o corpo esperado ao atualizar um cliente.
type UpdateCustomerRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Phone string `json:"phone"`
}
