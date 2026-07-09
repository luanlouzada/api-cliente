package routes

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func FrontendRoutes(r chi.Router) {
	fileServer := http.FileServer(http.Dir("frontend"))

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "frontend/index.html")
	})
	r.Handle("/frontend/*", http.StripPrefix("/frontend/", fileServer))
}
