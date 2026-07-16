package mapper

import (
	"cliente-api/internal/dto"
	"cliente-api/internal/model"
)

// ToCreateCustomerInput converte o DTO recebido na entrada de criação do Model.
// Nenhuma regra é aplicada aqui: o Mapper apenas copia os campos necessários.
func ToCreateCustomerInput(request dto.CreateCustomerRequest) model.CreateCustomerInput {
	return model.CreateCustomerInput{
		Name:     request.Name,
		Email:    request.Email,
		Phone:    request.Phone,
		Password: request.Password,
	}
}

// ToUpdateCustomerInput seleciona somente os campos públicos permitidos na
// atualização de cliente.
func ToUpdateCustomerInput(request dto.UpdateCustomerRequest) model.UpdateCustomerInput {
	return model.UpdateCustomerInput{
		Name:  request.Name,
		Email: request.Email,
		Phone: request.Phone,
	}
}

// ToCustomerResponse transforma o Customer do Model na representação segura
// devolvida pela API. PasswordHash não faz parte do DTO e, portanto, não pode
// vazar por serialização acidental.
func ToCustomerResponse(customer model.Customer) dto.CustomerResponse {
	return dto.CustomerResponse{
		ID:        customer.ID,
		Name:      customer.Name,
		Email:     customer.Email,
		Phone:     customer.Phone,
		Role:      string(customer.Role),
		CreatedAt: customer.CreatedAt,
		UpdatedAt: customer.UpdatedAt,
	}
}

// ToCustomerResponses converte uma coleção preservando a ordem. make cria uma
// slice não nil mesmo quando a entrada é nil, para a View produzir [] em vez de
// null sem levar essa preocupação de apresentação para o Model.
func ToCustomerResponses(customers []model.Customer) []dto.CustomerResponse {
	responses := make([]dto.CustomerResponse, 0, len(customers))
	for _, customer := range customers {
		responses = append(responses, ToCustomerResponse(customer))
	}
	return responses
}
