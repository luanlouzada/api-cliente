package controllers

import (
	"cliente-api/model"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

type CustomerRepository interface {
	Create(customer model.Customer) model.Customer
	FindAll() []model.Customer
	FindByID(id int) (model.Customer, error)
	Update(id int, customer model.Customer) (model.Customer, error)
	Delete(id int) error
}

type CustomerController struct {
	repository CustomerRepository
}

type ErrorResponse struct {
	Error string `json:"error"`
}

func NewCustomerController(repository CustomerRepository) *CustomerController {
	return &CustomerController{repository: repository}
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
	customer = c.repository.Create(customer)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(customer)

}

func (c *CustomerController) FindAllCustomers(w http.ResponseWriter, r *http.Request) {
	customers := c.repository.FindAll()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(customers)
}

// cliente/1
func (c *CustomerController) FindCustomerByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	idInt, err := strconv.Atoi(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	customer, err := c.repository.FindByID(idInt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(customer)
}

func (c *CustomerController) UpdateCustomer(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	idInt, err := strconv.Atoi(id)
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
	customer, err = c.repository.Update(idInt, customer)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(customer)
}

func (c *CustomerController) DeleteCustomer(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	idInt, err := strconv.Atoi(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := c.repository.Delete(idInt); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
