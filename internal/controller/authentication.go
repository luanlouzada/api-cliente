package controller

import (
	"context"
	"log/slog"
	"net/http"

	"cliente-api/internal/dto"
	"cliente-api/internal/mapper"
	"cliente-api/internal/model"
	"cliente-api/internal/view"
)

// AuthenticationModel descreve as operações do Model usadas por este Controller.
type AuthenticationModel interface {
	// Register cria um cliente e a primeira sessão a partir dos dados recebidos no cadastro.
	Register(context.Context, model.RegisterInput) (model.AuthenticationResult, error)
	// Login valida as credenciais e cria uma nova sessão para o cliente existente.
	Login(context.Context, model.LoginInput) (model.AuthenticationResult, error)
	// Refresh rotaciona a sessão identificada pelo refresh token apresentado.
	Refresh(context.Context, string) (model.AuthenticationResult, error)
	// Logout revoga a sessão identificada pelo refresh token apresentado.
	Logout(context.Context, string) error
}

// AuthenticationController recebe HTTP e coordena DTO, Mapper, Model e View.
// Regras de autenticação permanecem no Model.
type AuthenticationController struct {
	model  AuthenticationModel
	logger *slog.Logger
}

// NewAuthenticationController cria o Controller com o Model e o logger usados
// para atender as rotas de autenticação.
func NewAuthenticationController(
	authenticationModel AuthenticationModel,
	logger *slog.Logger,
) *AuthenticationController {
	if logger == nil {
		logger = slog.Default()
	}
	return &AuthenticationController{model: authenticationModel, logger: logger}
}

// Register recebe o cadastro, converte o DTO em entrada do Model e devolve a
// sessão criada. Regras de negócio permanecem no Model.
func (controller *AuthenticationController) Register(
	w http.ResponseWriter,
	request *http.Request,
) {
	var body dto.CreateCustomerRequest
	if err := decodeJSONBody(w, request, &body); err != nil {
		writeApplicationError(controller.logger, w, request, err)
		return
	}

	result, err := controller.model.Register(request.Context(), mapper.ToRegisterInput(body))
	if err != nil {
		writeApplicationError(controller.logger, w, request, err)
		return
	}
	controller.writeAuthenticationResponse(w, request, http.StatusCreated, result)
}

// Login autentica credenciais enviadas por HTTP e responde com os tokens e o cliente autenticado.
// Erros são encaminhados ao tradutor central para preservar o mesmo contrato JSON em toda a API.
func (controller *AuthenticationController) Login(w http.ResponseWriter, request *http.Request) {
	var body dto.LoginRequest
	if err := decodeJSONBody(w, request, &body); err != nil {
		writeApplicationError(controller.logger, w, request, err)
		return
	}

	result, err := controller.model.Login(request.Context(), mapper.ToLoginInput(body))
	if err != nil {
		writeApplicationError(controller.logger, w, request, err)
		return
	}
	controller.writeAuthenticationResponse(w, request, http.StatusOK, result)
}

// Refresh troca um refresh token válido por um novo par de tokens.
// O Controller coordena entrada e saída; rotação, expiração e tentativa de
// reutilizar um token anterior são decisões do Model.
func (controller *AuthenticationController) Refresh(w http.ResponseWriter, request *http.Request) {
	body, ok := controller.decodeRefreshTokenRequest(w, request)
	if !ok {
		return
	}

	result, err := controller.model.Refresh(request.Context(), body.RefreshToken)
	if err != nil {
		writeApplicationError(controller.logger, w, request, err)
		return
	}
	controller.writeAuthenticationResponse(w, request, http.StatusOK, result)
}

// Logout encerra a família de sessão identificada pelo refresh token e retorna 204 sem corpo.
func (controller *AuthenticationController) Logout(w http.ResponseWriter, request *http.Request) {
	body, ok := controller.decodeRefreshTokenRequest(w, request)
	if !ok {
		return
	}
	if err := controller.model.Logout(request.Context(), body.RefreshToken); err != nil {
		writeApplicationError(controller.logger, w, request, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// decodeRefreshTokenRequest decodifica o corpo comum a refresh e logout.
// O booleano informa ao chamador se a resposta de erro já foi escrita e evita escrita duplicada.
func (controller *AuthenticationController) decodeRefreshTokenRequest(
	w http.ResponseWriter,
	request *http.Request,
) (dto.RefreshTokenRequest, bool) {
	var body dto.RefreshTokenRequest
	if err := decodeJSONBody(w, request, &body); err != nil {
		writeApplicationError(controller.logger, w, request, err)
		return dto.RefreshTokenRequest{}, false
	}
	return body, true
}

// writeAuthenticationResponse converte o resultado do Model para o DTO público e o entrega à View.
// Os cabeçalhos impedem que navegadores e proxies armazenem respostas que contêm credenciais.
func (controller *AuthenticationController) writeAuthenticationResponse(
	w http.ResponseWriter,
	request *http.Request,
	status int,
	result model.AuthenticationResult,
) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	if err := view.WriteJSON(w, status, mapper.ToAuthenticationResponse(result)); err != nil {
		controller.logger.Error(
			"falha ao escrever a resposta de autenticação",
			"path", request.URL.Path,
			"error", err,
		)
	}
}
