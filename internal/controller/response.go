package controller

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"cliente-api/internal/model"
	"cliente-api/internal/view"
)

// ErrBearerTokenRequired indica ausência ou formato inválido do cabeçalho Authorization.
var ErrBearerTokenRequired = errors.New("o cabeçalho Authorization deve usar o esquema Bearer")

// writeApplicationError traduz erros do Model para o contrato público. Detalhes
// inesperados ficam apenas no log, sem expor implementação ou dados sensíveis.
func writeApplicationError(
	logger *slog.Logger,
	w http.ResponseWriter,
	request *http.Request,
	err error,
) {
	if errors.Is(err, model.ErrForbidden) {
		w.Header().Set("WWW-Authenticate", `Bearer error="insufficient_scope"`)
	}
	status, code, message, isInternal := classifyError(err)
	if errors.Is(err, ErrInvalidJSON) {
		logger.Debug(
			"corpo JSON inválido",
			"method", request.Method,
			"path", request.URL.Path,
			"error", err,
		)
	} else if isInternal {
		logger.Error(
			"falha na requisição",
			"method", request.Method,
			"path", request.URL.Path,
			"error", err,
		)
	}
	if writeErr := view.WriteError(w, status, code, message); writeErr != nil {
		logger.Error("falha ao escrever a resposta de erro", "error", writeErr)
	}
}

// classifyError associa erros conhecidos a status, código e mensagem seguros para o consumidor.
// O último retorno indica se o erro deve ser registrado como falha interna pela aplicação.
func classifyError(err error) (status int, code, message string, internal bool) {
	switch {
	case errors.Is(err, ErrUnsupportedMediaType):
		return http.StatusUnsupportedMediaType, "unsupported_media_type", err.Error(), false
	case errors.Is(err, ErrBodyTooLarge):
		return http.StatusRequestEntityTooLarge, "payload_too_large", err.Error(), false
	case errors.Is(err, ErrEmptyBody),
		errors.Is(err, ErrMultipleJSONObjects):
		return http.StatusBadRequest, "invalid_request", err.Error(), false
	case errors.Is(err, ErrInvalidJSON):
		return http.StatusBadRequest, "invalid_request", ErrInvalidJSON.Error(), false
	case model.IsValidationError(err):
		return http.StatusBadRequest, "validation_error", err.Error(), false
	case errors.Is(err, model.ErrCustomerNotFound):
		return http.StatusNotFound, "not_found", model.ErrCustomerNotFound.Error(), false
	case errors.Is(err, model.ErrCustomerEmailAlreadyExists):
		return http.StatusConflict, "conflict", model.ErrCustomerEmailAlreadyExists.Error(), false
	case errors.Is(err, model.ErrInvalidCredentials),
		errors.Is(err, model.ErrRefreshTokenInvalid),
		errors.Is(err, model.ErrRefreshTokenExpired),
		errors.Is(err, model.ErrRefreshTokenReused):
		message := model.ErrInvalidCredentials.Error()
		if !errors.Is(err, model.ErrInvalidCredentials) {
			message = model.ErrRefreshTokenInvalid.Error()
		}
		return http.StatusUnauthorized, "unauthorized", message, false
	case errors.Is(err, ErrBearerTokenRequired):
		return http.StatusUnauthorized, "unauthorized", ErrBearerTokenRequired.Error(), false
	case errors.Is(err, model.ErrForbidden):
		return http.StatusForbidden, "forbidden", model.ErrForbidden.Error(), false
	case errors.Is(err, model.ErrAccessTokenExpired):
		return http.StatusUnauthorized, "unauthorized", model.ErrAccessTokenExpired.Error(), false
	case errors.Is(err, model.ErrAccessTokenInvalid):
		return http.StatusUnauthorized, "unauthorized", model.ErrAccessTokenInvalid.Error(), false
	case errors.Is(err, context.DeadlineExceeded):
		return http.StatusGatewayTimeout, "timeout", "tempo limite da requisição excedido", false
	case errors.Is(err, context.Canceled):
		return http.StatusRequestTimeout, "request_canceled", "requisição cancelada", false
	default:
		return http.StatusInternalServerError, "internal_error", "erro interno do servidor", true
	}
}
