package auth

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"
)

const maxJSONBodyBytes = 64 * 1024

var (
	ErrInvalidContentType  = errors.New("content-type deve ser application/json")
	ErrBodyTooLarge        = errors.New("corpo da requisicao excede o limite permitido")
	ErrEmptyBody           = errors.New("corpo da requisicao nao pode ser vazio")
	ErrMultipleJSONObjects = errors.New("corpo da requisicao deve conter apenas um objeto json")
)

func DecodeJSONBody(w http.ResponseWriter, r *http.Request, destination any) error {

	contentType := r.Header.Get("Content-Type")
	mediaType, _, err := mime.ParseMediaType(contentType)

	if err != nil || !strings.EqualFold(mediaType, "application/json") {
		return ErrInvalidContentType
	}

	limitedBody := http.MaxBytesReader(w, r.Body, maxJSONBodyBytes)
	decoder := json.NewDecoder(limitedBody)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(destination); err != nil {
		if errors.Is(err, io.EOF) {
			return ErrEmptyBody
		}

		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			return ErrBodyTooLarge
		}
		return err
	}

	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return ErrMultipleJSONObjects
	}

	return nil
}
