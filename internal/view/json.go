package view

import (
	"encoding/json"
	"net/http"

	"cliente-api/internal/dto"
)

// WriteJSON transforma um DTO em JSON e publica a representação HTTP. A
// serialização acontece antes do status para que uma falha ainda possa ser
// tratada pelo Controller.
func WriteJSON(w http.ResponseWriter, status int, value any) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	payload = append(payload, '\n')

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, err = w.Write(payload)
	return err
}

// WriteError monta e publica a View JSON uniforme usada nos erros da API.
func WriteError(w http.ResponseWriter, status int, code, message string) error {
	return WriteJSON(w, status, dto.ErrorResponse{
		Error: dto.ErrorDetail{Code: code, Message: message},
	})
}
