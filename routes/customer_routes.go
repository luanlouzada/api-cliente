package routes

import (
	"cliente-api/controllers"

	"github.com/go-chi/chi/v5"
)

func CustomerRoutes(r *chi.Mux, controller *controllers.CustomerController) {
	r.Post("/cliente", controller.CreateCustomer)
	r.Get("/cliente", controller.FindAllCustomers)
	r.Get("/cliente/{id}", controller.FindCustomerByID)
	r.Put("/cliente/{id}", controller.UpdateCustomer)
	r.Delete("/cliente/{id}", controller.DeleteCustomer)
}
