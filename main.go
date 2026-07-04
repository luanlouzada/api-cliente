package main

import (
	"cliente-api/controllers"
	"cliente-api/repository"
	"cliente-api/routes"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	customerRepository := repository.NewCustomerRepository()
	customerController := controllers.NewCustomerController(customerRepository)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	routes.CustomerRoutes(r, customerController)

	log.Println("API rodando em http://localhost:8080")
	log.Println("POST   /cliente      -> criar cliente")
	log.Println("GET    /cliente      -> listar clientes")
	log.Println("GET    /cliente/{id} -> buscar por id")

	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatalf("erro ao iniciar servidor: %v", err)
	}
}
