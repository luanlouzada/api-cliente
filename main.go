package main

import (
	"context"
	"log"
	"net/http"

	"cliente-api/config"
	"cliente-api/controllers"
	"cliente-api/database"
	"cliente-api/repository"
	"cliente-api/routes"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	ctx := context.Background()

	cfg := config.Load()

	pool, err := database.NewPostgresPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("erro ao conectar no banco: %v", err)
	}
	defer pool.Close()

	customerRepository := repository.NewCustomerRepository(pool)
	customerController := controllers.NewCustomerController(customerRepository)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	routes.CustomerRoutes(r, customerController)

	log.Printf("API rodando em http://localhost:%s\n", cfg.Port)
	log.Println("POST   /cliente      -> criar cliente")
	log.Println("GET    /cliente      -> listar clientes")
	log.Println("GET    /cliente/{id} -> buscar por id")
	log.Println("PUT    /cliente/{id} -> atualizar cliente")
	log.Println("DELETE /cliente/{id} -> remover cliente")

	if err := http.ListenAndServe(":"+cfg.Port, r); err != nil {
		log.Fatalf("erro ao iniciar servidor: %v", err)
	}
}
