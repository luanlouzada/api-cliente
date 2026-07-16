package controller

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"cliente-api/internal/model"
	"cliente-api/internal/view"

	"github.com/go-chi/chi/v5/middleware"
)

// AccessTokenValidator lista o único método do Model que o Controller precisa
// para autenticar requisições. É uma interface comum de Go, não uma nova camada.
type AccessTokenValidator interface {
	// Validate verifica assinatura e claims do token e devolve somente dados autenticados.
	Validate(string) (model.Claims, error)
}

type claimsContextKey struct{}
type requestMetadataContextKey struct{}

// requestMetadata compartilha dados de observabilidade entre middlewares da mesma requisição.
type requestMetadata struct {
	mutex      sync.RWMutex
	customerID string
}

// setCustomerID registra de forma concorrente o cliente autenticado para enriquecer o log da requisição.
func (metadata *requestMetadata) setCustomerID(customerID string) {
	metadata.mutex.Lock()
	defer metadata.mutex.Unlock()
	metadata.customerID = customerID
}

// customerIDValue lê com segurança o identificador que poderá ser incluído no log ao fim da requisição.
func (metadata *requestMetadata) customerIDValue() string {
	metadata.mutex.RLock()
	defer metadata.mutex.RUnlock()
	return metadata.customerID
}

// ClaimsFromContext recupera as claims inseridas pelo middleware de autenticação.
// O booleano permite distinguir claims ausentes de uma estrutura válida com valores vazios.
func ClaimsFromContext(ctx context.Context) (model.Claims, bool) {
	claims, ok := ctx.Value(claimsContextKey{}).(model.Claims)
	return claims, ok
}

// authenticationMiddleware envolve as rotas protegidas e exige um token Bearer
// válido antes de chamar o próximo Controller. Os campos assinados já verificados
// são guardados no contexto da requisição para não validar o token novamente.
func authenticationMiddleware(
	validator AccessTokenValidator,
	logger *slog.Logger,
) func(http.Handler) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
			token, ok := bearerToken(request.Header.Get("Authorization"))
			if !ok {
				w.Header().Set("WWW-Authenticate", "Bearer")
				writeApplicationError(logger, w, request, ErrBearerTokenRequired)
				return
			}

			claims, err := validator.Validate(token)
			if err != nil {
				if !errors.Is(err, model.ErrAccessTokenExpired) {
					err = model.ErrAccessTokenInvalid
				}
				w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token"`)
				writeApplicationError(logger, w, request, err)
				return
			}

			ctx := context.WithValue(request.Context(), claimsContextKey{}, claims)
			if metadata, ok := ctx.Value(requestMetadataContextKey{}).(*requestMetadata); ok {
				metadata.setCustomerID(claims.Subject)
			}
			next.ServeHTTP(w, request.WithContext(ctx))
		})
	}
}

// bearerToken extrai um token do esquema Authorization Bearer, aceitando diferenças de caixa no esquema.
func bearerToken(authorization string) (string, bool) {
	parts := strings.Fields(authorization)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", false
	}
	return parts[1], true
}

// requestLogger registra método, rota, status, tamanho, duração e identidade autenticada da requisição.
// Os metadados sincronizados permitem que um handler executado pelo timeout atualize o log com segurança.
func requestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
			startedAt := time.Now()
			metadata := &requestMetadata{}
			ctx := context.WithValue(request.Context(), requestMetadataContextKey{}, metadata)
			request = request.WithContext(ctx)
			wrapped := middleware.NewWrapResponseWriter(w, request.ProtoMajor)
			next.ServeHTTP(wrapped, request)

			attributes := []any{
				"request_id", middleware.GetReqID(request.Context()),
				"method", request.Method,
				"path", request.URL.Path,
				"status", wrapped.Status(),
				"bytes", wrapped.BytesWritten(),
				"duration", time.Since(startedAt),
			}
			if customerID := metadata.customerIDValue(); customerID != "" {
				attributes = append(attributes, "customer_id", customerID)
			}
			logger.Info("requisição HTTP", attributes...)
		})
	}
}

// recoverer converte pânicos dos handlers em erro HTTP estável e preserva a pilha apenas no log interno.
func recoverer(logger *slog.Logger) func(http.Handler) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
			defer func() {
				if recovered := recover(); recovered != nil {
					logger.Error(
						"pânico recuperado",
						"method", request.Method,
						"path", request.URL.Path,
						"panic", fmt.Sprint(recovered),
						"stack", string(debug.Stack()),
					)
					if buffered, ok := w.(*bufferedResponseWriter); ok {
						buffered.reset()
					}
					w.Header().Set("Cache-Control", "no-store")
					w.Header().Set("Pragma", "no-cache")
					_ = view.WriteError(w, http.StatusInternalServerError, "internal_error", "erro interno do servidor")
				}
			}()
			next.ServeHTTP(w, request)
		})
	}
}

// securityHeaders adiciona proteções de navegador que valem para API e frontend antes do próximo handler.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Permissions-Policy", "camera=(), geolocation=(), microphone=()")
		w.Header().Set(
			"Content-Security-Policy",
			"default-src 'self'; base-uri 'none'; frame-ancestors 'none'; "+
				"form-action 'self'; object-src 'none'",
		)
		next.ServeHTTP(w, request)
	})
}

// noStoreResponses impede cache das rotas que podem transportar tokens ou dados autenticados.
func noStoreResponses(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Pragma", "no-cache")
		next.ServeHTTP(w, request)
	})
}
