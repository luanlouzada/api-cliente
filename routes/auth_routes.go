package routes

import (
	"cliente-api/controllers"

	"github.com/go-chi/chi/v5"
)

func AuthRoutes(r chi.Router, controller *controllers.AuthController) {
	r.Post("/auth/register", controller.Register)
	r.Post("/auth/login", controller.Login)
	r.Post("/auth/refresh", controller.Refresh)
	r.Post("/auth/logout", controller.Logout)
}
