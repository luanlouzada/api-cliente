package auth

import (
	"log"
	"net/http"
	"time"
)

func AuthLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startedAt := time.Now()

		claims, ok := ClaimsFromContext(r.Context())
		if ok {
			log.Printf(
				"auth request method=%s path=%s customer_id=%s email=%s",
				r.Method,
				r.URL.Path,
				claims.Subject,
				claims.Email,
			)
		}

		next.ServeHTTP(w, r)

		if ok {
			log.Printf(
				"auth request finished method=%s path=%s customer_id=%s duration=%s",
				r.Method,
				r.URL.Path,
				claims.Subject,
				time.Since(startedAt),
			)
		}
	})
}
