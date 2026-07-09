package main

import (
	"context"
	"log"
	"net/http"

	"cliente-api/auth"
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

	tokenManager, err := auth.NewTokenManager(cfg.Auth.JWTSecret, cfg.Auth.AccessTokenTTL)
	if err != nil {
		log.Fatalf("erro ao configurar JWT: %v", err)
	}
	authController := controllers.NewAuthController(customerRepository, tokenManager)

	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	routes.FrontendRoutes(r)

	routes.AuthRoutes(r, authController)

	r.Group(func(r chi.Router) {
		r.Use(tokenManager.Middleware)

		r.Use(auth.AuthLogger)
		routes.CustomerRoutes(r, customerController)
	})

	log.Printf("API rodando em http://localhost:%s\n", cfg.Port)
	log.Println("GET    /               -> frontend de demonstracao")
	log.Println("POST   /auth/register -> cadastrar customer e retornar JWT")
	log.Println("POST   /auth/login    -> autenticar customer e retornar JWT")
	log.Println("POST   /cliente       -> criar cliente (JWT)")
	log.Println("GET    /cliente       -> listar clientes (JWT)")
	log.Println("GET    /cliente/{id}  -> buscar por id (JWT)")
	log.Println("PUT    /cliente/{id}  -> atualizar cliente (JWT)")
	log.Println("DELETE /cliente/{id}  -> remover cliente (JWT)")

	if err := http.ListenAndServe(":"+cfg.Port, r); err != nil {
		log.Fatalf("erro ao iniciar servidor: %v", err)
	}
}
