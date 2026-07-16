package controller

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
)

const maxJSONBodyBytes = 64 * 1024

var (
	// ErrUnsupportedMediaType indica que o corpo não foi enviado como JSON.
	ErrUnsupportedMediaType = errors.New("content-type deve ser application/json")
	// ErrBodyTooLarge indica que o corpo ultrapassou o limite aceito pela API.
	ErrBodyTooLarge = errors.New("corpo da requisição excede o limite permitido")
	// ErrEmptyBody indica que a requisição não forneceu o objeto JSON obrigatório.
	ErrEmptyBody = errors.New("corpo da requisição não pode ser vazio")
	// ErrMultipleJSONObjects indica que há dados adicionais depois do primeiro objeto JSON.
	ErrMultipleJSONObjects = errors.New("corpo da requisição deve conter apenas um objeto JSON")
	// ErrInvalidJSON indica que o corpo não pôde ser decodificado no DTO esperado.
	ErrInvalidJSON = errors.New("JSON inválido")
)

// decodeJSONBody valida Content-Type e decodifica exatamente um objeto JSON no destino.
// O limite de tamanho e a rejeição de campos desconhecidos reduzem ambiguidades e abuso da API.
func decodeJSONBody(w http.ResponseWriter, request *http.Request, destination any) error {
	mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || !strings.EqualFold(mediaType, "application/json") {
		return ErrUnsupportedMediaType
	}

	limitedBody := http.MaxBytesReader(w, request.Body, maxJSONBodyBytes)
	decoder := json.NewDecoder(limitedBody)
	var rawBody json.RawMessage

	// A primeira leitura preserva o JSON bruto enquanto aplica o limite de 64 KiB.
	// Preservá-lo permite conferir o formato inteiro antes de preencher o DTO.
	if err := decoder.Decode(&rawBody); err != nil {
		if errors.Is(err, io.EOF) {
			return ErrEmptyBody
		}
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			return ErrBodyTooLarge
		}
		return fmt.Errorf("%w: %w", ErrInvalidJSON, err)
	}

	// Uma segunda leitura deve encontrar o fim do corpo. Se encontrar outro valor,
	// a requisição tentou enviar dois documentos JSON no mesmo corpo.
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			return ErrBodyTooLarge
		}
		return ErrMultipleJSONObjects
	}

	trimmedBody := bytes.TrimSpace(rawBody)
	if len(trimmedBody) == 0 || trimmedBody[0] != '{' {
		return fmt.Errorf("%w: o corpo deve ser um objeto JSON", ErrInvalidJSON)
	}

	strictDecoder := json.NewDecoder(bytes.NewReader(rawBody))
	// O segundo Decoder conhece o tipo do DTO e rejeita nomes de campos que esse
	// tipo não declara, evitando aceitar dados que a API ignoraria silenciosamente.
	strictDecoder.DisallowUnknownFields()
	if err := strictDecoder.Decode(destination); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidJSON, err)
	}
	return nil
}
