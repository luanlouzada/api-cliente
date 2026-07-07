package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"cliente-api/model"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type CustomerRepository interface {
	Create(ctx context.Context, customer model.Customer) (model.Customer, error)
	FindAll(ctx context.Context) ([]model.Customer, error)
	FindByID(ctx context.Context, id string) (model.Customer, error)
	Update(ctx context.Context, id string, customer model.Customer) (model.Customer, error)
	Delete(ctx context.Context, id string) error
}

type CustomerController struct {
	repository CustomerRepository
}

func NewCustomerController(repository CustomerRepository) *CustomerController {
	return &CustomerController{repository: repository}
}

func customerIDFromRequest(r *http.Request) (string, error) {
	id := chi.URLParam(r, "id")
	if _, err := uuid.Parse(id); err != nil {
		return "", errors.New("id deve ser um uuid valido")
	}
	return id, nil
}

func writeCustomerError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, model.ErrCustomerNotFound):
		http.Error(w, err.Error(), http.StatusNotFound)
	case errors.Is(err, model.ErrCustomerEmailAlreadyExists):
		http.Error(w, err.Error(), http.StatusConflict)
	default:
		log.Printf("erro interno ao processar cliente: %v", err)
		http.Error(w, "erro interno do servidor", http.StatusInternalServerError)
	}
}

func (c *CustomerController) CreateCustomer(w http.ResponseWriter, r *http.Request) {
	var request model.CreateCustomerRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	customer, err := model.NewCustomer(request.Name, request.Email, request.Phone)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	customer, err = c.repository.Create(r.Context(), customer)
	if err != nil {
		writeCustomerError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(customer)
}

func (c *CustomerController) FindAllCustomers(w http.ResponseWriter, r *http.Request) {
	customers, err := c.repository.FindAll(r.Context())
	if err != nil {
		writeCustomerError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(customers)
}

func (c *CustomerController) FindCustomerByID(w http.ResponseWriter, r *http.Request) {
	id, err := customerIDFromRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	customer, err := c.repository.FindByID(r.Context(), id)
	if err != nil {
		writeCustomerError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(customer)
}

func (c *CustomerController) UpdateCustomer(w http.ResponseWriter, r *http.Request) {
	id, err := customerIDFromRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var request model.UpdateCustomerRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	customer, err := model.NewCustomer(request.Name, request.Email, request.Phone)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	customer, err = c.repository.Update(r.Context(), id, customer)
	if err != nil {
		writeCustomerError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(customer)
}

func (c *CustomerController) DeleteCustomer(w http.ResponseWriter, r *http.Request) {
	id, err := customerIDFromRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := c.repository.Delete(r.Context(), id); err != nil {
		writeCustomerError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
