package dto

// ErrorResponse define o envelope uniforme de erro devolvido por todos os endpoints.
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail separa um código estável para máquinas de uma mensagem legível para pessoas.
type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
