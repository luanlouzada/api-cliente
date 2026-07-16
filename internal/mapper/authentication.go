package mapper

import (
	"cliente-api/internal/dto"
	"cliente-api/internal/model"
)

// ToRegisterInput converte o DTO de cadastro HTTP para a entrada do Model.
// A função apenas copia valores; normalização e validação pertencem ao Model.
func ToRegisterInput(request dto.CreateCustomerRequest) model.RegisterInput {
	return model.RegisterInput(ToCreateCustomerInput(request))
}

// ToLoginInput converte as credenciais recebidas no formato esperado pelo Model.
func ToLoginInput(request dto.LoginRequest) model.LoginInput {
	return model.LoginInput{Email: request.Email, Password: request.Password}
}

// ToAuthenticationResponse cria o DTO público a partir do resultado do Model.
// O Mapper fixa o esquema Bearer e reutiliza a conversão de cliente para não
// expor campos internos sensíveis. A View serializará esse DTO como JSON.
func ToAuthenticationResponse(result model.AuthenticationResult) dto.AuthenticationResponse {
	return dto.AuthenticationResponse{
		AccessToken:      result.AccessToken,
		TokenType:        "Bearer",
		ExpiresAt:        result.AccessExpiresAt,
		RefreshToken:     result.RefreshToken,
		RefreshExpiresAt: result.RefreshExpiresAt,
		SessionExpiresAt: result.SessionExpiresAt,
		Customer:         ToCustomerResponse(result.Customer),
	}
}
