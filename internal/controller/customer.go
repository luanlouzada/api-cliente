package controller

import (
	"context"
	"log/slog"
	"net/http"

	"cliente-api/internal/dto"
	"cliente-api/internal/mapper"
	"cliente-api/internal/model"
	"cliente-api/internal/view"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// CustomerModel descreve as operações do Model usadas por este Controller.
type CustomerModel interface {
	// Create solicita a criação de um cliente em nome do principal autenticado.
	Create(context.Context, model.Principal, model.CreateCustomerInput) (model.Customer, error)
	// List devolve os clientes que o principal autenticado tem permissão para listar.
	List(context.Context, model.Principal) ([]model.Customer, error)
	// Get busca um cliente por identificador respeitando a autorização do principal.
	Get(context.Context, model.Principal, uuid.UUID) (model.Customer, error)
	// Update altera os campos permitidos do cliente quando o principal pode executar a operação.
	Update(context.Context, model.Principal, uuid.UUID, model.UpdateCustomerInput) (model.Customer, error)
	// Delete remove o cliente indicado quando a política de autorização permite.
	Delete(context.Context, model.Principal, uuid.UUID) error
}

// CustomerController recebe HTTP e coordena DTO, Mapper, Model e View.
// Validação de negócio e autorização permanecem no Model.
type CustomerController struct {
	model  CustomerModel
	logger *slog.Logger
}

// NewCustomerController cria o Controller de clientes com seu Model e logger.
func NewCustomerController(customerModel CustomerModel, logger *slog.Logger) *CustomerController {
	if logger == nil {
		logger = slog.Default()
	}
	return &CustomerController{model: customerModel, logger: logger}
}

// Create recebe um cadastro administrativo e solicita sua criação ao Model.
func (controller *CustomerController) Create(w http.ResponseWriter, request *http.Request) {
	principal, err := principalFromRequest(request)
	if err != nil {
		writeApplicationError(controller.logger, w, request, err)
		return
	}

	var body dto.CreateCustomerRequest
	if err := decodeJSONBody(w, request, &body); err != nil {
		writeApplicationError(controller.logger, w, request, err)
		return
	}

	customer, err := controller.model.Create(request.Context(), principal, mapper.ToCreateCustomerInput(body))
	if err != nil {
		writeApplicationError(controller.logger, w, request, err)
		return
	}
	controller.writeResponse(w, request, http.StatusCreated, mapper.ToCustomerResponse(customer))
}

// List solicita ao Model a lista permitida para o usuário autenticado.
// A autorização não é decidida pelo Controller: ele apenas entrega o principal.
func (controller *CustomerController) List(w http.ResponseWriter, request *http.Request) {
	principal, err := principalFromRequest(request)
	if err != nil {
		writeApplicationError(controller.logger, w, request, err)
		return
	}

	customers, err := controller.model.List(request.Context(), principal)
	if err != nil {
		writeApplicationError(controller.logger, w, request, err)
		return
	}
	controller.writeResponse(w, request, http.StatusOK, mapper.ToCustomerResponses(customers))
}

// Get busca um cliente pelo UUID e respeita a política aplicada pelo Model.
func (controller *CustomerController) Get(w http.ResponseWriter, request *http.Request) {
	id, err := customerIDFromRequest(request)
	if err != nil {
		writeApplicationError(controller.logger, w, request, err)
		return
	}
	principal, err := principalFromRequest(request)
	if err != nil {
		writeApplicationError(controller.logger, w, request, err)
		return
	}

	customer, err := controller.model.Get(request.Context(), principal, id)
	if err != nil {
		writeApplicationError(controller.logger, w, request, err)
		return
	}
	controller.writeResponse(w, request, http.StatusOK, mapper.ToCustomerResponse(customer))
}

// Update decodifica os campos editáveis e delega validação e autorização ao Model.
func (controller *CustomerController) Update(w http.ResponseWriter, request *http.Request) {
	id, err := customerIDFromRequest(request)
	if err != nil {
		writeApplicationError(controller.logger, w, request, err)
		return
	}
	principal, err := principalFromRequest(request)
	if err != nil {
		writeApplicationError(controller.logger, w, request, err)
		return
	}

	var body dto.UpdateCustomerRequest
	if err := decodeJSONBody(w, request, &body); err != nil {
		writeApplicationError(controller.logger, w, request, err)
		return
	}

	customer, err := controller.model.Update(request.Context(), principal, id, mapper.ToUpdateCustomerInput(body))
	if err != nil {
		writeApplicationError(controller.logger, w, request, err)
		return
	}
	controller.writeResponse(w, request, http.StatusOK, mapper.ToCustomerResponse(customer))
}

// Delete remove o cliente indicado quando o Model autoriza a operação.
// O sucesso usa 204 porque uma exclusão não precisa devolver representação no corpo.
func (controller *CustomerController) Delete(w http.ResponseWriter, request *http.Request) {
	id, err := customerIDFromRequest(request)
	if err != nil {
		writeApplicationError(controller.logger, w, request, err)
		return
	}
	principal, err := principalFromRequest(request)
	if err != nil {
		writeApplicationError(controller.logger, w, request, err)
		return
	}
	if err := controller.model.Delete(request.Context(), principal, id); err != nil {
		writeApplicationError(controller.logger, w, request, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// principalFromRequest transforma as claims no principal entendido pelo Model.
// Claims ausentes, UUID nulo ou UUID inválido são tratados como acesso proibido por segurança.
func principalFromRequest(request *http.Request) (model.Principal, error) {
	claims, ok := ClaimsFromContext(request.Context())
	if !ok {
		return model.Principal{}, model.ErrForbidden
	}
	authenticatedCustomerID, err := uuid.Parse(claims.Subject)
	if err != nil || authenticatedCustomerID == uuid.Nil {
		return model.Principal{}, model.ErrForbidden
	}
	return model.Principal{CustomerID: authenticatedCustomerID, Role: claims.Role}, nil
}

// writeResponse centraliza a serialização das respostas de cliente e registra falhas de escrita.
func (controller *CustomerController) writeResponse(
	w http.ResponseWriter,
	request *http.Request,
	status int,
	value any,
) {
	if err := view.WriteJSON(w, status, value); err != nil {
		controller.logger.Error(
			"falha ao escrever a resposta de cliente",
			"path", request.URL.Path,
			"error", err,
		)
	}
}

// customerIDFromRequest valida o UUID chamado "id" antes de chamar o Model.
func customerIDFromRequest(request *http.Request) (uuid.UUID, error) {
	id, err := uuid.Parse(chi.URLParam(request, "id"))
	if err != nil {
		return uuid.Nil, model.ErrInvalidCustomerID
	}
	return id, nil
}
