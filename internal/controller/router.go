package controller

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"cliente-api/internal/view"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// RouterDependencies reúne todas as dependências necessárias para compor a fronteira HTTP.
type RouterDependencies struct {
	Authentication *AuthenticationController
	Customers      *CustomerController
	TokenValidator AccessTokenValidator
	Frontend       *FrontendController
	Logger         *slog.Logger
}

// NewRouter compõe rotas, handlers e intermediários na fronteira HTTP da aplicação.
// A ordem é deliberada: a recuperação de pânico fica dentro da rotina criada
// pelo controle de prazo.
func NewRouter(dependencies RouterDependencies) http.Handler {
	router := chi.NewRouter()
	authRateLimiter := newIPRateLimiter(authRequestsPerMinute, authRequestCapacity)
	sessionRateLimiter := newIPRateLimiter(sessionRequestsPerMinute, sessionRequestCapacity)
	router.Use(middleware.RequestID)
	router.Use(securityHeaders)
	router.Use(requestLogger(dependencies.Logger))
	router.Use(middleware.GetHead)
	router.Use(requestTimeout(10*time.Second, dependencies.Logger))
	// requestTimeout executa os handlers seguintes em outra goroutine para poder
	// publicar a resposta de prazo excedido; a recuperação de pânico fica dentro dela.
	router.Use(recoverer(dependencies.Logger))

	router.Get("/", dependencies.Frontend.Index)
	router.Method(
		http.MethodGet,
		"/frontend/*",
		http.StripPrefix("/frontend/", dependencies.Frontend.Assets()),
	)

	router.Route("/auth", func(router chi.Router) {
		router.Use(noStoreResponses)
		router.With(authRateLimiter.middleware).Post("/register", dependencies.Authentication.Register)
		router.With(authRateLimiter.middleware).Post("/login", dependencies.Authentication.Login)
		router.With(sessionRateLimiter.middleware).Post("/refresh", dependencies.Authentication.Refresh)
		router.With(sessionRateLimiter.middleware).Post("/logout", dependencies.Authentication.Logout)
	})

	router.Group(func(router chi.Router) {
		router.Use(noStoreResponses)
		router.Use(authenticationMiddleware(dependencies.TokenValidator, dependencies.Logger))
		router.Post("/cliente", dependencies.Customers.Create)
		router.Get("/cliente", dependencies.Customers.List)
		router.Get("/cliente/{id}", dependencies.Customers.Get)
		router.Put("/cliente/{id}", dependencies.Customers.Update)
		router.Delete("/cliente/{id}", dependencies.Customers.Delete)
	})

	router.NotFound(func(w http.ResponseWriter, request *http.Request) {
		_ = view.WriteError(w, http.StatusNotFound, "route_not_found", "rota não encontrada")
	})
	router.MethodNotAllowed(func(w http.ResponseWriter, request *http.Request) {
		w.Header().Set("Allow", strings.Join(allowedMethods(router, request.URL.Path), ", "))
		_ = view.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "método não permitido")
	})

	return router
}

// allowedMethods calcula os métodos aceitos para uma rota e inclui HEAD quando GET é válido.
// O resultado alimenta o cabeçalho Allow das respostas 405 sem manter uma lista duplicada de rotas.
func allowedMethods(routes chi.Routes, path string) []string {
	methods := make([]string, 0, 2)
	for _, method := range []string{
		http.MethodConnect,
		http.MethodDelete,
		http.MethodGet,
		http.MethodHead,
		http.MethodOptions,
		http.MethodPatch,
		http.MethodPost,
		http.MethodPut,
		http.MethodTrace,
	} {
		matches := routes.Match(chi.NewRouteContext(), method, path)
		if method == http.MethodHead && !matches {
			matches = routes.Match(chi.NewRouteContext(), http.MethodGet, path)
		}
		if matches {
			methods = append(methods, method)
		}
	}
	return methods
}
