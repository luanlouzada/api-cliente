package dto

import (
	"time"

	"github.com/google/uuid"
)

// CreateCustomerRequest define os campos aceitos no cadastro de um cliente.
type CreateCustomerRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Password string `json:"password"`
}

// UpdateCustomerRequest restringe a atualização aos dados públicos editáveis.
// Papel e senha ficam de fora para não permitir mudanças por atribuição indevida de campos.
type UpdateCustomerRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Phone string `json:"phone"`
}

// CustomerResponse é a representação pública de um cliente na API.
// O tipo não contém hash de senha nem outros detalhes internos do Model.
type CustomerResponse struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Phone     string    `json:"phone"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
